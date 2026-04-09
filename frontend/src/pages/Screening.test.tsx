import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const backendMocks = vi.hoisted(() => ({
  canResolveFilePaths: vi.fn(),
  resolveFilePaths: vi.fn(),
  onExtractProgress: vi.fn(),
  createScreeningSession: vi.fn(),
  uploadScreeningFiles: vi.fn(),
  extractPaperContent: vi.fn(),
  getScreeningSession: vi.fn(),
  analyzePapers: vi.fn(),
  applyScreeningChoice: vi.fn(),
  completeScreening: vi.fn(),
  getPapers: vi.fn(),
}));

vi.mock('../lib/backend', () => backendMocks);

import { Screening } from './Screening';

describe('Screening page', () => {
  beforeEach(() => {
    vi.clearAllMocks();

    backendMocks.canResolveFilePaths.mockReturnValue(true);
    backendMocks.resolveFilePaths.mockReturnValue(['/tmp/paper-1.pdf', '/tmp/paper-2.pdf']);
    backendMocks.onExtractProgress.mockReturnValue(() => undefined);
    backendMocks.createScreeningSession.mockResolvedValue({
      id: 'session-1',
      title: 'Screening',
      status: 'upload',
      totalPapers: 0,
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    });
    backendMocks.uploadScreeningFiles.mockResolvedValue({
      session: {
        id: 'session-1',
        title: 'Screening',
        status: 'extract',
        totalPapers: 2,
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
      },
      papers: [
        {
          id: 'paper-1',
          sessionId: 'session-1',
          fileName: 'paper-1.pdf',
          filePath: '/tmp/paper-1.pdf',
          fileSize: 100,
          status: 'pending',
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        },
        {
          id: 'paper-2',
          sessionId: 'session-1',
          fileName: 'paper-2.pdf',
          filePath: '/tmp/paper-2.pdf',
          fileSize: 100,
          status: 'pending',
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        },
      ],
      currentNode: null,
      pathHistory: [],
    });
    backendMocks.extractPaperContent.mockResolvedValue({
      sessionId: 'session-1',
      total: 2,
      completed: 2,
      currentFile: '',
      status: 'completed',
      errorMessage: '',
    });
    backendMocks.getScreeningSession
      .mockResolvedValueOnce({
        session: {
          id: 'session-1',
          title: 'Screening',
          status: 'screen',
          totalPapers: 2,
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        },
        papers: [
          {
            id: 'paper-1',
            sessionId: 'session-1',
            fileName: 'paper-1.pdf',
            filePath: '/tmp/paper-1.pdf',
            fileSize: 100,
            status: 'screening',
            title: 'Paper One',
            authors: 'Author One',
            abstract: 'Paper one abstract',
            createdAt: new Date().toISOString(),
            updatedAt: new Date().toISOString(),
          },
          {
            id: 'paper-2',
            sessionId: 'session-1',
            fileName: 'paper-2.pdf',
            filePath: '/tmp/paper-2.pdf',
            fileSize: 100,
            status: 'screening',
            title: 'Paper Two',
            authors: 'Author Two',
            abstract: 'Paper two abstract',
            createdAt: new Date().toISOString(),
            updatedAt: new Date().toISOString(),
          },
        ],
        currentNode: null,
        pathHistory: [],
      })
      .mockResolvedValueOnce({
        session: {
          id: 'session-1',
          title: 'Screening',
          status: 'screen',
          totalPapers: 2,
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        },
        papers: [
          {
            id: 'paper-1',
            sessionId: 'session-1',
            fileName: 'paper-1.pdf',
            filePath: '/tmp/paper-1.pdf',
            fileSize: 100,
            status: 'screening',
            title: 'Paper One',
            authors: 'Author One',
            abstract: 'Paper one abstract',
            createdAt: new Date().toISOString(),
            updatedAt: new Date().toISOString(),
          },
        ],
        currentNode: null,
        pathHistory: [{ dimension: '研究子方向', choice: '综述' }],
      })
      .mockResolvedValue({
        session: {
          id: 'session-1',
          title: 'Screening',
          status: 'complete',
          totalPapers: 2,
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        },
        papers: [
          {
            id: 'paper-1',
            sessionId: 'session-1',
            fileName: 'paper-1.pdf',
            filePath: '/tmp/paper-1.pdf',
            fileSize: 100,
            status: 'selected',
            title: 'Paper One',
            authors: 'Author One',
            abstract: 'Paper one abstract',
            createdAt: new Date().toISOString(),
            updatedAt: new Date().toISOString(),
          },
        ],
        currentNode: null,
        pathHistory: [{ dimension: '研究子方向', choice: '综述' }],
      });
    backendMocks.analyzePapers.mockResolvedValue({
      id: 'node-1',
      nodeType: 'branch',
      message: '按研究子方向筛选',
      dimension: '研究子方向',
      options: [
        { key: 'survey', label: '综述', paperIds: ['paper-1'], count: 1 },
        { key: 'benchmark', label: '基准', paperIds: ['paper-2'], count: 1 },
      ],
      allowMultiSelect: true,
      allowSkip: false,
      remainingPaperIds: ['paper-1', 'paper-2'],
    });
    backendMocks.applyScreeningChoice.mockResolvedValue({
      id: 'node-2',
      nodeType: 'complete',
      message: '筛选完成，请确认导入。',
      dimension: '结果确认',
      options: [],
      allowMultiSelect: false,
      allowSkip: false,
      remainingPaperIds: ['paper-1'],
    });
    backendMocks.completeScreening.mockResolvedValue([
      {
        id: 'paper-1',
        title: 'Paper One',
        authors: 'Author One',
        abstract: 'Paper one abstract',
        year: 2024,
        journal: '',
        url: '',
        pdfPath: '/tmp/paper-1.pdf',
        folderId: 'folder-1',
        category: 'survey',
        tags: ['survey'],
        addedAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
      },
    ]);
    backendMocks.getPapers.mockResolvedValue([]);
  });

  it('resolves file paths, advances screening, and imports selected papers', async () => {
    const { container } = render(<Screening />);
    const input = container.querySelector('input[type="file"]') as HTMLInputElement;

    fireEvent.change(input, {
      target: {
        files: [new File(['pdf'], 'paper-1.pdf', { type: 'application/pdf' })],
      },
    });

    expect(await screen.findByText('按研究子方向筛选')).toBeInTheDocument();

    fireEvent.click(screen.getByText('综述'));
    fireEvent.click(screen.getByRole('button', { name: '继续下一步' }));

    expect(await screen.findByText('筛选完成')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: '导入到文库' }));

    await waitFor(() => {
      expect(screen.getByText('已成功导入 1 篇论文到文库。')).toBeInTheDocument();
    });

    expect(backendMocks.resolveFilePaths).toHaveBeenCalled();
    expect(backendMocks.completeScreening).toHaveBeenCalledWith('session-1', '');
  });

  it('shows extraction errors instead of silently falling back to demo state', async () => {
    backendMocks.extractPaperContent.mockRejectedValueOnce(new Error('pdf service unavailable'));

    const { container } = render(<Screening />);
    const input = container.querySelector('input[type="file"]') as HTMLInputElement;

    fireEvent.change(input, {
      target: {
        files: [new File(['pdf'], 'paper-1.pdf', { type: 'application/pdf' })],
      },
    });

    expect(await screen.findByText('pdf service unavailable')).toBeInTheDocument();
    expect(screen.getByText('提取论文内容')).toBeInTheDocument();
  });
});
