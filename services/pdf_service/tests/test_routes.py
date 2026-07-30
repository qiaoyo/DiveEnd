import asyncio
import json
import os
from unittest.mock import patch

import httpx
import pytest
from fastapi import HTTPException
from fastapi.testclient import TestClient

from app.main import create_app
from core.llm_client import LLMClient, LLMConfig


def test_llm_config_treats_blank_api_key_as_missing_and_uses_env():
    with patch.dict(os.environ, {"OPENAI_API_KEY": "env-key"}):
        config = LLMConfig(
            provider=" OpenAI ",
            model=" ",
            api_key="  ",
            base_url=" https://example.test/v1 ",
        )

    assert config.provider == "openai"
    assert config.model == "gpt-4o-mini"
    assert config.api_key == "env-key"
    assert config.base_url == "https://example.test/v1"


def test_llm_config_rejects_missing_or_blank_api_key():
    with patch.dict(os.environ, {"OPENAI_API_KEY": "  "}):
        with pytest.raises(ValueError, match="API key not found for provider: openai"):
            LLMConfig(provider="openai", api_key=" ")


def test_parse_upload_route_returns_expected_schema():
    app = create_app()
    observed_temp_path = None

    with patch("app.routes.parse.parse_pdf_with_pymupdf4llm") as mock_parse:
        def fake_parse(path, extract_sections):
            nonlocal observed_temp_path
            observed_temp_path = path
            assert os.stat(path).st_mode & 0o777 == 0o600
            with open(path, "rb") as handle:
                assert handle.read().startswith(b"%PDF-1.4")
            return {
                "success": True,
                "markdown": "# Title\n\n## Abstract",
                "metadata": {"title": "Example Paper"},
                "sections": ["Title", "  Abstract"],
                "error": None,
            }

        mock_parse.side_effect = fake_parse

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
    assert observed_temp_path is not None
    assert not os.path.exists(observed_temp_path)


def test_parse_upload_accepts_uppercase_pdf_extension_and_redacts_path_from_logs():
    app = create_app()
    secret_dir = "SECRET_SOURCE_DIR"

    with patch("app.routes.parse.parse_pdf_with_pymupdf4llm") as mock_parse:
        mock_parse.return_value = {
            "success": True,
            "markdown": "# Uppercase",
            "metadata": {},
            "sections": [],
            "error": None,
        }
        with patch("app.routes.parse.logger") as mock_logger:
            with TestClient(app) as client:
                response = client.post(
                    "/parse/upload",
                    files={"file": (f"{secret_dir}/Example.PDF", b"%PDF-1.4 mock", "application/pdf")},
                    data={"extract_sections": "true"},
                )

    assert response.status_code == 200
    logged = " ".join(
        " ".join(str(item) for item in call.args)
        for call in mock_logger.info.call_args_list
    )
    assert "Example.PDF" in logged
    assert secret_dir not in logged


def test_parse_pdf_runtime_unavailable_does_not_echo_import_error():
    from app.routes.parse import parse_pdf_with_pymupdf4llm

    secret = "SECRET_IMPORT_PATH"
    with patch("app.routes.parse._resolve_parser_runtime", side_effect=ImportError(f"missing from {secret}")):
        with patch("app.routes.parse.logger") as mock_logger:
            result = parse_pdf_with_pymupdf4llm("/tmp/example.pdf")

    assert result["success"] is False
    assert result["error"] == "PDF parsing library not available"
    assert secret not in json.dumps(result)
    logged = " ".join(
        " ".join(str(item) for item in call.args)
        for call in mock_logger.error.call_args_list
    )
    assert "ImportError" in logged
    assert secret not in logged


def test_parse_upload_rejects_fake_pdf_before_parser():
    app = create_app()

    with patch("app.routes.parse.parse_pdf_with_pymupdf4llm") as mock_parse:
        with TestClient(app) as client:
            response = client.post(
                "/parse/upload",
                files={"file": ("fake.pdf", b"not actually a pdf", "application/pdf")},
                data={"extract_sections": "true"},
            )

    assert response.status_code == 400
    assert "not a PDF" in response.text
    mock_parse.assert_not_called()


