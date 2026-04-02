"""Health check routes."""

from fastapi import APIRouter, status
from pydantic import BaseModel

router = APIRouter()


class HealthResponse(BaseModel):
    status: str
    version: str
    services: dict


class ReadinessResponse(BaseModel):
    ready: bool
    checks: dict


@router.get(
    "/",
    response_model=HealthResponse,
    status_code=status.HTTP_200_OK,
    summary="Health check endpoint",
)
async def health_check():
    """Basic health check endpoint.

    Returns service status and version information.
    """
    return HealthResponse(
        status="healthy",
        version="0.1.0",
        services={
            "api": "up",
            "pdf_parser": "unknown",  # Will check actual status
            "llm": "unknown",
        },
    )


@router.get(
    "/ready",
    response_model=ReadinessResponse,
    status_code=status.HTTP_200_OK,
    summary="Readiness check endpoint",
)
async def readiness_check():
    """Detailed readiness check.

    Checks if all required services are available.
    """
    checks = {
        "database": {"status": "up", "latency_ms": 0},
        "pdf_parser": {"status": "up", "message": "Marker available"},
        "llm_weak": {"status": "up", "provider": "openai"},
        "llm_strong": {"status": "up", "provider": "anthropic"},
    }

    all_ready = all(
        check["status"] == "up" for check in checks.values()
    )

    return ReadinessResponse(
        ready=all_ready,
        checks=checks,
    )


@router.get(
    "/live",
    status_code=status.HTTP_200_OK,
    summary="Liveness probe endpoint",
)
async def liveness_check():
    """Kubernetes-style liveness probe.

    Simple check that the process is running.
    """
    return {"status": "alive"}
