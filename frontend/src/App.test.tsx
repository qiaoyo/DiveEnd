import { render, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useAppStore } from './stores/appStore';

const backendMocks = vi.hoisted(() => ({
  getInitialState: vi.fn(),
}));

vi.mock('./lib/backend', () => backendMocks);
vi.mock('./components/layout/Router', () => ({
  AppRouter: () => <div data-testid="app-router" />,
}));

import App from './App';

describe('App initialization', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useAppStore.setState({ error: null, isHydrating: false });
  });

  it('redacts sensitive values from initialization failures', async () => {
    backendMocks.getInitialState.mockRejectedValueOnce(
      new Error('init failed api_key=sk-init-secret and access_token=init-token-secret')
    );

    render(<App />);

    await waitFor(() => {
      const error = useAppStore.getState().error ?? '';
      expect(error).toContain('api_key=[redacted]');
      expect(error).toContain('access_token=[redacted]');
      expect(error).not.toContain('sk-init-secret');
      expect(error).not.toContain('init-token-secret');
    });
  });
});
