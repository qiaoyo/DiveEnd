"""Configuration management for PDF service."""

import os
from functools import lru_cache
from typing import Any, Optional

from pydantic import Field
from pydantic_settings import BaseSettings, SettingsConfigDict


class LLMConfig(BaseSettings):
    """LLM provider configuration."""

    model_config = SettingsConfigDict(env_prefix="LLM_")

    provider: str = Field(default="openai", description="LLM provider: openai or anthropic")
    model: str = Field(default="gpt-4o-mini", description="Model name")
    api_key: Optional[str] = Field(default=None, description="API key")
    base_url: Optional[str] = Field(default=None, description="Custom base URL")
    max_tokens: int = Field(default=2000, description="Max tokens per request")
    temperature: float = Field(default=0.0, description="Sampling temperature")
    timeout: int = Field(default=30, description="Request timeout in seconds")

    def model_post_init(self, __context: Any) -> None:
        self.provider = (self.provider or "openai").strip().lower()
        self.model = (self.model or "").strip()
        if not self.model:
            self.model = "claude-sonnet-4-20250514" if self.provider == "anthropic" else "gpt-4o-mini"
        if self.api_key is not None:
            self.api_key = self.api_key.strip()
        if self.base_url is not None:
            self.base_url = self.base_url.strip() or None

        if not self.api_key:
            # Try to load from environment based on provider
            if self.provider == "openai":
                self.api_key = os.getenv("OPENAI_API_KEY")
            elif self.provider == "anthropic":
                self.api_key = os.getenv("ANTHROPIC_API_KEY")
            if self.api_key is not None:
                self.api_key = self.api_key.strip() or None


class PDFServiceConfig(BaseSettings):
    """PDF service configuration."""

    model_config = SettingsConfigDict(
        env_prefix="PDF_",
        env_file=".env",
        env_file_encoding="utf-8",
    )

    # Server settings
    host: str = Field(default="127.0.0.1", description="Server host")
    port: int = Field(default=50051, description="Server port")
    workers: int = Field(default=1, description="Number of worker processes")
    cors_allow_origin_regex: str = Field(
        default=r"^(http://(localhost|127\.0\.0\.1)(:\d+)?|wails://wails\.localhost)$",
        description="Allowed CORS origin regex",
    )

    # PDF processing settings
    max_pdf_size: int = Field(default=50*1024*1024, description="Max PDF size in bytes (50MB)")
    pdf_timeout: int = Field(default=120, description="PDF processing timeout in seconds")

    # Marker settings
    marker_batch_multiplier: int = Field(default=2, description="Marker batch multiplier")

    # LLM settings
    weak_llm: LLMConfig = Field(default_factory=lambda: LLMConfig())
    strong_llm: LLMConfig = Field(default_factory=lambda: LLMConfig(
        provider="anthropic",
        model="claude-sonnet-4-20250514",
        max_tokens=4000,
        temperature=0.3,
        timeout=60,
    ))


@lru_cache()
def get_config() -> PDFServiceConfig:
    """Get cached configuration instance."""
    return PDFServiceConfig()
