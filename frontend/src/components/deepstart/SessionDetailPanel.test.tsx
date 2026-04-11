import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useAppStore } from '../../stores/appStore';
import { defaultConfig } from '../../types';

const backendMocks = vi.hoisted(() => ({
  cancelDeepStartTask: vi.fn(),
  createFolderNode: vi.fn(),
  deleteFolderNode: vi.fn(),
  getFolderStorageTreeOverview: vi.fn(),
  getFolderTree: vi.fn(),
  getPapers: vi.fn(),
  importPapersWithAssets: vi.fn(),
  onDeepStartProgress: vi.fn(),
  replyDeepStartSession: vi.fn(),
  rerunDeepStartSearch: vi.fn(),
  undoDeepStartNarrow: vi.fn(),
  updateDeepStartSelections: vi.fn(),
}));

vi.mock('../../lib/backend', () => backendMocks);
vi.mock('react-router-dom', () => ({
  useNavigate: () => vi.fn(),
}));

import { SessionDetailPanel } from './SessionDetailPanel';

describe('SessionDetailPanel', () => {
  const buildDetail = () => {
    const detail = useAppStore.getState().activeDeepStartSession;
    if (!detail) {
      throw new Error('missing active deepstart session in test store');
    }
    return detail;
  };

  beforeEach(() => {
    vi.clearAllMocks();
    backendMocks.getPapers.mockResolvedValue([]);
    backendMocks.onDeepStartProgress.mockReturnValue(() => undefined);
    backendMocks.createFolderNode.mockResolvedValue({
      id: 'folder-2',
      name: 'New Folder',
      path: 'Cache/New Folder',
      parentId: 'folder-1',
      isSystem: false,
      createdAt: new Date().toISOString(),
    });
    backendMocks.getFolderTree.mockResolvedValue([
      {
        folder: {
          id: 'folder-1',
          name: 'Cache',
          parentId: '',
          path: 'Cache',
          isSystem: true,
          createdAt: new Date().toISOString(),
        },
        children: [],
      },
    ]);
    backendMocks.getFolderStorageTreeOverview.mockResolvedValue({
      rootPath: '/tmp/papers',
      directories: [
        {
          folderId: 'folder-1',
          folderName: 'Cache',
          folderPath: '/tmp/papers/folder-1',
          paperCount: 1,
          queued: 1,
          downloading: 0,
          downloaded: 0,
          failed: 0,
          children: [],
        },
      ],
      generatedAt: new Date().toISOString(),
    });
    backendMocks.replyDeepStartSession.mockResolvedValue(null);
    backendMocks.rerunDeepStartSearch.mockResolvedValue(null);
    backendMocks.updateDeepStartSelections.mockResolvedValue(null);
    backendMocks.importPapersWithAssets.mockResolvedValue({
      imported: [],
      skipped: [],
      queued: 0,
      message: 'ok',
    });

    useAppStore.setState({
      activePanel: 'deepstart',
      activeFolderId: 'folder-1',
      selectedPaper: null,
      papers: [],
      folders: [
        {
          id: 'folder-1',
          name: 'Cache',
          parentId: '',
          path: 'Cache',
          isSystem: true,
          createdAt: new Date().toISOString(),
        },
      ],
      deepStartSessions: [],
      activeDeepStartSession: {
        summary: {
          id: 'session-1',
          title: 'Embodied Intelligence',
          rootPrompt: 'embodied intelligence',
          currentQuery: 'embodied intelligence benchmark',
          targetFolderId: '',
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        },
        messages: [
          {
            id: 'msg-1',
            sessionId: 'session-1',
            role: 'assistant',
            content: '先看 survey，再看 benchmark。',
            createdAt: new Date().toISOString(),
          },
        ],
        currentResults: [
          {
            id: 'paper-1',
            title: 'Unified Embodied Agent Benchmark',
            authors: 'Alice, Bob',
            abstract: 'This embodied benchmark studies robot policy transfer in realistic scenarios.',
            year: 2025,
            journal: 'arXiv',
            url: 'https://example.org/paper-1',
            category: 'benchmark',
            tags: ['robotics'],
            source: 'arxiv',
            institutions: ['CMU', 'OpenAI'],
            keywords: ['embodied', 'benchmark', 'robot policy'],
            sourceLabel: 'arXiv',
          },
        ],
        currentAnalysis: {
          overview: '这是本轮探索概览。',
          directions: [
            {
              id: 'd-1',
              name: 'Benchmark',
              summary: '围绕具身智能评测',
              why: '先看 benchmark 更容易对齐任务边界。',
              paperIds: ['paper-1'],
            },
          ],
          paperNotes: [
            {
              paperId: 'paper-1',
              tier: 'core',
              reason: 'embodied benchmark 代表作',
              directionIds: ['d-1'],
            },
          ],
          followUpQuestions: [],
          suggestedQueries: ['embodied benchmark'],
          recommendedPaperIds: ['paper-1'],
          retainedPaperIds: ['paper-1'],
          searchStats: {
            query: 'embodied intelligence benchmark',
            rawCount: 210,
            dedupCount: 200,
            finalCount: 200,
          },
        },
        selectedPaperIds: [],
      },
      searchQuery: '',
      searchResults: [],
      translations: [],
      config: defaultConfig,
      leftPanelCollapsed: false,
      rightPanelCollapsed: false,
      theme: 'light',
      isHydrating: false,
      isSearching: false,
      isSavingConfig: false,
      isTranslating: false,
      error: null,
    });
  });

  it('renders sticky AI chat with suggested queries and compact paper cards', () => {
    render(<SessionDetailPanel />);

    const chatTitle = screen.getByText('AI Chat');
    const chatSection = chatTitle.closest('section');
    expect(chatSection).toBeTruthy();
    expect(chatSection).toHaveClass('sticky');
    expect(within(chatSection as HTMLElement).getByText('embodied benchmark')).toBeInTheDocument();

    expect(screen.getByText('Unified Embodied Agent Benchmark')).toBeInTheDocument();
    expect(screen.getByText('arXiv · 2025')).toBeInTheDocument();
    expect(screen.getByText('CMU · OpenAI')).toBeInTheDocument();
    expect(screen.getByText('robot policy')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '选择' })).toBeInTheDocument();
  });

  it('fills chat input when suggested query is clicked instead of triggering rerun', async () => {
    render(<SessionDetailPanel />);

    fireEvent.click(screen.getByRole('button', { name: 'embodied benchmark' }));

    expect(screen.getByPlaceholderText('补充你的筛选偏好')).toHaveValue('embodied benchmark');
    expect(backendMocks.rerunDeepStartSearch).not.toHaveBeenCalled();
  });

  it('opens detail drawer and shows highlighted full abstract', async () => {
    render(<SessionDetailPanel />);

    fireEvent.click(screen.getByText('Unified Embodied Agent Benchmark'));

    expect(screen.getByText('Paper Detail')).toBeInTheDocument();
    expect(screen.getByText('作者列表')).toBeInTheDocument();
    expect(screen.getByText('Alice')).toBeInTheDocument();
    expect(screen.getByText('完整摘要')).toBeInTheDocument();
  });

  it('opens create-folder modal and submits', async () => {
    render(<SessionDetailPanel />);

    fireEvent.click(screen.getByRole('button', { name: /新建文件夹/i }));
    expect(screen.getByRole('heading', { name: '新建文件夹' })).toBeInTheDocument();

    fireEvent.change(screen.getByPlaceholderText('目录名（可选）'), {
      target: { value: 'My Folder' },
    });
    fireEvent.click(screen.getByRole('button', { name: /^创建$/ }));

    await waitFor(() => {
      expect(backendMocks.createFolderNode).toHaveBeenCalledWith(
        expect.objectContaining({ name: 'My Folder' })
      );
    });
  });

  it('can collapse chat bar', async () => {
    render(<SessionDetailPanel />);

    fireEvent.click(screen.getByRole('button', { name: /收起/i }));

    expect(screen.getByRole('button', { name: /展开/i })).toBeInTheDocument();
    expect(screen.queryByText('embodied benchmark')).not.toBeInTheDocument();
  });

  it('sends chat reply via replyDeepStartSession path only', async () => {
    backendMocks.replyDeepStartSession.mockResolvedValue(buildDetail());
    render(<SessionDetailPanel />);

    fireEvent.change(screen.getByPlaceholderText('补充你的筛选偏好'), {
      target: { value: '请在当前候选里重点看 benchmark 对比设置' },
    });
    fireEvent.click(screen.getByRole('button', { name: '发送' }));

    await waitFor(() => {
      expect(backendMocks.replyDeepStartSession).toHaveBeenCalledWith(
        'session-1',
        '请在当前候选里重点看 benchmark 对比设置'
      );
    });
    expect(backendMocks.rerunDeepStartSearch).not.toHaveBeenCalled();
  });

  it('keeps top rerun path separated from chat reply path', async () => {
    backendMocks.rerunDeepStartSearch.mockResolvedValue(buildDetail());
    render(<SessionDetailPanel />);

    fireEvent.click(screen.getByRole('button', { name: '重搜' }));

    await waitFor(() => {
      expect(backendMocks.rerunDeepStartSearch).toHaveBeenCalledWith('session-1', 'embodied intelligence benchmark');
    });
    expect(backendMocks.replyDeepStartSession).not.toHaveBeenCalled();
  });

  it('can stop a running rerun task', async () => {
    let resolveRerun: ((value: any) => void) | null = null;
    backendMocks.rerunDeepStartSearch.mockImplementation(
      () =>
        new Promise((resolve) => {
          resolveRerun = resolve;
        })
    );
    render(<SessionDetailPanel />);

    fireEvent.click(screen.getByRole('button', { name: '重搜' }));
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /停止/i })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /停止/i }));
    await waitFor(() => {
      expect(backendMocks.cancelDeepStartTask).toHaveBeenCalledWith('session-1');
    });

    await act(async () => {
      resolveRerun?.(buildDetail());
      await Promise.resolve();
    });
  });
});
