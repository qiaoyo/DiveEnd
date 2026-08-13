import { fireEvent, render, screen } from '@testing-library/react';
import { MemoryRouter, useLocation } from 'react-router-dom';
import { beforeEach, describe, expect, it } from 'vitest';
import { GlobalNav } from './GlobalNav';
import { useAppStore } from '../../stores/appStore';

function LocationProbe() {
  const location = useLocation();
  return <output data-testid="location">{location.pathname}</output>;
}

describe('GlobalNav', () => {
  beforeEach(() => {
    useAppStore.setState({
      config: { ...useAppStore.getState().config, theme: 'light' },
      theme: 'light',
      error: null,
    });
  });

  it('toggles utility pages and returns to the previous workspace', () => {
    render(
      <MemoryRouter initialEntries={['/home']}>
        <GlobalNav />
        <LocationProbe />
      </MemoryRouter>,
    );

    fireEvent.click(screen.getByRole('button', { name: '研究记录' }));
    expect(screen.getByTestId('location')).toHaveTextContent('/history');

    fireEvent.click(screen.getByRole('button', { name: '研究记录' }));
    expect(screen.getByTestId('location')).toHaveTextContent('/home');
  });

  it('keeps the global theme control in the utility area', () => {
    render(
      <MemoryRouter initialEntries={['/discover']}>
        <GlobalNav />
      </MemoryRouter>,
    );

    expect(screen.getByRole('button', { name: '切换到深色模式' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '研究记录' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '同步' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '设置' })).toBeInTheDocument();
  });
});
