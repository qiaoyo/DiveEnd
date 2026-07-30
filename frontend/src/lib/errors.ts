const REDACTED = '[redacted]';
const MAX_USER_ERROR_LENGTH = 1200;

type RedactionRule = {
  pattern: RegExp;
  replace: string | ((substring: string, ...args: string[]) => string);
};

const redactionRules: RedactionRule[] = [
  {
    pattern: /\b(Authorization\s*[:=]\s*)Bearer\s+([A-Za-z0-9._~+/=-]{8,})/gi,
    replace: (_match, prefix: string) => `${prefix}Bearer ${REDACTED}`,
  },
  {
    pattern: /\bBearer\s+([A-Za-z0-9._~+/=-]{8,})/gi,
    replace: `Bearer ${REDACTED}`,
  },
  {
    pattern: /\b(sk-[A-Za-z0-9_-]{8,})\b/g,
    replace: REDACTED,
  },
  {
    pattern:
      /([?&](?:access_token|refresh_token|api_key|apikey|key|token|client_secret|clientsecret)=)([^&#\s]+)/gi,
    replace: (_match, prefix: string) => `${prefix}${REDACTED}`,
  },
  {
    pattern:
      /\b((?:api[_-]?key|apikey|access[_-]?token|refresh[_-]?token|client[_-]?secret|clientsecret|token)\s*[:=]\s*)("[^"]*"|'[^']*'|[^\s,;)}\]]+)/gi,
    replace: (_match, prefix: string) => `${prefix}${REDACTED}`,
  },
  {
    pattern:
      /(["']?(?:api[_-]?key|apiKey|access[_-]?token|accessToken|refresh[_-]?token|refreshToken|client[_-]?secret|clientSecret|token)["']?\s*:\s*)("[^"]*"|'[^']*'|[^\s,}]+)/gi,
    replace: (_match, prefix: string) => `${prefix}${REDACTED}`,
  },
];

export function sanitizeUserVisibleError(message: string): string {
  const trimmed = String(message ?? '').trim();
  if (!trimmed) {
    return '';
  }

  let sanitized = trimmed;
  for (const rule of redactionRules) {
    if (typeof rule.replace === 'string') {
      sanitized = sanitized.replace(rule.pattern, rule.replace);
    } else {
      const replace = rule.replace;
      sanitized = sanitized.replace(rule.pattern, (substring, ...args) =>
        replace(substring, ...args.map((arg) => String(arg))),
      );
    }
  }

  if (sanitized.length <= MAX_USER_ERROR_LENGTH) {
    return sanitized;
  }
  return `${sanitized.slice(0, MAX_USER_ERROR_LENGTH)}... [truncated]`;
}

export function errorToUserMessage(error: unknown, fallback: string): string {
  if (error instanceof Error && error.message.trim()) {
    return sanitizeUserVisibleError(error.message);
  }
  if (typeof error === 'string' && error.trim()) {
    return sanitizeUserVisibleError(error);
  }
  return fallback;
}

export function isCancellationError(error: unknown): boolean {
  const message =
    error instanceof Error
      ? error.message
      : typeof error === 'string'
        ? error
        : '';
  return /\bcancel+ed\b/i.test(message);
}
