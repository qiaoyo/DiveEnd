import json
import os
from unittest.mock import patch

from fastapi.testclient import TestClient

from app.main import create_app


def test_parse_upload_route_returns_expected_schema():
    app = create_app()

    with patch("app.routes.parse.parse_pdf_with_pymupdf4llm") as mock_parse:
        mock_parse.return_value = {
            "success": True,
            "markdown": "# Title\n\n## Abstract",
            "metadata": {"title": "Example Paper"},
            "sections": ["Title", "  Abstract"],
            "error": None,
        }

        with TestClient(app) as client:
            response = client.post(
                "/parse/upload",
                files={"file": ("example.pdf", b"%PDF-1.4 mock", "application/pdf")},
                data={"extract_sections": "true"},
            )

    assert response.status_code == 200
    payload = response.json()
    assert payload["success"] is True
    assert payload["markdown"].startswith("# Title")
    assert payload["metadata"]["title"] == "Example Paper"
    assert payload["sections"] == ["Title", "  Abstract"]
    assert payload["error"] is None


def test_extract_route_returns_expected_schema():
    app = create_app()

    class FakeLLMClient:
        def __init__(self, *_args, **_kwargs):
            pass

        async def chat(self, system_prompt: str, user_prompt: str) -> str:
            if "Required fields" in system_prompt:
                return json.dumps(
                    {
                        "title": "Example Paper",
                        "authors": ["Author One", "Author Two"],
                        "affiliations": ["Example University"],
                        "abstract": "A concise abstract.",
                        "problem": "The paper studies route contracts.",
                        "method": "It uses mocked LLM calls in tests.",
                        "github_url": "https://github.com/example/repo",
                        "arxiv_url": "https://arxiv.org/abs/1234.5678",
                        "keywords": ["testing", "contracts"],
                    }
                )

            return json.dumps(
                {
                    "metrics": [
                        {
                            "metric_name": "Accuracy",
                            "dataset_or_task": "ExampleBench",
                            "ours_value": "95.0",
                            "unit": "%",
                        }
                    ],
                    "baselines": [
                        {
                            "metric_name": "Accuracy",
                            "method_name": "Baseline",
                            "value": "90.0",
                        }
                    ],
                    "relevance_tags": ["screening", "pdf-service"],
                }
            )

    with patch.dict(os.environ, {"OPENAI_API_KEY": "test-key"}):
        with patch("app.routes.extract.LLMClient", FakeLLMClient):
            with TestClient(app) as client:
                response = client.post(
                    "/extract/",
                    json={
                        "markdown": "# Example Paper\n\n## Abstract\nA concise abstract.",
                        "extraction_type": "all",
                        "provider": "openai",
                    },
                )

    assert response.status_code == 200
    payload = response.json()
    assert payload["success"] is True
    assert payload["provider"] == "openai"
    assert payload["data"]["metadata"]["title"] == "Example Paper"
    assert payload["data"]["metrics"][0]["metric_name"] == "Accuracy"
    assert payload["data"]["baselines"][0]["method_name"] == "Baseline"
    assert payload["data"]["relevance_tags"] == ["screening", "pdf-service"]
