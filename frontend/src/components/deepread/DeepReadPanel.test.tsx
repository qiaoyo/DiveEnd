import type { ReactNode } from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useAppStore } from '../../stores/appStore';
import { defaultConfig } from '../../types';

const backendMocks = vi.hoisted(() => ({
  getDeepReadState: vi.fn(),
  getFolderTree: vi.fn(),
  getPapers: vi.fn(),
  prepareDeepReadPaper: vi.fn(),
  retryPaperDownload: vi.fn(),
  retryPaperDownloadWithURL: vi.fn(),
  saveConfig: vi.fn(),
  saveDeepReadNote: vi.fn(),
  translatePaperSection: vi.fn(),
}));

vi.mock('../../lib/backend', () => backendMocks);
vi.mock('react-pdf', () => ({
  pdfjs: {
    version: '3.0.0',
    GlobalWorkerOptions: {
      workerSrc: '',
    },
  },
  Document: ({ children }: { children: ReactNode }) => <div data-testid="mock-pdf-document">{children}</div>,
  Page: ({ pageNumber }: { pageNumber: number }) => <div data-testid="mock-pdf-page">page-{pageNumber}</div>,
}));

import { DeepReadPanel } from './DeepReadPanel';

describe('DeepReadPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks();

    backendMocks.getFolderTree.mockResolvedValue([
      {
        folder: {
          id: 'folder-1',
          name: 'Cache',
          path: 'Cache',
          parentId: null,
          isSystem: true,
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        },
        children: [
          {
            folder: {
              id: 'folder-2',
              name: 'Subtopic',
              path: 'Cache/Subtopic',
              parentId: 'folder-1',
              isSystem: false,
              createdAt: new Date().toISOString(),
              updatedAt: new Date().toISOString(),
            },
            children: [],
          },
        ],
      },
    ]);

    backendMocks.getPapers.mockImplementation(async (folderId: string) => {
      if (folderId === 'folder-2') {
        return [
          {
            id: 'paper-2',
            sourcePaperId: 'paper-2',
            title: 'Child Folder Paper',
            authors: 'Carol',
            abstract: 'child abstract',
            year: 2024,
            journal: 'RSS',
            url: 'https://example.org/paper-2',
            pdfPath: '',
            downloadStatus: 'failed',
            downloadError: 'no downloadable pdf url',
            folderId: 'folder-2',
            category: '',
            tags: [],
            addedAt: new Date().toISOString(),
            updatedAt: new Date().toISOString(),
          },
        ];
      }
      return [
        {
          id: 'paper-1',
          sourcePaperId: 'paper-1',
          title: 'Embodied Agent Study',
          authors: 'Alice, Bob',
          abstract: 'abstract',
          year: 2025,
          journal: 'ICRA',
          url: 'https://example.org/paper-1',
          pdfPath: '',
          downloadStatus: 'queued',
          downloadError: '',
          folderId: 'folder-1',
          category: '',
          tags: [],
          addedAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        },
      ];
    });

    backendMocks.getDeepReadState.mockResolvedValue({
      paperId: 'paper-1',
      hasPdf: false,
      pdfPath: '',
      parseStatus: 'missing_pdf',
      parseError: '本地 PDF 不可用，请先等待下载完成或手动导入 PDF。',
      sections: [],
      markdown: '',
      translations: [],
      notes: [],
      lastPreparedAt: new Date().toISOString(),
    });

    backendMocks.prepareDeepReadPaper.mockResolvedValue({
      paperId: 'paper-1',
      hasPdf: false,
      pdfPath: '',
      parseStatus: 'missing_pdf',
      parseError: '本地 PDF 不可用，请先等待下载完成或手动导入 PDF。',
      sections: [],
      markdown: '',
      translations: [],
      notes: [],
      lastPreparedAt: new Date().toISOString(),
    });

    backendMocks.retryPaperDownload.mockResolvedValue(undefined);
    backendMocks.retryPaperDownloadWithURL.mockResolvedValue(undefined);
    backendMocks.saveConfig.mockResolvedValue({
      config: defaultConfig,
      requiresRestart: false,
      message: '',
    });

    backendMocks.saveDeepReadNote.mockResolvedValue({
      id: 'note-1',
      paperId: 'paper-1',
      section: 'General',
      content: 'important insight',
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    });

    backendMocks.translatePaperSection.mockResolvedValue({
      id: 'translation-1',
      paperId: 'paper-1',
      section: 'General',
      originalText: 'text',
      translatedText: '翻译',
      summary: '摘要',
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    });

    useAppStore.setState({
      activePanel: 'deepread',
      activeFolderId: 'folder-1',
      selectedPaper: {
        id: 'paper-1',
        sourcePaperId: 'paper-1',
        title: 'Embodied Agent Study',
        authors: 'Alice, Bob',
        abstract: 'abstract',
        year: 2025,
        journal: 'ICRA',
        url: 'https://example.org/paper-1',
        pdfPath: '',
        downloadStatus: 'queued',
        downloadError: '',
        folderId: 'folder-1',
        category: '',
        tags: [],
        addedAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
      },
      papers: [],
      folders: [],
      deepStartSessions: [],
      activeDeepStartSession: null,
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

  it('renders missing-pdf fallback state and can trigger prepare', async () => {
    render(<DeepReadPanel />);

    expect(await screen.findByText('本地 PDF 不可用，请先等待下载完成或手动导入 PDF。')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: '准备阅读内容' }));
    await waitFor(() => {
      expect(backendMocks.prepareDeepReadPaper).toHaveBeenCalledWith('paper-1');
    });
  });

  it('saves deepread note via backend api', async () => {
    render(<DeepReadPanel />);

    const noteInput = await screen.findByPlaceholderText('记录你的阅读笔记');
    fireEvent.change(noteInput, { target: { value: 'important insight' } });
    fireEvent.click(screen.getByRole('button', { name: '保存笔记' }));

    await waitFor(() => {
      expect(backendMocks.saveDeepReadNote).toHaveBeenCalledWith('paper-1', 'Untitled Section', 'important insight');
    });
    expect(await screen.findByText('important insight')).toBeInTheDocument();
  });

  it('supports folder switching and retrying failed downloads in deepread library', async () => {
    render(<DeepReadPanel />);

    expect(await screen.findByText('文件夹目录')).toBeInTheDocument();
    expect(await screen.findByText('论文卡片集')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Cache/Subtopic' }));
    await waitFor(() => {
      expect(backendMocks.getPapers).toHaveBeenCalledWith('folder-2');
    });

    expect(await screen.findByText('no downloadable pdf url')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '重试' }));
    await waitFor(() => {
      expect(backendMocks.retryPaperDownload).toHaveBeenCalledWith('paper-2');
    });

    expect(await screen.findByText('填写可访问的 http/https PDF 链接')).toBeInTheDocument();
    fireEvent.change(screen.getByPlaceholderText('https://...'), {
      target: { value: 'https://example.org/manual-paper.pdf' },
    });
    fireEvent.click(screen.getByRole('button', { name: '使用该链接重试' }));

    await waitFor(() => {
      expect(backendMocks.retryPaperDownloadWithURL).toHaveBeenCalledWith(
        'paper-2',
        'https://example.org/manual-paper.pdf'
      );
    });
  });

  it('uses light-first theme container classes for deepread workspace', async () => {
    const { container } = render(<DeepReadPanel />);
    await screen.findByText('DeepRead Workspace');

    const root = container.firstElementChild as HTMLElement;
    expect(root.className).toContain('bg-slate-50');
    expect(root.className).toContain('dark:bg-slate-950');
  });
});
