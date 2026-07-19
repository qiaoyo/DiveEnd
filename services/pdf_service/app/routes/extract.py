"""LLM extraction routes."""

import json
from typing import Any, Optional

from fastapi import APIRouter, HTTPException, status
from pydantic import BaseModel, Field

from core.logger import setup_logging
from core.llm_client import LLMClient, LLMConfig
from core.redaction import redact_sensitive_text

router = APIRouter()
logger = setup_logging()
MAX_EXTRACTION_MARKDOWN_CHARS = 2 * 1024 * 1024


class ExtractionRequest(BaseModel):
    """Extraction request."""

    markdown: str = Field(
        ...,
        min_length=1,
        max_length=MAX_EXTRACTION_MARKDOWN_CHARS,
        description="Markdown text to extract from",
    )
    extraction_type: str = Field(
        default="paper_metadata",
        description="Type of extraction: paper_metadata, metrics, baselines, all"
    )
    provider: str = Field(default="openai", description="LLM provider: openai or anthropic")
    model: Optional[str] = Field(default=None, description="LLM model override")
    api_key: Optional[str] = Field(default=None, description="LLM API key override")
    base_url: Optional[str] = Field(default=None, description="OpenAI-compatible or Anthropic base URL")
    max_tokens: int = Field(default=4096, ge=1, le=8192, description="Maximum output tokens")
    temperature: float = Field(default=0.0, ge=0.0, le=2.0, description="Sampling temperature")
    timeout: int = Field(default=120, ge=1, le=300, description="Request timeout in seconds")


class PaperMetadata(BaseModel):
    """Paper metadata extraction result."""

    title: str = Field(description="Paper title")
    authors: list[str] = Field(default_factory=list, description="Author names")
    affiliations: list[str] = Field(default_factory=list, description="Author affiliations")
    abstract: str = Field(default="", description="Paper abstract")
    problem: str = Field(default="", description="Problem statement")
    method: str = Field(default="", description="Method description")
    github_url: Optional[str] = Field(None, description="GitHub repository URL")
    arxiv_url: Optional[str] = Field(None, description="arXiv URL")
    keywords: list[str] = Field(default_factory=list, description="Paper keywords")


class Metric(BaseModel):
    """Performance metric."""

    metric_name: str = Field(description="Name of the metric")
    dataset_or_task: str = Field(description="Dataset or task name")
    ours_value: str = Field(description="This paper's value")
    unit: Optional[str] = Field(None, description="Unit of measurement")


class Baseline(BaseModel):
    """Baseline comparison."""

    metric_name: str = Field(description="Associated metric name")
    method_name: str = Field(description="Baseline method name")
    value: str = Field(description="Baseline value")


class ExtractionResult(BaseModel):
    """Complete extraction result."""

    metadata: PaperMetadata = Field(default_factory=PaperMetadata)
    metrics: list[Metric] = Field(default_factory=list)
    baselines: list[Baseline] = Field(default_factory=list)
    relevance_tags: list[str] = Field(default_factory=list)


class ExtractionResponse(BaseModel):
    """Extraction response."""

    success: bool = Field(description="Whether extraction succeeded")
    data: Optional[ExtractionResult] = Field(None, description="Extraction result")
    error: Optional[str] = Field(None, description="Error message if failed")
    provider: str = Field(description="LLM provider used")
    model: str = Field(description="LLM model used")


# Extraction prompts
PAPER_METADATA_PROMPT = """You are a research paper extraction assistant. Extract the following information from the provided paper text:

Required fields:
- title: The full paper title
- authors: List of author names
- affiliations: List of author affiliations/institutions
- abstract: The complete abstract
- problem: What problem does this paper address (2-3 sentences)
- method: What method/approach does the paper propose (2-3 sentences)
- github_url: GitHub repository URL if mentioned (null if not found)
- arxiv_url: arXiv URL if mentioned (null if not found)
- keywords: List of 5-10 relevant keywords/tags

Return ONLY a valid JSON object with these fields. No markdown, no explanation."""

