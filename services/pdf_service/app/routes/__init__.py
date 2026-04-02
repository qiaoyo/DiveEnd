"""API routes for PDF processing service."""

from app.routes.health import router as health_router
from app.routes.parse import router as parse_router
from app.routes.extract import router as extract_router

__all__ = [
    "health_router",
    "parse_router",
    "extract_router",
]