def test_parse_upload_parser_failure_is_generic_and_log_safe():
    app = create_app()
    secret = "SECRET_PARSER_TOKEN"

    with patch("app.routes.parse.parse_pdf_with_pymupdf4llm", side_effect=RuntimeError(f"parser leaked {secret}")):
        with patch("app.routes.parse.logger") as mock_logger:
            with TestClient(app) as client:
                response = client.post(
                    "/parse/upload",
                    files={"file": ("example.pdf", b"%PDF-1.4 mock", "application/pdf")},
                    data={"extract_sections": "true"},
                )

    assert response.status_code == 500
    assert response.json()["error"] == "Failed to parse PDF"
    assert secret not in response.text

    logged = " ".join(
        " ".join(str(item) for item in call.args)
        for call in mock_logger.error.call_args_list
    )
    assert "RuntimeError" in logged
    assert secret not in logged


def test_parse_url_rejects_redirect_to_private_host():
    app = create_app()
    seen_urls = []
    original_async_client = httpx.AsyncClient

    def handler(request: httpx.Request) -> httpx.Response:
        seen_urls.append(str(request.url))
        if request.url.host == "example.org":
            return httpx.Response(
                status_code=302,
                headers={"Location": "http://127.0.0.1/private.pdf"},
                request=request,
            )
        raise AssertionError(f"private redirect target should not be requested: {request.url}")

    def async_client_factory(*args, **kwargs):
        return original_async_client(*args, transport=httpx.MockTransport(handler), **kwargs)

    with patch(
        "app.routes.parse.socket.getaddrinfo",
        return_value=[(None, None, None, None, ("93.184.216.34", 443))],
    ):
        with patch("httpx.AsyncClient", side_effect=async_client_factory):
            with TestClient(app) as client:
                response = client.post(
                    "/parse/url",
                    json={"url": "https://example.org/start.pdf"},
                )

    assert response.status_code == 400
    assert "Local/private PDF URLs are not allowed" in response.text
    assert seen_urls == ["https://example.org/start.pdf"]


def test_parse_url_rejects_fake_pdf_body_before_parser():
    app = create_app()
    original_async_client = httpx.AsyncClient

    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(
            status_code=200,
            headers={"Content-Type": "application/pdf"},
            content=b"not actually a pdf",
            request=request,
        )

    def async_client_factory(*args, **kwargs):
        return original_async_client(*args, transport=httpx.MockTransport(handler), **kwargs)

    with patch(
        "app.routes.parse.socket.getaddrinfo",
        return_value=[(None, None, None, None, ("93.184.216.34", 443))],
    ):
        with patch("httpx.AsyncClient", side_effect=async_client_factory):
            with patch("app.routes.parse.parse_pdf_with_pymupdf4llm") as mock_parse:
                with TestClient(app) as client:
                    response = client.post(
                        "/parse/url",
                        json={"url": "https://example.org/fake.pdf"},
                    )

    assert response.status_code == 400
    assert "not a PDF" in response.text
    mock_parse.assert_not_called()


def test_parse_url_redacts_signed_url_from_logs_and_error_response():
    app = create_app()
    original_async_client = httpx.AsyncClient
    secret = "SECRET_QUERY_TOKEN"
    signed_url = f"https://example.org/private.pdf?token={secret}"

    def handler(request: httpx.Request) -> httpx.Response:
        raise httpx.ConnectError(f"failed to fetch {request.url}", request=request)

    def async_client_factory(*args, **kwargs):
        return original_async_client(*args, transport=httpx.MockTransport(handler), **kwargs)

    with patch(
        "app.routes.parse.socket.getaddrinfo",
        return_value=[(None, None, None, None, ("93.184.216.34", 443))],
    ):
        with patch("httpx.AsyncClient", side_effect=async_client_factory):
            with patch("app.routes.parse.logger") as mock_logger:
                with TestClient(app) as client:
                    response = client.post(
                        "/parse/url",
                        json={"url": signed_url},
                    )

    assert response.status_code == 400
    assert secret not in response.text

    logged = " ".join(
        " ".join(str(item) for item in call.args)
        for call in [
            *mock_logger.info.call_args_list,
            *mock_logger.warning.call_args_list,
            *mock_logger.error.call_args_list,
        ]
    )
    assert "example.org" in logged
    assert secret not in logged
    assert signed_url not in logged


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
    assert payload["usage"] == {
        "input_tokens": 0,
        "output_tokens": 0,
        "total_tokens": 0,
    }


