"""FastAPI application for PDF processing service."""

import os
import sys
from contextlib import asynccontextmanager
from pathlib import Path
from typing import AsyncGenerator

# Add parent directory to path for imports
sys.path.insert(0, str(Path(__file__).parent.parent))

from fastapi import FastAPI, HTTPException, status
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import JSONResponse

from app.routes import health, parse, extract
from core.config import get_config
from core.logger import setup_logging
from core.redaction import redact_sensitive_value

# Setup logging
logger = setup_logging()


@asynccontextmanager
async def lifespan(app: FastAPI) -> AsyncGenerator[None, None]:
    """Application lifespan manager."""
    # Startup
    logger.info("Starting PDF Processing Service...")
    config = get_config()
    logger.info(f"Configuration loaded: host={config.host}, port={config.port}")

    # Verify parser backend is available
    available, backend = parse.parser_runtime_status()
    if available:
        logger.info(f"PDF parser is available (backend={backend})")
    else:
        logger.error("PDF parser backend not available")
        logger.warning("PDF parsing functionality will not work!")

    yield

    # Shutdown
    logger.info("Shutting down PDF Processing Service...")


def create_app() -> FastAPI:
    """Create and configure FastAPI application."""
    config = get_config()

    app = FastAPI(
        title="DiveEnd PDF Processing Service",
        description="PDF parsing and LLM extraction service for DiveEnd",
        version="0.1.0",
        lifespan=lifespan,
        docs_url="/docs",
        redoc_url="/redoc",
    )

    # Add CORS middleware
    app.add_middleware(
        CORSMiddleware,
        allow_origins=[],
        allow_origin_regex=config.cors_allow_origin_regex,
        allow_credentials=True,
        allow_methods=["GET", "POST", "OPTIONS"],
        allow_headers=["Authorization", "Content-Type"],
    )

    # Include routers
    app.include_router(health.router, prefix="/health", tags=["health"])
    app.include_router(parse.router, prefix="/parse", tags=["parsing"])
    app.include_router(extract.router, prefix="/extract", tags=["extraction"])

    # Global exception handlers
    @app.exception_handler(HTTPException)
    async def http_exception_handler(request, exc):
        safe_detail = redact_sensitive_value(exc.detail)
        logger.error("HTTP Exception: %s - %s", exc.status_code, safe_detail)
        return JSONResponse(
            status_code=exc.status_code,
            content={"error": safe_detail, "status_code": exc.status_code},
        )

    @app.exception_handler(Exception)
    async def general_exception_handler(request, exc):
        logger.error("Unhandled Exception: %s", type(exc).__name__)
        return JSONResponse(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            content={"error": "Internal server error", "status_code": 500},
        )

    return app


# Create the application instance
app = create_app()


if __name__ == "__main__":
    import uvicorn

    config = get_config()

    uvicorn.run(
        "app.main:app",
        host=config.host,
        port=config.port,
        workers=config.workers,
        reload=True,  # Enable auto-reload for development
        log_level="info",
    )
