import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import { useAppStore } from '../../stores/appStore';
import { defaultConfig } from '../../types';

const backendMocks = vi.hoisted(() => ({
  createFolder: vi.fn(),
  deleteFolderNode: vi.fn(),
  deletePaper: vi.fn(),
  getFolders: vi.fn(),
  getPapers: vi.fn(),
  moveFolderNode: vi.fn(),
  movePapersToFolder: vi.fn(),
  movePaperToFolder: vi.fn(),
  renameFolderNode: vi.fn(),
}));

vi.mock('../../lib/backend', () => backendMocks);

import { PaperListPanel } from './PaperListPanel';

describe('PaperListPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useAppStore.setState({
      activePanel: 'deepstart',
      activeFolderId: 'folder-1',
      selectedPaper: null,
      papers: [
        {
          id: 'paper-1',
          sourcePaperId: 'paper-1',
          title: 'Secure Paper Deletion',
          authors: 'Alice',
          abstract: 'abstract',
          year: 2026,
          journal: 'Local',
          url: 'https://example.org/paper-1',
          pdfPath: '',
          downloadStatus: 'downloaded',
          downloadError: '',
          folderId: 'folder-1',
          category: '',
          tags: [],
          addedAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        },
      ],
      folders: [
        {
          id: 'folder-1',
          name: 'Library',
          path: 'Library',
          parentId: '',
          isSystem: true,
          createdAt: new Date().toISOString(),
        },
      ],
      deepStartSessions: [],
      activeDeepStartSession: null,
      searchQuery: '',
      searchResults: [],
      translations: [],
      config: defaultConfig,
      leftPanelCollapsed: false,
      rightPanelCollapsed: false,
      theme: defaultConfig.theme,
      isHydrating: false,
      isSearching: false,
      isSavingConfig: false,
      isTranslating: false,
      error: null,
    });
  });

  it('redacts sensitive create-folder errors before showing global error state', async () => {
    backendMocks.createFolder.mockRejectedValueOnce(
      new Error('create folder failed api_key=sk-paper-list-secret access_token=paper-list-token-secret')
    );

    render(
      <MemoryRouter>
        <PaperListPanel />
      </MemoryRouter>
    );

    fireEvent.click(screen.getByTitle('新建文件夹'));
    fireEvent.change(screen.getByPlaceholderText('新文件夹名称'), { target: { value: 'New Library' } });
    fireEvent.click(screen.getByRole('button', { name: '创建' }));

    await waitFor(() => {
      expect(useAppStore.getState().error).toContain('[redacted]');
    });
    const error = useAppStore.getState().error ?? '';
    expect(error).not.toContain('sk-paper-list-secret');
    expect(error).not.toContain('paper-list-token-secret');
  });

  it('redacts sensitive delete-paper errors before showing global error state', async () => {
    backendMocks.deletePaper.mockRejectedValueOnce(
      new Error('delete failed Authorization: Bearer sk-delete-secret token=paper-delete-token-secret')
    );

    render(
      <MemoryRouter>
        <PaperListPanel />
      </MemoryRouter>
    );

    fireEvent.click(screen.getByTitle('删除论文'));

    await waitFor(() => {
      expect(useAppStore.getState().error).toContain('[redacted]');
    });
    const error = useAppStore.getState().error ?? '';
    expect(error).not.toContain('sk-delete-secret');
    expect(error).not.toContain('paper-delete-token-secret');
  });

  it('deletes non-system folders and switches to the remaining system folder', async () => {
    backendMocks.deleteFolderNode.mockResolvedValueOnce(undefined);
    backendMocks.getPapers.mockResolvedValueOnce([]);
    useAppStore.setState({
      activeFolderId: 'folder-2',
      selectedPaper: useAppStore.getState().papers[0],
      folders: [
        {
          id: 'folder-1',
          name: 'Library',
          path: 'Library',
          parentId: '',
          isSystem: true,
          createdAt: new Date().toISOString(),
        },
        {
          id: 'folder-2',
          name: 'Drafts',
          path: 'Drafts',
          parentId: '',
          isSystem: false,
          createdAt: new Date().toISOString(),
        },
        {
          id: 'folder-3',
          name: 'Nested',
          path: 'Drafts/Nested',
          parentId: 'folder-2',
          isSystem: false,
          createdAt: new Date().toISOString(),
        },
      ],
    });

    render(
      <MemoryRouter>
        <PaperListPanel />
      </MemoryRouter>
    );

    expect(screen.queryByTitle('删除文件夹 Library')).not.toBeInTheDocument();
    fireEvent.click(screen.getByTitle('删除文件夹 Drafts'));

    expect(await screen.findByText('确认删除文件夹')).toBeInTheDocument();
    expect(screen.getByText(/子文件夹数量：1/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '确认删除' }));

    await waitFor(() => {
      expect(backendMocks.deleteFolderNode).toHaveBeenCalledWith('folder-2');
    });
    await waitFor(() => {
      expect(useAppStore.getState().activeFolderId).toBe('folder-1');
    });
    expect(useAppStore.getState().selectedPaper).toBeNull();
    expect(useAppStore.getState().folders.map((folder) => folder.id)).toEqual(['folder-1']);
    expect(backendMocks.getPapers).toHaveBeenCalledWith('folder-1');

  });

  it('renames non-system folders and refreshes current papers', async () => {
    const updatedPaper = {
      ...useAppStore.getState().papers[0],
      title: 'Renamed Folder Paper',
    };
    backendMocks.renameFolderNode.mockResolvedValueOnce({
      id: 'folder-2',
      name: 'Renamed',
      path: 'Renamed',
      parentId: '',
      isSystem: false,
      createdAt: new Date().toISOString(),
    });
    backendMocks.getFolders.mockResolvedValueOnce([
      {
        id: 'folder-1',
        name: 'Library',
        path: 'Library',
        parentId: '',
        isSystem: true,
        createdAt: new Date().toISOString(),
      },
      {
        id: 'folder-2',
        name: 'Renamed',
        path: 'Renamed',
        parentId: '',
        isSystem: false,
        createdAt: new Date().toISOString(),
      },
    ]);
    backendMocks.getPapers.mockResolvedValueOnce([updatedPaper]);
    useAppStore.setState({
      activeFolderId: 'folder-2',
      selectedPaper: useAppStore.getState().papers[0],
      folders: [
        {
          id: 'folder-1',
          name: 'Library',
          path: 'Library',
          parentId: '',
          isSystem: true,
          createdAt: new Date().toISOString(),
        },
        {
          id: 'folder-2',
          name: 'Drafts',
          path: 'Drafts',
          parentId: '',
          isSystem: false,
          createdAt: new Date().toISOString(),
        },
      ],
    });

    render(
      <MemoryRouter>
        <PaperListPanel />
      </MemoryRouter>
    );

    fireEvent.click(screen.getByTitle('重命名文件夹 Drafts'));
    fireEvent.change(screen.getByPlaceholderText('新的文件夹名称'), { target: { value: 'Renamed' } });
    fireEvent.click(screen.getByRole('button', { name: '保存' }));

    await waitFor(() => {
      expect(backendMocks.renameFolderNode).toHaveBeenCalledWith({ folderId: 'folder-2', name: 'Renamed' });
    });
    expect(backendMocks.getFolders).toHaveBeenCalledTimes(1);
    expect(backendMocks.getPapers).toHaveBeenCalledWith('folder-2');
    await waitFor(() => {
      expect(useAppStore.getState().folders.find((folder) => folder.id === 'folder-2')?.name).toBe('Renamed');
    });
    expect(useAppStore.getState().papers[0].title).toBe('Renamed Folder Paper');
    expect(useAppStore.getState().selectedPaper?.title).toBe('Renamed Folder Paper');
  });

  it('moves non-system folders under another parent and refreshes current papers', async () => {
    const updatedPaper = {
      ...useAppStore.getState().papers[0],
      title: 'Moved Folder Paper',
    };
    backendMocks.moveFolderNode.mockResolvedValueOnce({
      id: 'folder-2',
      name: 'Drafts',
      path: 'Archive/Drafts',
      parentId: 'folder-3',
      isSystem: false,
      createdAt: new Date().toISOString(),
    });
    backendMocks.getFolders.mockResolvedValueOnce([
      {
        id: 'folder-1',
        name: 'Library',
        path: 'Library',
        parentId: '',
        isSystem: true,
        createdAt: new Date().toISOString(),
      },
      {
        id: 'folder-3',
        name: 'Archive',
        path: 'Archive',
        parentId: '',
        isSystem: false,
        createdAt: new Date().toISOString(),
      },
      {
        id: 'folder-2',
        name: 'Drafts',
        path: 'Archive/Drafts',
        parentId: 'folder-3',
        isSystem: false,
        createdAt: new Date().toISOString(),
      },
    ]);
    backendMocks.getPapers.mockResolvedValueOnce([updatedPaper]);
    useAppStore.setState({
      activeFolderId: 'folder-2',
      selectedPaper: useAppStore.getState().papers[0],
      folders: [
        {
          id: 'folder-1',
          name: 'Library',
          path: 'Library',
          parentId: '',
          isSystem: true,
          createdAt: new Date().toISOString(),
        },
        {
          id: 'folder-2',
          name: 'Drafts',
          path: 'Drafts',
          parentId: '',
          isSystem: false,
          createdAt: new Date().toISOString(),
        },
        {
          id: 'folder-3',
          name: 'Archive',
          path: 'Archive',
          parentId: '',
          isSystem: false,
          createdAt: new Date().toISOString(),
        },
      ],
    });

    render(
      <MemoryRouter>
        <PaperListPanel />
      </MemoryRouter>
    );

    fireEvent.click(screen.getByTitle('移动文件夹 Drafts'));
    fireEvent.change(screen.getByLabelText('移动文件夹目标父级'), { target: { value: 'folder-3' } });
    fireEvent.click(screen.getByRole('button', { name: '移动' }));

    await waitFor(() => {
      expect(backendMocks.moveFolderNode).toHaveBeenCalledWith({ folderId: 'folder-2', parentId: 'folder-3' });
    });
    expect(backendMocks.getFolders).toHaveBeenCalledTimes(1);
    expect(backendMocks.getPapers).toHaveBeenCalledWith('folder-2');
    await waitFor(() => {
      expect(useAppStore.getState().folders.find((folder) => folder.id === 'folder-2')?.path).toBe('Archive/Drafts');
    });
    expect(useAppStore.getState().papers[0].title).toBe('Moved Folder Paper');
    expect(useAppStore.getState().selectedPaper?.title).toBe('Moved Folder Paper');
  });

  it('moves papers to another folder and refreshes the active folder', async () => {
    const currentPaper = useAppStore.getState().papers[0];
    backendMocks.movePaperToFolder.mockResolvedValueOnce({
      ...currentPaper,
      folderId: 'folder-2',
    });
    backendMocks.getPapers.mockResolvedValueOnce([]);
    useAppStore.setState({
      selectedPaper: currentPaper,
      folders: [
        {
          id: 'folder-1',
          name: 'Library',
          path: 'Library',
          parentId: '',
          isSystem: true,
          createdAt: new Date().toISOString(),
        },
        {
          id: 'folder-2',
          name: 'Archive',
          path: 'Archive',
          parentId: '',
          isSystem: false,
          createdAt: new Date().toISOString(),
        },
      ],
    });

    render(
      <MemoryRouter>
        <PaperListPanel />
      </MemoryRouter>
    );

    fireEvent.click(screen.getByTitle('移动论文'));
    expect(screen.getByLabelText('移动目标文件夹')).toHaveValue('folder-2');
    fireEvent.click(screen.getByRole('button', { name: '移动' }));

    await waitFor(() => {
      expect(backendMocks.movePaperToFolder).toHaveBeenCalledWith('paper-1', 'folder-2');
    });
    expect(backendMocks.getPapers).toHaveBeenCalledWith('folder-1');
    await waitFor(() => {
      expect(useAppStore.getState().papers).toEqual([]);
    });
    expect(useAppStore.getState().selectedPaper).toBeNull();
  });

  it('batch moves selected papers to another folder and clears selection', async () => {
    const firstPaper = useAppStore.getState().papers[0];
    const secondPaper = {
      ...firstPaper,
      id: 'paper-2',
      sourcePaperId: 'paper-2',
      title: 'Batch Move Candidate',
      url: 'https://example.org/paper-2',
    };
    backendMocks.movePapersToFolder.mockResolvedValueOnce([
      { ...firstPaper, folderId: 'folder-2' },
      { ...secondPaper, folderId: 'folder-2' },
    ]);
    backendMocks.getPapers.mockResolvedValueOnce([]);
    useAppStore.setState({
      selectedPaper: secondPaper,
      papers: [firstPaper, secondPaper],
      folders: [
        {
          id: 'folder-1',
          name: 'Library',
          path: 'Library',
          parentId: '',
          isSystem: true,
          createdAt: new Date().toISOString(),
        },
        {
          id: 'folder-2',
          name: 'Archive',
          path: 'Archive',
          parentId: '',
          isSystem: false,
          createdAt: new Date().toISOString(),
        },
      ],
    });

    render(
      <MemoryRouter>
        <PaperListPanel />
      </MemoryRouter>
    );

    fireEvent.click(screen.getByLabelText('选择论文 Secure Paper Deletion'));
    fireEvent.click(screen.getByLabelText('选择论文 Batch Move Candidate'));
    expect(screen.getByText('已选 2 篇')).toBeInTheDocument();
    expect(screen.getByLabelText('批量移动目标文件夹')).toHaveValue('folder-2');
    fireEvent.click(screen.getByRole('button', { name: '批量移动' }));

    await waitFor(() => {
      expect(backendMocks.movePapersToFolder).toHaveBeenCalledWith(['paper-1', 'paper-2'], 'folder-2');
    });
    expect(backendMocks.getPapers).toHaveBeenCalledWith('folder-1');
    await waitFor(() => {
      expect(useAppStore.getState().papers).toEqual([]);
    });
    expect(useAppStore.getState().selectedPaper).toBeNull();
    expect(screen.queryByText('已选 2 篇')).not.toBeInTheDocument();
  });
});
