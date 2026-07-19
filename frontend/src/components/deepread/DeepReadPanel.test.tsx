import type { ReactNode } from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useAppStore } from '../../stores/appStore';
import { defaultConfig } from '../../types';

const backendMocks = vi.hoisted(() => ({
  getDeepReadState: vi.fn(),
  getPDFServiceStatus: vi.fn(),
  getFolderTree: vi.fn(),
  getPapers: vi.fn(),
  prepareDeepReadPaper: vi.fn(),
  retryFolderPendingDownloads: vi.fn(),
  retryPaperDownload: vi.fn(),
  retryPaperDownloadWithURL: vi.fn(),
  saveConfig: vi.fn(),
  saveDeepReadNote: vi.fn(),
  selectAndAttachPaperPDF: vi.fn(),
  translatePaperSection: vi.fn(),
  getDeepReadPDFURL: vi.fn(),
  getDeepReadPDFBytes: vi.fn(),
}));

vi.mock('../../lib/backend', () => backendMocks);
vi.mock('react-pdf', () => ({
  pdfjs: {
    version: '3.0.0',
    GlobalWorkerOptions: {
      workerSrc: '',
    },
  },
  Document: ({ children, file }: { children: ReactNode; file?: unknown }) => {
    const fileKind = typeof file === 'object' && file && 'data' in file ? 'bytes' : typeof file === 'string' ? file : 'empty';
    return (
      <div data-testid="mock-pdf-document" data-file-kind={fileKind}>
        {children}
      </div>
    );
  },
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
    backendMocks.retryFolderPendingDownloads.mockResolvedValue(1);
    backendMocks.retryPaperDownloadWithURL.mockResolvedValue(undefined);
    backendMocks.getDeepReadPDFURL.mockResolvedValue('');
    backendMocks.getDeepReadPDFBytes.mockResolvedValue('');
    backendMocks.getPDFServiceStatus.mockResolvedValue({
      enabled: true,
      url: 'http://127.0.0.1:50051',
      healthy: true,
      ready: true,
      checks: { pdf_parser: 'up' },
      checkedAt: new Date().toISOString(),
      message: 'PDF 服务 ready',
    });
    backendMocks.selectAndAttachPaperPDF.mockResolvedValue({
      id: 'paper-2',
      sourcePaperId: 'paper-2',
      title: 'Child Folder Paper',
      authors: 'Carol',
      abstract: 'child abstract',
      year: 2024,
      journal: 'RSS',
      url: 'https://example.org/paper-2',
      pdfPath: '/tmp/paper-2.pdf',
      downloadStatus: 'downloaded',
      downloadError: '',
      folderId: 'folder-2',
      category: '',
      tags: [],
      addedAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    });
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

  it('falls back to backend pdf bytes when the deepread asset url is unavailable', async () => {
    backendMocks.getDeepReadState.mockResolvedValueOnce({
      paperId: 'paper-1',
      hasPdf: true,
      pdfPath: '/managed/papers/paper-1.pdf',
      parseStatus: 'idle',
      parseError: '',
      sections: [
        {
          id: 'abstract',
          title: 'Abstract',
          content: 'abstract text',
          level: 1,
          index: 0,
        },
      ],
      markdown: '# Abstract\n\nabstract text',
      translations: [],
      notes: [],
      lastPreparedAt: new Date().toISOString(),
    });
    backendMocks.getDeepReadPDFURL.mockRejectedValueOnce(new Error('asset server unavailable'));
    backendMocks.getDeepReadPDFBytes.mockResolvedValueOnce(window.btoa('%PDF-1.4 fallback'));

    render(<DeepReadPanel />);

    await waitFor(() => {
      expect(backendMocks.getDeepReadPDFURL).toHaveBeenCalledWith('paper-1');
      expect(backendMocks.getDeepReadPDFBytes).toHaveBeenCalledWith('paper-1');
    });

    expect(await screen.findByTestId('mock-pdf-document')).toHaveAttribute('data-file-kind', 'bytes');
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

  it('redacts sensitive values in local translation errors', async () => {
    backendMocks.translatePaperSection.mockRejectedValueOnce(
      new Error('translation failed api_key=sk-translation-secret and access_token=translation-token-secret')
    );

    render(<DeepReadPanel />);

    expect(await screen.findByDisplayValue('abstract')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '生成翻译与摘要' }));

    await waitFor(() => {
      expect(document.body.textContent).toContain('api_key=[redacted]');
      expect(document.body.textContent).toContain('access_token=[redacted]');
    });
    expect(document.body.textContent).not.toContain('sk-translation-secret');
    expect(document.body.textContent).not.toContain('translation-token-secret');
  });

  it('redacts sensitive values in initial library load errors', async () => {
    backendMocks.getFolderTree.mockRejectedValueOnce(
      new Error('folder load failed api_key=sk-library-secret and access_token=library-token-secret')
    );

    render(<DeepReadPanel />);

    await waitFor(() => {
      const error = useAppStore.getState().error ?? '';
      expect(error).toContain('api_key=[redacted]');
      expect(error).toContain('access_token=[redacted]');
      expect(error).not.toContain('sk-library-secret');
      expect(error).not.toContain('library-token-secret');
    });
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
