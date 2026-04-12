"""PDF parsing routes (PyMuPDF4LLM backend)."""

import tempfile
from functools import lru_cache
from pathlib import Path
from typing import Optional

from fastapi import APIRouter, File, Form, HTTPException, UploadFile, status
from pydantic import BaseModel, Field

from core.logger import setup_logging

router = APIRouter()
logger = setup_logging()


@lru_cache(maxsize=1)
def _resolve_parser_runtime():
    """Resolve PyMuPDF4LLM runtime in a lazy and safe way."""
    import pymupdf4llm  # type: ignore
    import pymupdf as fitz  # type: ignore

    return {
        "pymupdf4llm": pymupdf4llm,
        "fitz": fitz,
    }


@lru_cache(maxsize=1)
def parser_runtime_status() -> tuple[bool, str]:
    """Return parser availability and backend kind."""
    try:
        _resolve_parser_runtime()
        return True, "pymupdf4llm"
    except Exception:
        return False, "unavailable"


class ParseResponse(BaseModel):
    """PDF parsing response."""

    success: bool = Field(description="Whether parsing succeeded")
    markdown: Optional[str] = Field(None, description="Extracted markdown text")
    metadata: dict = Field(default_factory=dict, description="PDF metadata")
    sections: list[str] = Field(default_factory=list, description="Identified sections")
    error: Optional[str] = Field(None, description="Error message if failed")


class ParseURLRequest(BaseModel):
    """Parse PDF from URL request."""

    url: str = Field(..., description="URL to PDF file")


@router.post(
    "/upload",
    response_model=ParseResponse,
    status_code=status.HTTP_200_OK,
    summary="Parse uploaded PDF file",
)
async def parse_upload(
    file: UploadFile = File(..., description="PDF file to parse"),
    extract_sections: bool = Form(True, description="Extract and identify sections"),
):
    """Parse an uploaded PDF file.

    Args:
        file: PDF file to parse
        extract_sections: Whether to extract and identify sections

    Returns:
        ParseResponse with extracted content
    """
    # Validate file type
    if not file.filename.endswith(".pdf"):
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="Only PDF files are supported",
        )

    try:
        # Save uploaded file to temp location
        with tempfile.NamedTemporaryFile(delete=False, suffix=".pdf") as tmp_file:
            content = await file.read()
            tmp_file.write(content)
            tmp_path = tmp_file.name

        logger.info(f"Processing PDF: {file.filename}, size: {len(content)} bytes")

        # Parse PDF using PyMuPDF4LLM
        result = parse_pdf_with_pymupdf4llm(tmp_path, extract_sections)

        # Cleanup temp file
        Path(tmp_path).unlink(missing_ok=True)

        return ParseResponse(**result)

    except Exception as e:
        logger.exception(f"Error parsing PDF: {e}")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Failed to parse PDF: {str(e)}",
        )


@router.post(
    "/url",
    response_model=ParseResponse,
    status_code=status.HTTP_200_OK,
    summary="Parse PDF from URL",
)
async def parse_url(
    request: ParseURLRequest,
    extract_sections: bool = True,
):
    """Parse a PDF from a URL.

    Args:
        request: URL request with PDF location
        extract_sections: Whether to extract and identify sections

    Returns:
        ParseResponse with extracted content
    """
    import httpx

    try:
        # Download PDF from URL
        logger.info(f"Downloading PDF from: {request.url}")

        async with httpx.AsyncClient() as client:
            response = await client.get(request.url, timeout=60.0)
            response.raise_for_status()

            # Save to temp file
            with tempfile.NamedTemporaryFile(delete=False, suffix=".pdf") as tmp_file:
                tmp_file.write(response.content)
                tmp_path = tmp_file.name

        logger.info(f"Downloaded PDF, size: {len(response.content)} bytes")

        # Parse PDF using PyMuPDF4LLM
        result = parse_pdf_with_pymupdf4llm(tmp_path, extract_sections)

        # Cleanup temp file
        Path(tmp_path).unlink(missing_ok=True)

        return ParseResponse(**result)

    except httpx.HTTPError as e:
        logger.exception(f"HTTP error downloading PDF: {e}")
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail=f"Failed to download PDF: {str(e)}",
        )
    except Exception as e:
        logger.exception(f"Error parsing PDF from URL: {e}")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Failed to parse PDF: {str(e)}",
        )


def parse_pdf_with_pymupdf4llm(pdf_path: str, extract_sections: bool = True) -> dict:
    """Parse PDF using PyMuPDF4LLM.

    Args:
        pdf_path: Path to PDF file
        extract_sections: Whether to extract and identify sections

    Returns:
        Dictionary with markdown, metadata, and sections
    """
    try:
        runtime = _resolve_parser_runtime()
        pymupdf4llm = runtime["pymupdf4llm"]
        fitz = runtime["fitz"]
        logger.info(f"Parsing PDF with PyMuPDF4LLM: {pdf_path}")

        markdown = pymupdf4llm.to_markdown(pdf_path)

        metadata = {}
        try:
            document = fitz.open(pdf_path)
            metadata = document.metadata or {}
            metadata["pages"] = len(document)
            document.close()
        except Exception:
            metadata = {}

        result = {
            "success": True,
            "markdown": markdown,
            "metadata": metadata if metadata else {},
            "sections": [],
            "error": None,
        }

        # Extract sections if requested
        if extract_sections and markdown:
            result["sections"] = extract_sections_from_markdown(markdown)

        logger.info(f"Successfully parsed PDF, markdown length: {len(markdown) if markdown else 0}")

        return result

    except ImportError as e:
        logger.error(f"PyMuPDF4LLM library not available: {e}")
        return {
            "success": False,
            "markdown": None,
            "metadata": {},
            "sections": [],
            "error": f"PDF parsing library not available: {str(e)}",
        }
    except Exception as e:
        logger.exception(f"Error parsing PDF: {e}")
        return {
            "success": False,
            "markdown": None,
            "metadata": {},
            "sections": [],
            "error": str(e),
        }


def extract_sections_from_markdown(markdown: str) -> list[str]:
    """Extract section headers from markdown.

    Args:
        markdown: Markdown text

    Returns:
        List of section names
    """
    import re

    sections = []
    lines = markdown.split("\n")

    for line in lines:
        # Match markdown headers (# ## ###)
        match = re.match(r"^(#{1,3})\s+(.+)$", line.strip())
        if match:
            level = len(match.group(1))
            title = match.group(2).strip()
            if level == 1:
                sections.append(f"{title}")
            elif level == 2:
                sections.append(f"  {title}")
            else:
                sections.append(f"    {title}")

    return sections
