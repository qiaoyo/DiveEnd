"""PDF parsing routes (PyMuPDF4LLM backend)."""

import ipaddress
import os
import socket
import tempfile
from functools import lru_cache
from pathlib import Path
from typing import Optional
from urllib.parse import urljoin, urlparse

from fastapi import APIRouter, File, Form, HTTPException, UploadFile, status
from pydantic import BaseModel, Field

from core.config import get_config
from core.logger import setup_logging

router = APIRouter()
logger = setup_logging()
DOWNLOAD_CHUNK_SIZE = 1024 * 1024
MAX_PDF_REDIRECTS = 5


def secure_temp_pdf_file():
    tmp_file = tempfile.NamedTemporaryFile(delete=False, suffix=".pdf")
    os.chmod(tmp_file.name, 0o600)
    return tmp_file


def flush_temp_pdf(tmp_file) -> None:
    tmp_file.flush()
    os.fsync(tmp_file.fileno())


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
    display_filename = safe_upload_filename_for_log(file.filename)
    # Validate file type
    if not display_filename.lower().endswith(".pdf"):
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="Only PDF files are supported",
        )

    tmp_path = None
    try:
        # Save uploaded file to temp location
        with secure_temp_pdf_file() as tmp_file:
            tmp_path = tmp_file.name
            size = 0
            first_chunk = b""
            while True:
                chunk = await file.read(DOWNLOAD_CHUNK_SIZE)
                if not chunk:
                    break
                if not first_chunk:
                    first_chunk = chunk[:1024]
                size += len(chunk)
                if size > get_config().max_pdf_size:
                    raise HTTPException(
                        status_code=status.HTTP_413_REQUEST_ENTITY_TOO_LARGE,
                        detail="PDF exceeds configured size limit",
                    )
                tmp_file.write(chunk)
            flush_temp_pdf(tmp_file)

        if not looks_like_pdf(first_chunk):
            raise HTTPException(
                status_code=status.HTTP_400_BAD_REQUEST,
                detail="Uploaded file is not a PDF",
            )

        logger.info("Processing uploaded PDF: %s, size: %d bytes", display_filename, size)

        # Parse PDF using PyMuPDF4LLM
        result = parse_pdf_with_pymupdf4llm(tmp_path, extract_sections)

        return ParseResponse(**result)

    except HTTPException:
        raise
    except Exception as e:
        logger.error("Error parsing uploaded PDF: %s", type(e).__name__)
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail="Failed to parse PDF",
        )
    finally:
        if tmp_path:
            Path(tmp_path).unlink(missing_ok=True)


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

    tmp_path = None
    try:
        validated_url = validate_public_pdf_url(request.url)
        request_host = safe_url_host_for_log(validated_url)
        # Download PDF from URL. Keep query strings out of logs; signed PDF URLs
        # often carry short-lived credentials.
        logger.info("Downloading PDF from host: %s", request_host)
        config = get_config()

        size = 0
        first_chunk = b""
        async with httpx.AsyncClient(follow_redirects=False, timeout=float(config.pdf_timeout)) as client:
            current_url = validated_url
            for redirect_count in range(MAX_PDF_REDIRECTS + 1):
                async with client.stream("GET", current_url) as response:
                    if response.is_redirect:
                        if redirect_count >= MAX_PDF_REDIRECTS:
                            raise HTTPException(
                                status_code=status.HTTP_400_BAD_REQUEST,
                                detail="Too many PDF URL redirects",
                            )
                        location = response.headers.get("location")
                        if not location:
                            raise HTTPException(
                                status_code=status.HTTP_400_BAD_REQUEST,
                                detail="PDF URL redirect is missing a Location header",
                            )
                        redirected_url = urljoin(str(response.url), location)
                        current_url = validate_public_pdf_url(redirected_url)
                        continue

                    response.raise_for_status()
                    content_length = response.headers.get("content-length")
                    if content_length:
                        try:
                            declared_size = int(content_length)
                        except ValueError as exc:
                            raise HTTPException(
                                status_code=status.HTTP_400_BAD_REQUEST,
                                detail="Invalid Content-Length header",
                            ) from exc
                        if declared_size > config.max_pdf_size:
                            raise HTTPException(
                                status_code=status.HTTP_413_REQUEST_ENTITY_TOO_LARGE,
                                detail="PDF exceeds configured size limit",
                            )

                    # Save to temp file
                    with secure_temp_pdf_file() as tmp_file:
                        tmp_path = tmp_file.name
                        async for chunk in response.aiter_bytes(DOWNLOAD_CHUNK_SIZE):
                            if not chunk:
                                continue
                            if not first_chunk:
                                first_chunk = chunk[:1024]
                            size += len(chunk)
                            if size > config.max_pdf_size:
                                raise HTTPException(
                                    status_code=status.HTTP_413_REQUEST_ENTITY_TOO_LARGE,
                                    detail="PDF exceeds configured size limit",
                                )
                            tmp_file.write(chunk)
                        flush_temp_pdf(tmp_file)
                    break

        if not looks_like_pdf(first_chunk):
            raise HTTPException(
                status_code=status.HTTP_400_BAD_REQUEST,
                detail="Downloaded response is not a PDF",
            )

        logger.info("Downloaded PDF from host: %s, size: %d bytes", request_host, size)

        # Parse PDF using PyMuPDF4LLM
        result = parse_pdf_with_pymupdf4llm(tmp_path, extract_sections)

        return ParseResponse(**result)

    except HTTPException:
        raise
    except httpx.HTTPError as e:
        logger.warning("HTTP error downloading PDF from host %s: %s", safe_url_host_for_log(request.url), type(e).__name__)
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="Failed to download PDF",
        )
    except Exception as e:
        logger.error("Error parsing PDF from URL host %s: %s", safe_url_host_for_log(request.url), type(e).__name__)
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail="Failed to parse PDF",
        )
    finally:
        if tmp_path:
            Path(tmp_path).unlink(missing_ok=True)