METRICS_PROMPT = """Extract all performance metrics reported in this paper.

For each metric, extract:
- metric_name: The name of the metric (e.g., "Accuracy", "F1 Score", "BLEU")
- dataset_or_task: The dataset or task this metric applies to
- ours_value: The value reported for this paper's method
- unit: The unit of measurement if applicable (e.g., "%", "ms", null if none)

Also extract baseline comparisons for each metric:
- metric_name: Which metric this baseline compares to
- method_name: Name of the baseline method
- value: The baseline's value

Return a JSON object with:
{
  "metrics": [...],
  "baselines": [...],
  "relevance_tags": ["tag1", "tag2", ...]
}

Relevance tags should categorize the paper (e.g., "sim-to-real", "locomotion", "diffusion-policy", etc.)"""


def _loads_llm_json(response: str) -> Any:
    """Parse JSON from LLM output, tolerating fences and trailing prose."""
    text = response.strip()
    try:
        return json.loads(text)
    except json.JSONDecodeError:
        pass

    decoder = json.JSONDecoder()
    for index, char in enumerate(text):
        if char not in "[{":
            continue
        try:
            parsed, _ = decoder.raw_decode(text[index:])
            return parsed
        except json.JSONDecodeError:
            continue

    raise json.JSONDecodeError("No JSON object found in LLM response", text, 0)


def public_extraction_error(error: Exception) -> str:
    """Return a client-safe extraction error."""
    message = str(error).strip()
    if isinstance(error, ValueError) and (
        message.startswith("API key not found")
        or message.startswith("Unsupported provider")
    ):
        return redact_sensitive_text(message)
    return "Extraction failed"


@router.post(
    "/",
    response_model=ExtractionResponse,
    status_code=status.HTTP_200_OK,
    summary="Extract structured data from markdown",
)
async def extract_data(request: ExtractionRequest):
    """Extract structured data from paper markdown.

    Uses LLM to extract metadata, metrics, and baselines from paper text.

    Args:
        request: Extraction request with markdown and parameters

    Returns:
        ExtractionResponse with extracted data
    """
    try:
        # Initialize LLM client
        model = request.model or (
            "gpt-4o-mini" if request.provider == "openai" else "claude-sonnet-4-20250514"
        )
        llm_config = LLMConfig(
            provider=request.provider,
            model=model,
            api_key=request.api_key,
            base_url=request.base_url,
            max_tokens=request.max_tokens,
            temperature=request.temperature,
            timeout=request.timeout,
        )
        llm_client = LLMClient(llm_config)

        # Step 1: Extract metadata
        logger.info("Extracting paper metadata...")
        metadata_response = await llm_client.chat(
            system_prompt=PAPER_METADATA_PROMPT,
            user_prompt=request.markdown[:15000],  # Limit input size
        )

        # Parse metadata
        try:
            metadata_dict = _loads_llm_json(metadata_response)
            metadata = PaperMetadata(**metadata_dict)
        except (json.JSONDecodeError, Exception) as e:
            logger.error("Failed to parse metadata: %s", type(e).__name__)
            metadata = PaperMetadata(
                title="",
                problem="Failed to parse metadata",
            )

        # Step 2: Extract metrics and baselines
        logger.info("Extracting metrics and baselines...")
        metrics_response = await llm_client.chat(
            system_prompt=METRICS_PROMPT,
            user_prompt=request.markdown[:20000],
        )

        # Parse metrics
        metrics = []
        baselines = []
        relevance_tags = []

        try:
            metrics_dict = _loads_llm_json(metrics_response)
            metrics = [Metric(**m) for m in metrics_dict.get("metrics", [])]
            baselines = [Baseline(**b) for b in metrics_dict.get("baselines", [])]
            relevance_tags = metrics_dict.get("relevance_tags", [])
        except (json.JSONDecodeError, Exception) as e:
            logger.error("Failed to parse metrics: %s", type(e).__name__)

        # Build result
        result = ExtractionResult(
            metadata=metadata,
            metrics=metrics,
            baselines=baselines,
            relevance_tags=relevance_tags,
        )

        return ExtractionResponse(
            success=True,
            data=result,
            error=None,
            provider=request.provider,
            model=llm_config.model,
        )

    except Exception as e:
        logger.warning("Error during extraction: %s", type(e).__name__)
        return ExtractionResponse(
            success=False,
            data=None,
            error=public_extraction_error(e),
            provider=redact_sensitive_text(request.provider),
            model="unknown",
        )
