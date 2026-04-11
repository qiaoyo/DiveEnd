import { fireEvent, render, screen, within } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useAppStore } from '../../stores/appStore';
import { defaultConfig } from '../../types';

const backendMocks = vi.hoisted(() => ({
  createFolder: vi.fn(),
  getPapers: vi.fn(),
  importPapers: vi.fn(),
  replyDeepStartSession: vi.fn(),
  rerunDeepStartSearch: vi.fn(),
  updateDeepStartSelections: vi.fn(),
}));

vi.mock('../../lib/backend', () => backendMocks);
vi.mock('react-router-dom', () => ({
  useNavigate: () => vi.fn(),
}));

import { SessionDetailPanel } from './SessionDetailPanel';

describe('SessionDetailPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    backendMocks.getPapers.mockResolvedValue([]);
    backendMocks.createFolder.mockResolvedValue({ id: 'folder-2', name: 'New Folder', createdAt: new Date().toISOString() });
    backendMocks.replyDeepStartSession.mockResolvedValue(null);
    backendMocks.rerunDeepStartSearch.mockResolvedValue(null);
    backendMocks.updateDeepStartSelections.mockResolvedValue(null);
    backendMocks.importPapers.mockResolvedValue([]);

    useAppStore.setState({
      activePanel: 'deepstart',
      activeFolderId: 'folder-1',
      selectedPaper: null,
      papers: [],
      folders: [{ id: 'folder-1', name: 'Inbox', createdAt: new Date().toISOString() }],
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
  });

  it('opens detail drawer and highlights abstract keywords', async () => {
    const { container } = render(<SessionDetailPanel />);

    fireEvent.click(screen.getByText('Unified Embodied Agent Benchmark'));

    expect(screen.getByText('Paper Detail')).toBeInTheDocument();
    expect(screen.getByText('作者列表')).toBeInTheDocument();
    expect(screen.getByText('Alice')).toBeInTheDocument();
    expect(screen.getByText('完整摘要（规则高亮）')).toBeInTheDocument();
    expect(container.querySelector('mark')).not.toBeNull();
  });

  it('can collapse chat bar', async () => {
    render(<SessionDetailPanel />);

    fireEvent.click(screen.getByRole('button', { name: /收起/i }));

    expect(screen.getByRole('button', { name: /展开/i })).toBeInTheDocument();
    expect(screen.queryByText('embodied benchmark')).not.toBeInTheDocument();
  });
});
