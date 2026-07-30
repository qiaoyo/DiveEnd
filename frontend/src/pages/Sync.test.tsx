import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { SyncProgress } from '../types';

const backendMocks = vi.hoisted(() => ({
  getSyncStatus: vi.fn(),
  getSyncProgress: vi.fn(),
  getSyncConflicts: vi.fn(),
  getSyncRecords: vi.fn(),
  getSyncSettings: vi.fn(),
  getPendingDatabaseRestore: vi.fn(),
  getSyncPreview: vi.fn(),
  onSyncProgress: vi.fn(),
  triggerSync: vi.fn(),
  refreshBaiduToken: vi.fn(),
  saveSyncSettings: vi.fn(),
  applyPendingDatabaseRestore: vi.fn(),
  cancelPendingDatabaseRestore: vi.fn(),
  resolveSyncConflict: vi.fn(),
}));

vi.mock('../lib/backend', () => backendMocks);

import { Sync } from './Sync';

describe('Sync page', () => {
  let syncProgressListener: ((progress: SyncProgress) => void) | undefined;
  let unsubscribe: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    vi.clearAllMocks();
    syncProgressListener = undefined;
    unsubscribe = vi.fn();

    backendMocks.getSyncStatus.mockResolvedValue({
      enabled: true,
      provider: 'baidu_cloud',
      lastSync: null,
      syncInProgress: false,
      pendingFiles: 0,
      conflicts: 0,
      totalSynced: 0,
      totalFailed: 0,
    });
    backendMocks.getSyncProgress.mockResolvedValue({
      total: 0,
      completed: 0,
      currentFile: '',
      status: 'idle',
      message: '',
    });
    backendMocks.getSyncConflicts.mockResolvedValue([]);
    backendMocks.getSyncRecords.mockResolvedValue([]);
    backendMocks.getSyncSettings.mockResolvedValue({
      autoSync: false,
      syncOnStartup: false,
      syncBeforeExit: false,
      syncInterval: 30,
      conflictResolution: 'timestamp',
    });
    backendMocks.getPendingDatabaseRestore.mockResolvedValue({
      pending: false,
      applied: false,
      stagedPath: '',
      backupPath: '',
      remotePath: '',
      message: '',
    });
    backendMocks.onSyncProgress.mockImplementation((callback: (progress: SyncProgress) => void) => {
      syncProgressListener = callback;
      return unsubscribe;
    });
    backendMocks.getSyncPreview.mockResolvedValue({
      enabled: true,
      dataPath: '/Users/bytedance/DiveEndData',
      remoteRoot: '/apps/pcstest_oauth/diveend-v1',
      tokenFile: 'baiduyun_token.json',
      totalFiles: 3,
      totalBytes: 2048,
      databaseBytes: 1024,
      paperPdfCount: 1,
      paperPdfBytes: 768,
      otherFiles: 1,
      files: [
        { key: 'data/diveend.db', fileName: 'diveend.db', kind: 'database', size: 1024, remotePath: '/apps/pcstest_oauth/diveend-v1/data/diveend.db' },
        { key: 'papers/paper-1/paper.pdf', fileName: 'paper.pdf', kind: 'paper_pdf', size: 768, remotePath: '/apps/pcstest_oauth/diveend-v1/papers/paper-1/paper.pdf' },
      ],
      checkedAt: new Date().toISOString(),
    });
    backendMocks.refreshBaiduToken.mockResolvedValue({
      enabled: true,
      tokenFile: 'baiduyun_token.json',
      hasAccessToken: true,
      hasRefreshToken: true,
      hasClientId: true,
      hasClientSecret: true,
      refreshed: true,
      checkedAt: new Date().toISOString(),
      message: '百度 access token 已通过 refresh token 更新',
    });
  });

  afterEach(() => {
    cleanup();
  });

  it('renders backend state and updates progress from sync-progress events', async () => {
    render(<Sync />);

    expect(await screen.findByText('已连接')).toBeInTheDocument();
    expect(backendMocks.onSyncProgress).toHaveBeenCalledTimes(1);

    act(() => {
      syncProgressListener?.({
        total: 4,
        completed: 2,
        currentFile: 'data/diveend.db',
        status: 'uploading',
        message: '上传数据库快照',
      });
    });

    expect(screen.getByText('上传数据库快照')).toBeInTheDocument();
    expect(screen.getByText('2/4')).toBeInTheDocument();

    act(() => {
      syncProgressListener?.({
        total: 4,
        completed: 4,
        currentFile: '',
        status: 'complete',
        message: '同步完成',
      });
    });

    expect(screen.getByText('同步完成')).toBeInTheDocument();
    await waitFor(() => {
      expect(backendMocks.getSyncStatus).toHaveBeenCalledTimes(2);
    });
  });

  it('refreshes Baidu access token from the Sync toolbar', async () => {
    render(<Sync />);

    expect(await screen.findByText('已连接')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '更新凭证' }));

    await waitFor(() => {
      expect(backendMocks.refreshBaiduToken).toHaveBeenCalledTimes(1);
    });
    expect(await screen.findByText('百度 access token 已通过 refresh token 更新')).toBeInTheDocument();
    expect(backendMocks.getSyncStatus).toHaveBeenCalledTimes(2);
  });

  it('previews files before triggering manual sync', async () => {
    render(<Sync />);

    expect(await screen.findByText('已连接')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '检查并同步' }));

    expect(await screen.findByText('确认本次百度云同步')).toBeInTheDocument();
    expect(screen.getByText('3')).toBeInTheDocument();
    expect(screen.getByText('data/diveend.db')).toBeInTheDocument();
    expect(backendMocks.triggerSync).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole('button', { name: '确认开始同步' }));
    await waitFor(() => {
      expect(backendMocks.triggerSync).toHaveBeenCalledTimes(1);
    });
  });

  it('blocks manual sync while another sync is active', async () => {
    backendMocks.getSyncStatus.mockResolvedValueOnce({
      enabled: true,
      provider: 'baidu_cloud',
      lastSync: null,
      syncInProgress: true,
      pendingFiles: 0,
      conflicts: 0,
      totalSynced: 0,
      totalFailed: 0,
    });
    backendMocks.getSyncProgress.mockResolvedValueOnce({
      total: 2,
      completed: 1,
      currentFile: 'data/diveend.db',
      status: 'uploading',
      message: '同步中',
    });

    render(<Sync />);

    expect(await screen.findByText('已连接')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '同步中...' })).toBeDisabled();
    expect(screen.getByRole('button', { name: '更新凭证' })).toBeDisabled();
    expect(backendMocks.getSyncPreview).not.toHaveBeenCalled();
  });

  it('renders sync history messages from backend records', async () => {
    backendMocks.getSyncRecords.mockResolvedValueOnce([
      {
        id: 'record-1',
        type: 'download',
        fileName: 'diveend.db',
        fileSize: 2048,
        remotePath: '/apps/pcstest_oauth/diveend-v1/data/diveend.db',
        localPath: '/tmp/diveend-remote.db',
        status: 'success',
        message: 'Conflict resolved by keeping cloud version',
        createdAt: '2026-06-13T06:00:00Z',
        completedAt: '2026-06-13T06:00:01Z',
      },
    ]);

    render(<Sync />);

    expect(await screen.findByText('已连接')).toBeInTheDocument();
    expect(screen.getByText(/Conflict resolved by keeping cloud version · download ·/)).toBeInTheDocument();
  });

  it('renders conflict diff metadata and preservation hint', async () => {
    backendMocks.getSyncStatus.mockResolvedValueOnce({
      enabled: true,
      provider: 'baidu_cloud',
      lastSync: null,
      syncInProgress: false,
      pendingFiles: 0,
      conflicts: 1,
      totalSynced: 0,
      totalFailed: 0,
    });
    backendMocks.getSyncConflicts.mockResolvedValueOnce([
      {
        id: 'conflict-1',
        fileName: 'diveend.db',
        fileKind: 'database',
        localPath: '/Users/test/.diveend/diveend.db',
        localSize: 2048,
        localTime: '2026-06-13T06:00:00Z',
        remotePath: '/apps/pcstest_oauth/diveend-v1/data/diveend.db',
        remoteSize: 4096,
        remoteTime: '2026-06-13T06:05:00Z',
        newerSide: 'remote',
        resolution: '',
        createdAt: '2026-06-13T06:05:01Z',
      },
    ]);

    render(<Sync />);

    expect(await screen.findByText('已连接')).toBeInTheDocument();
    fireEvent.click(screen.getByText('冲突处理'));

    expect(screen.getByText('SQLite 数据库')).toBeInTheDocument();
    expect(screen.getByText('云端较新')).toBeInTheDocument();
    expect(screen.getByText(/大小差异 2 KiB/)).toBeInTheDocument();
    expect(screen.getByText(/\.sync-conflicts/)).toBeInTheDocument();
    expect(screen.getByText('文件大小：2 KiB')).toBeInTheDocument();
    expect(screen.getByText('文件大小：4 KiB')).toBeInTheDocument();
  });

  it('treats legacy completed progress events as terminal', async () => {
    render(<Sync />);

    expect(await screen.findByText('已连接')).toBeInTheDocument();

    act(() => {
      syncProgressListener?.({
        total: 2,
        completed: 2,
        currentFile: '',
        status: 'completed',
        message: '同步完成',
      });
    });

    expect(screen.getByText('同步完成')).toBeInTheDocument();
    await waitFor(() => {
      expect(backendMocks.getSyncStatus).toHaveBeenCalledTimes(2);
    });
  });

  it('rolls back optimistic sync setting changes when saving fails', async () => {
    backendMocks.saveSyncSettings.mockRejectedValueOnce(new Error('sync settings are locked during active sync'));

    render(<Sync />);

    expect(await screen.findByText('已连接')).toBeInTheDocument();
    const startupToggle = screen.getByLabelText('启动时同步') as HTMLInputElement;
    expect(startupToggle.checked).toBe(false);

    fireEvent.click(startupToggle);

    await waitFor(() => {
      expect(backendMocks.saveSyncSettings).toHaveBeenCalledWith({
        autoSync: false,
        syncOnStartup: true,
        syncBeforeExit: false,
        syncInterval: 30,
        conflictResolution: 'timestamp',
      });
    });
    await waitFor(() => {
      expect(startupToggle.checked).toBe(false);
    });
    expect(screen.getByText('sync settings are locked during active sync')).toBeInTheDocument();
  });

  it('persists auto-sync toggle and interval settings', async () => {
    backendMocks.saveSyncSettings.mockImplementation(async (settings) => settings);

    render(<Sync />);

    expect(await screen.findByText('已连接')).toBeInTheDocument();

    const autoSyncToggle = screen.getByLabelText('定时同步变更') as HTMLInputElement;
    expect(autoSyncToggle.checked).toBe(false);

    fireEvent.click(autoSyncToggle);

    await waitFor(() => {
      expect(backendMocks.saveSyncSettings).toHaveBeenCalledWith({
        autoSync: true,
        syncOnStartup: false,
        syncBeforeExit: false,
        syncInterval: 30,
        conflictResolution: 'timestamp',
      });
    });
    await waitFor(() => {
      expect(autoSyncToggle.checked).toBe(true);
    });

    const intervalSelect = screen.getByLabelText('同步间隔（分钟）') as HTMLSelectElement;
    fireEvent.change(intervalSelect, { target: { value: '60' } });

    await waitFor(() => {
      expect(backendMocks.saveSyncSettings).toHaveBeenCalledWith({
        autoSync: true,
        syncOnStartup: false,
        syncBeforeExit: false,
        syncInterval: 60,
        conflictResolution: 'timestamp',
      });
    });

    const strategySelect = screen.getByLabelText('冲突策略') as HTMLSelectElement;
    fireEvent.change(strategySelect, { target: { value: 'manual' } });

    await waitFor(() => {
      expect(backendMocks.saveSyncSettings).toHaveBeenCalledWith({
        autoSync: true,
        syncOnStartup: false,
        syncBeforeExit: false,
        syncInterval: 60,
        conflictResolution: 'manual',
      });
    });
  });

  it('persists exit-time sync independently from periodic auto-sync', async () => {
    backendMocks.saveSyncSettings.mockImplementation(async (settings) => settings);

    render(<Sync />);

    expect(await screen.findByText('已连接')).toBeInTheDocument();

    const exitToggle = screen.getByLabelText('退出前同步') as HTMLInputElement;
    expect(exitToggle.checked).toBe(false);

    fireEvent.click(exitToggle);

    await waitFor(() => {
      expect(backendMocks.saveSyncSettings).toHaveBeenCalledWith({
        autoSync: false,
        syncOnStartup: false,
        syncBeforeExit: true,
        syncInterval: 30,
        conflictResolution: 'timestamp',
      });
    });
    await waitFor(() => {
      expect(exitToggle.checked).toBe(true);
    });

    const autoSyncToggle = screen.getByLabelText('定时同步变更') as HTMLInputElement;
    expect(autoSyncToggle.checked).toBe(false);
  });

  it('unsubscribes from sync progress events on unmount', async () => {
    const { unmount } = render(<Sync />);

    expect(await screen.findByText('已连接')).toBeInTheDocument();
    unmount();

    expect(unsubscribe).toHaveBeenCalledTimes(1);
  });
});
