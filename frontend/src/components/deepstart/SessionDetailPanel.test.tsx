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
  openExternalURL: vi.fn(),
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
            publicationVenue: 'NeurIPS',
            publicationYear: 2025,
            citationCount: 156,
            url: 'https://example.org/paper-1',
            category: 'benchmark',
            tags: ['robotics'],
            source: 'arxiv',
            institutions: ['CMU', 'OpenAI'],
            keywords: ['embodied', 'benchmark', 'robot policy'],
            sourceLabel: 'arXiv',
            matchReason: '标题命中关键词：embodied、benchmark。',
            matchedTerms: ['embodied', 'benchmark'],
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

    const chatTitle = screen.getByText('研究助理');
    const chatSection = chatTitle.closest('section');
    expect(chatSection).toBeTruthy();
    expect(chatSection?.parentElement).toHaveClass('sticky');
    expect(within(chatSection as HTMLElement).getByText('embodied benchmark')).toBeInTheDocument();

    expect(screen.getByText('Unified Embodied Agent Benchmark')).toBeInTheDocument();
    expect(screen.getByText('NeurIPS · 2025 · 引用 156')).toBeInTheDocument();
    expect(screen.getByText(/CMU · OpenAI/)).toBeInTheDocument();
    expect(screen.getByText('标题命中关键词：embodied、benchmark。')).toBeInTheDocument();
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

    expect(screen.getByText('论文详情')).toBeInTheDocument();
    expect(screen.getByText('作者列表')).toBeInTheDocument();
    expect(screen.getByText('检索匹配')).toBeInTheDocument();
    expect(screen.getByText('命中词：embodied · benchmark')).toBeInTheDocument();
    expect(screen.getByText('Alice')).toBeInTheDocument();
    expect(screen.getByText('完整摘要')).toBeInTheDocument();
  });

  it('copies the active title and opens its external link from the drawer', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText },
    });

    render(<SessionDetailPanel />);

    fireEvent.click(screen.getByText('Unified Embodied Agent Benchmark'));
    fireEvent.click(screen.getByRole('button', { name: '复制论文标题' }));
    fireEvent.click(screen.getByRole('button', { name: '打开链接' }));

    await waitFor(() => {
      expect(writeText).toHaveBeenCalledWith('Unified Embodied Agent Benchmark');
    });
    expect(backendMocks.openExternalURL).toHaveBeenCalledWith('https://example.org/paper-1');
  });

  it('redacts sensitive values in clipboard copy failures', async () => {
    const writeText = vi.fn().mockRejectedValueOnce(
      new Error('clipboard failed api_key=sk-clipboard-secret and access_token=clipboard-token-secret')
    );
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText },
    });

    render(<SessionDetailPanel />);

    fireEvent.click(screen.getByText('Unified Embodied Agent Benchmark'));
    fireEvent.click(screen.getByRole('button', { name: '复制摘要' }));

    await waitFor(() => {
      expect(writeText).toHaveBeenCalled();
      expect(document.body.textContent).toContain('api_key=[redacted]');
      expect(document.body.textContent).toContain('access_token=[redacted]');
    });
    expect(document.body.textContent).not.toContain('sk-clipboard-secret');
    expect(document.body.textContent).not.toContain('clipboard-token-secret');
  });

  it('opens create-folder modal and submits', async () => {
    render(<SessionDetailPanel />);

    fireEvent.click(screen.getByRole('button', { name: /新建文件夹/i }));
    expect(screen.getByRole('heading', { name: '新建文件夹' })).toBeInTheDocument();

    fireEvent.change(screen.getByPlaceholderText('输入路径，例如 Robotics/VLA/Benchmarks'), {
      target: { value: 'Robotics/VLA/My Folder' },
    });
    fireEvent.click(screen.getByRole('button', { name: /^创建$/ }));

    await waitFor(() => {
      expect(backendMocks.createFolderNode).toHaveBeenCalledWith(
        expect.objectContaining({ path: 'Robotics/VLA/My Folder' })
      );
    });
  });

  it('redacts sensitive values in create-folder modal errors', async () => {
    backendMocks.createFolderNode.mockRejectedValueOnce(
      new Error('create failed api_key=sk-folder-secret and access_token=folder-token-secret')
    );

    render(<SessionDetailPanel />);

    fireEvent.click(screen.getByRole('button', { name: /新建文件夹/i }));
    fireEvent.change(screen.getByPlaceholderText('输入路径，例如 Robotics/VLA/Benchmarks'), {
      target: { value: 'Robotics/VLA/My Folder' },
    });
    fireEvent.click(screen.getByRole('button', { name: /^创建$/ }));

    await waitFor(() => {
      expect(document.body.textContent).toContain('api_key=[redacted]');
      expect(document.body.textContent).toContain('access_token=[redacted]');
    });
    expect(document.body.textContent).not.toContain('sk-folder-secret');
    expect(document.body.textContent).not.toContain('folder-token-secret');
  });

  it('does not submit create-folder on Enter key', async () => {
    render(<SessionDetailPanel />);

    fireEvent.click(screen.getByRole('button', { name: /新建文件夹/i }));
    const input = screen.getByPlaceholderText('输入路径，例如 Robotics/VLA/Benchmarks');
    fireEvent.change(input, {
      target: { value: 'Robotics/VLA/My Folder' },
    });
    fireEvent.keyDown(input, { key: 'Enter', code: 'Enter' });

    await Promise.resolve();
    expect(backendMocks.createFolderNode).not.toHaveBeenCalled();
  });

  it('blocks create-folder when path contains invalid characters', async () => {
    render(<SessionDetailPanel />);

    fireEvent.click(screen.getByRole('button', { name: /新建文件夹/i }));
    fireEvent.change(screen.getByPlaceholderText('输入路径，例如 Robotics/VLA/Benchmarks'), {
      target: { value: 'Robotics/VLA/2026#bad' },
    });
    fireEvent.click(screen.getByRole('button', { name: /^创建$/ }));

    expect(await screen.findByText('目录路径仅支持中文、英文、空格和下划线（_）')).toBeInTheDocument();
    expect(backendMocks.createFolderNode).not.toHaveBeenCalled();
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

  it('redacts sensitive values in chat runtime failures', async () => {
    backendMocks.replyDeepStartSession.mockRejectedValue(
      new Error('LLM failed with api_key=sk-session-secret and access_token=session-token-secret')
    );
    render(<SessionDetailPanel />);

    fireEvent.change(screen.getByPlaceholderText('补充你的筛选偏好'), {
      target: { value: '请重新压缩候选' },
    });
    fireEvent.click(screen.getByRole('button', { name: '发送' }));

    await waitFor(() => {
      expect(backendMocks.replyDeepStartSession).toHaveBeenCalled();
      expect(document.body.textContent).toContain('api_key=[redacted]');
      expect(document.body.textContent).toContain('access_token=[redacted]');
    });
    expect(document.body.textContent).not.toContain('sk-session-secret');
    expect(document.body.textContent).not.toContain('session-token-secret');
  });

  it('redacts sensitive values in undo failures', async () => {
    backendMocks.undoDeepStartNarrow.mockRejectedValueOnce(
      new Error('undo failed api_key=sk-undo-secret and access_token=undo-token-secret')
    );

    render(<SessionDetailPanel />);

    fireEvent.click(screen.getByRole('button', { name: /回退上一轮/i }));

    await waitFor(() => {
      const error = useAppStore.getState().error ?? '';
      expect(error).toContain('api_key=[redacted]');
      expect(error).toContain('access_token=[redacted]');
      expect(error).not.toContain('sk-undo-secret');
      expect(error).not.toContain('undo-token-secret');
    });
  });

  it('redacts sensitive values in DeepStart progress event messages', async () => {
    let progressHandler: ((event: any) => void) | null = null;
    backendMocks.onDeepStartProgress.mockImplementation((handler) => {
      progressHandler = handler;
      return () => undefined;
    });

    render(<SessionDetailPanel />);

    await act(async () => {
      progressHandler?.({
        sessionId: 'session-1',
        phase: 'cancelling',
        message: 'stopping with Authorization: Bearer progress-token-secret and client_secret=progress-client-secret',
        elapsedSeconds: 1,
        estimatedRemainingSeconds: 1,
        total: 1,
        completed: 0,
        overallPercent: 12,
      });
    });

    expect(document.body.textContent).toContain('Bearer [redacted]');
    expect(document.body.textContent).toContain('client_secret=[redacted]');
    expect(document.body.textContent).not.toContain('progress-token-secret');
    expect(document.body.textContent).not.toContain('progress-client-secret');
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
