import { afterEach, describe, expect, it } from 'vitest';
import { useAppStore } from './appStore';

describe('appStore error handling', () => {
  afterEach(() => {
    useAppStore.setState({ error: null });
  });

  it('redacts secrets before storing global user-visible errors', () => {
    useAppStore.getState().setError('LLM failed with api_key=sk-store-secret and token=baidu-token-secret');

    const error = useAppStore.getState().error ?? '';
    expect(error).toContain('[redacted]');
    expect(error).not.toContain('sk-store-secret');
    expect(error).not.toContain('baidu-token-secret');
  });
});
