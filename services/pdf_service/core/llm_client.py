"""LLM client for extraction tasks."""

import os
from dataclasses import dataclass
from typing import Optional

import openai
from anthropic import AsyncAnthropic

from core.logger import setup_logging

logger = setup_logging()


@dataclass
class LLMConfig:
    """LLM configuration."""

    provider: str = "openai"  # "openai" or "anthropic"
    model: str = "gpt-4o-mini"
    api_key: Optional[str] = None
    base_url: Optional[str] = None
    max_tokens: int = 2000
    temperature: float = 0.0
    timeout: int = 60

    def __post_init__(self):
        """Load API key from environment if not provided."""
        if self.api_key is None:
            if self.provider == "openai":
                self.api_key = os.getenv("OPENAI_API_KEY")
            elif self.provider == "anthropic":
                self.api_key = os.getenv("ANTHROPIC_API_KEY")

        if self.api_key is None:
            raise ValueError(f"API key not found for provider: {self.provider}")


class LLMClient:
    """Client for LLM API calls."""

    def __init__(self, config: LLMConfig):
        """Initialize LLM client.

        Args:
            config: LLM configuration
        """
        self.config = config
        self._client = None

        if config.provider == "openai":
            self._client = openai.AsyncOpenAI(
                api_key=config.api_key,
                base_url=config.base_url,
                timeout=config.timeout,
            )
        elif config.provider == "anthropic":
            self._client = AsyncAnthropic(
                api_key=config.api_key,
                base_url=config.base_url,
                timeout=config.timeout,
            )
        else:
            raise ValueError(f"Unsupported provider: {config.provider}")

    async def chat(self, system_prompt: str, user_prompt: str) -> str:
        """Send a chat request to the LLM.

        Args:
            system_prompt: System prompt
            user_prompt: User prompt

        Returns:
            LLM response text
        """
        try:
            if self.config.provider == "openai":
                response = await self._client.chat.completions.create(
                    model=self.config.model,
                    messages=[
                        {"role": "system", "content": system_prompt},
                        {"role": "user", "content": user_prompt},
                    ],
                    max_tokens=self.config.max_tokens,
                    temperature=self.config.temperature,
                )
                return response.choices[0].message.content

            elif self.config.provider == "anthropic":
                response = await self._client.messages.create(
                    model=self.config.model,
                    max_tokens=self.config.max_tokens,
                    temperature=self.config.temperature,
                    system=system_prompt,
                    messages=[
                        {"role": "user", "content": user_prompt},
                    ],
                )
                return response.content[0].text

            else:
                raise ValueError(f"Unsupported provider: {self.config.provider}")

        except Exception as e:
            logger.error(f"Error in LLM chat: {e}")
            raise