def test_extract_route_tolerates_llm_json_wrappers_and_trailing_text():
    app = create_app()

    class FakeLLMClient:
        def __init__(self, *_args, **_kwargs):
            pass

        async def chat(self, system_prompt: str, user_prompt: str) -> str:
            if "Required fields" in system_prompt:
                return """```json
{
  "title": "Wrapped Paper",
  "authors": ["Author One"],
  "affiliations": [],
  "abstract": "Abstract text.",
  "problem": "Problem text.",
  "method": "Method text.",
  "github_url": null,
  "arxiv_url": null,
  "keywords": ["wrapped-json"]
}
```
"""

            return """{
  "metrics": [
    {
      "metric_name": "Success Rate",
      "dataset_or_task": "RealBench",
      "ours_value": "88",
      "unit": "%"
    }
  ],
  "baselines": [],
  "relevance_tags": ["robust-json"]
}

Additional explanation that should be ignored."""

    with patch.dict(os.environ, {"OPENAI_API_KEY": "test-key"}):
        with patch("app.routes.extract.LLMClient", FakeLLMClient):
            with TestClient(app) as client:
                response = client.post(
                    "/extract/",
                    json={
                        "markdown": "# Wrapped Paper\n\n## Abstract\nAbstract text.",
                        "extraction_type": "all",
                        "provider": "openai",
                    },
                )

    assert response.status_code == 200
    payload = response.json()
    assert payload["success"] is True
    assert payload["data"]["metadata"]["title"] == "Wrapped Paper"
    assert payload["data"]["metrics"][0]["metric_name"] == "Success Rate"
    assert payload["data"]["relevance_tags"] == ["robust-json"]


def test_extract_route_rejects_oversized_markdown_before_llm():
    app = create_app()

    with patch("app.routes.extract.LLMClient") as mock_llm:
        with TestClient(app) as client:
            response = client.post(
                "/extract/",
                json={
                    "markdown": "x" * (2 * 1024 * 1024 + 1),
                    "provider": "openai",
                    "api_key": "test-key",
                },
            )

    assert response.status_code == 422
    mock_llm.assert_not_called()


def test_extract_route_redacts_upstream_error_from_response_and_logs():
    app = create_app()
    secret = "SECRET_UPSTREAM_TOKEN"

    class FailingLLMClient:
        def __init__(self, *_args, **_kwargs):
            pass

        async def chat(self, system_prompt: str, user_prompt: str) -> str:
            raise RuntimeError(f"upstream failure leaked {secret}")

    with patch("app.routes.extract.LLMClient", FailingLLMClient):
        with patch("app.routes.extract.logger") as mock_logger:
            with TestClient(app) as client:
                response = client.post(
                    "/extract/",
                    json={
                        "markdown": "# Example\n\nBody",
                        "provider": "openai",
                        "api_key": "test-key",
                    },
                )

    assert response.status_code == 200
    payload = response.json()
    assert payload["success"] is False
    assert payload["error"] == "Extraction failed"
    assert secret not in response.text

    logged = " ".join(
        " ".join(str(item) for item in call.args)
        for call in mock_logger.warning.call_args_list
    )
    assert "RuntimeError" in logged
    assert secret not in logged


