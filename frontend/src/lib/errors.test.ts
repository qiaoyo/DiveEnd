import { describe, expect, it } from 'vitest';
import { errorToUserMessage, sanitizeUserVisibleError } from './errors';

describe('frontend error sanitization', () => {
  it('redacts common API keys and bearer/query tokens from user-visible messages', () => {
    const message = sanitizeUserVisibleError(
      'request failed Authorization: Bearer sk-live-secret-12345 https://example.test?access_token=access-secret&refresh_token=refresh-secret api_key=sk-test-secret client_secret="client-secret"',
    );

    expect(message).toContain('[redacted]');
    expect(message).not.toContain('sk-live-secret-12345');
    expect(message).not.toContain('access-secret');
    expect(message).not.toContain('refresh-secret');
    expect(message).not.toContain('sk-test-secret');
    expect(message).not.toContain('client-secret');
  });

  it('formats unknown errors with a safe fallback', () => {
    expect(errorToUserMessage(null, '操作失败')).toBe('操作失败');
  });

  it('bounds very large user-visible error messages', () => {
    const message = sanitizeUserVisibleError('x'.repeat(2000));

    expect(message.length).toBeLessThan(1300);
    expect(message).toContain('[truncated]');
  });
});