def validate_public_pdf_url(raw: str) -> str:
    raw = raw.strip()
    parsed = urlparse(raw)
    if parsed.scheme not in {"http", "https"} or not parsed.hostname:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="URL must be a valid http/https URL",
        )

    host = parsed.hostname.rstrip(".").lower()
    if host == "localhost" or host.endswith(".localhost"):
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="Local/private PDF URLs are not allowed",
        )

    try:
        addresses = [ipaddress.ip_address(host)]
    except ValueError:
        try:
            addresses = [
                ipaddress.ip_address(info[4][0])
                for info in socket.getaddrinfo(host, parsed.port or 443, type=socket.SOCK_STREAM)
            ]
        except socket.gaierror as exc:
            raise HTTPException(
                status_code=status.HTTP_400_BAD_REQUEST,
                detail=f"Failed to resolve URL host: {exc}",
            ) from exc

    for address in addresses:
        if address.is_private or address.is_loopback or address.is_link_local or address.is_unspecified:
            raise HTTPException(
                status_code=status.HTTP_400_BAD_REQUEST,
                detail="Local/private PDF URLs are not allowed",
            )

    return raw


def safe_url_host_for_log(raw: str) -> str:
    parsed = urlparse(raw.strip())
    return parsed.hostname or "unknown"


def safe_upload_filename_for_log(raw: Optional[str]) -> str:
    filename = Path(raw or "upload.pdf").name.strip()
    return filename or "upload.pdf"


def looks_like_pdf(prefix: bytes) -> bool:
    return prefix.lstrip().startswith(b"%PDF-")


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
        logger.error("PyMuPDF4LLM library not available: %s", type(e).__name__)
        return {
            "success": False,
            "markdown": None,
            "metadata": {},
            "sections": [],
            "error": "PDF parsing library not available",
        }
    except Exception as e:
        logger.error("Error parsing PDF: %s", type(e).__name__)
        return {
            "success": False,
            "markdown": None,
            "metadata": {},
            "sections": [],
            "error": "PDF parsing failed",
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