def test_extract_route_redacts_invalid_provider_config_errors():
    app = create_app()
    secret = "SECRET_PROVIDER_TOKEN"

    with patch("app.routes.extract.logger") as mock_logger:
        with TestClient(app) as client:
            response = client.post(
                "/extract/",
                json={
                    "markdown": "# Example\n\nBody",
                    "provider": f"openai api_key=sk-{secret}",
                    "api_key": "test-key",
                },
            )

    assert response.status_code == 200
    payload = response.json()
    assert payload["success"] is False
    assert payload["error"].startswith("Unsupported provider")
    assert "[redacted]" in response.text
    assert secret not in response.text
    assert f"sk-{secret}" not in response.text

    logged = " ".join(
        " ".join(str(item) for item in call.args)
        for call in mock_logger.warning.call_args_list
    )
    assert "ValueError" in logged
    assert secret not in logged


def test_extract_route_does_not_log_raw_llm_parse_failures():
    app = create_app()
    secret = "SECRET_SCHEMA_TOKEN"

    class InvalidSchemaLLMClient:
        def __init__(self, *_args, **_kwargs):
            self.calls = 0

        async def chat(self, system_prompt: str, user_prompt: str) -> str:
            self.calls += 1
            if self.calls == 1:
                return json.dumps(
                    {
                        "title": {"api_key": f"sk-{secret}"},
                        "authors": [],
                        "affiliations": [],
                        "abstract": "",
                        "problem": "",
                        "method": "",
                        "keywords": [],
                    }
                )
            return json.dumps(
                {
                    "metrics": [
                        {
                            "metric_name": {"access_token": secret},
                            "dataset_or_task": "dataset",
                            "ours_value": "1.0",
                        }
                    ],
                    "baselines": [],
                    "relevance_tags": [],
                }
            )

    with patch("app.routes.extract.LLMClient", InvalidSchemaLLMClient):
        with patch("app.routes.extract.logger") as mock_logger:
            with TestClient(app) as client:
                response = client.post(
                    "/extract/",
                    json={
                        "markdown": "# Example\n\nBody",
                        "provider": "openai",
                        "api_key": "test-key",
                    },
                )

    assert response.status_code == 200
    payload = response.json()
    assert payload["success"] is True
    assert secret not in response.text

    logged = " ".join(
        " ".join(str(item) for item in call.args)
        for call in mock_logger.error.call_args_list
    )
    assert "ValidationError" in logged
    assert secret not in logged
    assert "sk-" not in logged


def test_global_http_exception_handler_redacts_sensitive_detail():
    app = create_app()
    secret = "SECRET_HTTP_TOKEN"

    @app.get("/test-http-error")
    async def test_http_error():
        raise HTTPException(
            status_code=400,
            detail={
                "message": f"bad request api_key=sk-http-secret and access_token={secret}",
                "items": [f"Authorization: Bearer sk-http-bearer-secret-12345"],
            },
        )

    with patch("app.main.logger") as mock_logger:
        with TestClient(app) as client:
            response = client.get("/test-http-error")

    assert response.status_code == 400
    assert secret not in response.text
    assert "sk-http-secret" not in response.text
    assert "sk-http-bearer-secret-12345" not in response.text
    assert "[redacted]" in response.text

    logged = " ".join(
        " ".join(str(item) for item in call.args)
        for call in mock_logger.error.call_args_list
    )
    assert secret not in logged
    assert "sk-http-secret" not in logged
    assert "sk-http-bearer-secret-12345" not in logged


def test_llm_client_logs_exception_type_without_secret():
    secret = "SECRET_LLM_TOKEN"

    class FailingCompletions:
        async def create(self, *_args, **_kwargs):
            raise RuntimeError(f"upstream leaked Authorization: Bearer {secret}")

    class FailingChat:
        completions = FailingCompletions()

    class FailingClient:
        chat = FailingChat()

    llm = object.__new__(LLMClient)
    llm.config = LLMConfig(provider="openai", api_key="test-key")
    llm._client = FailingClient()

    with patch("core.llm_client.logger") as mock_logger:
        with pytest.raises(RuntimeError):
            asyncio.run(llm.chat("system", "user"))

    logged = " ".join(
        " ".join(str(item) for item in call.args)
        for call in mock_logger.error.call_args_list
    )
    assert "RuntimeError" in logged
    assert secret not in logged
