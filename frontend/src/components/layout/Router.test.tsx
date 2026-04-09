import { render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('./AppLayout', () => ({
  AppLayout: () => <div data-testid="app-layout">layout</div>,
}));

import { AppRouter } from './Router';

describe('AppRouter', () => {
  beforeEach(() => {
    window.location.hash = '#/screening';
  });

  it('boots under HashRouter routes without blank-screen navigation failure', () => {
    render(<AppRouter />);
    expect(screen.getByTestId('app-layout')).toBeInTheDocument();
  });
});
