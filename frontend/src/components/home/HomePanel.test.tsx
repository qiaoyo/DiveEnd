import { fireEvent, render, screen } from '@testing-library/react';
import { MemoryRouter, useLocation } from 'react-router-dom';
import { beforeEach, describe, expect, it } from 'vitest';
import { HomePanel } from './HomePanel';
import { useAppStore } from '../../stores/appStore';
import type { DeepStartSessionSummary, Folder, Paper } from '../../types';

function LocationProbe() {
  const location = useLocation();
  return <output data-testid="location">{location.pathname}</output>;
}

const session: DeepStartSessionSummary = {
  id: 'session-1',
  title: '长任务中的代码智能体可靠性',
  rootPrompt: '哪些方法真正提升了代码智能体在长任务中的可靠性？',
  currentQuery: '代码智能体可靠性 benchmark',
  targetFolderId: 'folder-1',
  processingStatus: 'completed',
  updatedAt: '2026-08-05T08:00:00.000Z',
  createdAt: '2026-08-05T07:00:00.000Z',
};

const folder: Folder = {
  id: 'folder-1',
  name: '研究',
  path: '研究',
  isSystem: true,
  createdAt: '2026-08-01T00:00:00.000Z',
};

const paper = { id: 'paper-1', title: 'A paper' } as Paper;

describe('HomePanel', () => {
  beforeEach(() => {
    useAppStore.setState({
      deepStartSessions: [session],
      folders: [folder],
      papers: [paper],
    });
  });

  it('shows the research desk and current workspace counts', () => {
    render(
      <MemoryRouter initialEntries={['/home']}>
        <HomePanel />
      </MemoryRouter>,
    );

    expect(screen.getByRole('heading', { name: '从问题到结论' })).toBeInTheDocument();
    expect(screen.getAllByText('长任务中的代码智能体可靠性')).toHaveLength(1);
    expect(screen.getByText('论文')).toBeInTheDocument();
    expect(screen.getByText('研究空间')).toBeInTheDocument();
    expect(screen.queryByText('工作区概览')).not.toBeInTheDocument();
    expect(screen.queryByText('最近留下的线索')).not.toBeInTheDocument();
    expect(screen.getByText('个人研究工作台')).toBeInTheDocument();
  });

  it('carries a typed research question into discovery', () => {
    render(
      <MemoryRouter initialEntries={['/home']}>
        <HomePanel />
        <LocationProbe />
      </MemoryRouter>,
    );

    fireEvent.change(screen.getByRole('textbox'), {
      target: { value: 'VLA 在真实机器人任务上如何评测？' },
    });
    fireEvent.click(screen.getByRole('button', { name: '带着问题去发现' }));

    expect(screen.getByTestId('location')).toHaveTextContent('/discover');
  });
});
