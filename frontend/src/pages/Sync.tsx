import React, { useEffect, useMemo, useState } from 'react';
import { Cloud, RefreshCw, ShieldAlert } from 'lucide-react';
import type { DatabaseRestoreStatus, SyncConflict, SyncPreview, SyncProgress, SyncRecord, SyncSettings, SyncStatus } from '../types';
import * as backend from '../lib/backend';
import { errorToUserMessage } from '../lib/errors';

const emptyStatus: SyncStatus = {
  enabled: false,
  provider: 'baidu_cloud',
  lastSync: null,
  syncInProgress: false,
  pendingFiles: 0,
  conflicts: 0,
  totalSynced: 0,
  totalFailed: 0,
};

const emptyProgress: SyncProgress = {
  total: 0,
  completed: 0,
  currentFile: '',
  status: 'idle',
  message: '',
};

const defaultSyncSettings: SyncSettings = {
  autoSync: false,
  syncOnStartup: false,
  syncBeforeExit: false,
  syncInterval: 30,
  conflictResolution: 'timestamp',
};

const syncIntervalOptions = [5, 15, 30, 60, 180, 720, 1440];

const emptyDatabaseRestore: DatabaseRestoreStatus = {
  pending: false,
  applied: false,
  stagedPath: '',
  backupPath: '',
  remotePath: '',
  message: '',
};

type SyncTab = 'history' | 'conflicts';

const syncProgressDoneStatuses = new Set(['', 'idle', 'complete', 'completed', 'error']);

function isSyncProgressActive(status: string): boolean {
  return !syncProgressDoneStatuses.has(status.trim());
}

function isSyncProgressTerminal(status: string): boolean {
  return status === 'complete' || status === 'completed' || status === 'error';
}

function syncRecordMessage(record: SyncRecord): string {
  if (record.message) {
    return record.message;
  }
  switch (record.type) {
    case 'download':
      return 'Downloaded from cloud';
    case 'conflict':
      return 'Conflict event';
    case 'upload':
    default:
      return 'Uploaded to cloud';
  }
}

function formatBytes(value?: number | null): string {
  const bytes = Number(value ?? 0);
  if (!Number.isFinite(bytes) || bytes <= 0) {
    return '未知大小';
  }
  const units = ['B', 'KiB', 'MiB', 'GiB'];
  let current = bytes;
  let unitIndex = 0;
  while (current >= 1024 && unitIndex < units.length - 1) {
    current /= 1024;
    unitIndex += 1;
  }
  const decimals = unitIndex === 0 || current >= 10 || Number.isInteger(current) ? 0 : 1;
  return `${current.toFixed(decimals)} ${units[unitIndex]}`;
}

function conflictKindLabel(kind?: string): string {
  switch (kind) {
    case 'database':
      return 'SQLite 数据库';
    case 'paper_pdf':
      return '论文 PDF';
    default:
      return '同步文件';
  }
}

function conflictNewerLabel(side?: string): string {
  switch (side) {
    case 'local':
      return '本地较新';
    case 'remote':
      return '云端较新';
    case 'equal':
      return '时间相同';
    default:
      return '等待判断';
  }
}

export const Sync: React.FC = () => {
  const [syncStatus, setSyncStatus] = useState<SyncStatus>(emptyStatus);
  const [syncProgress, setSyncProgress] = useState<SyncProgress>(emptyProgress);
  const [syncHistory, setSyncHistory] = useState<SyncRecord[]>([]);
  const [syncConflicts, setSyncConflicts] = useState<SyncConflict[]>([]);
  const [databaseRestore, setDatabaseRestore] = useState<DatabaseRestoreStatus>(emptyDatabaseRestore);
  const [isLoading, setIsLoading] = useState(true);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [isApplyingRestore, setIsApplyingRestore] = useState(false);
  const [isSavingSyncSettings, setIsSavingSyncSettings] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<SyncTab>('history');
  const [syncSettings, setSyncSettings] = useState<SyncSettings>(defaultSyncSettings);
  const [isRefreshingToken, setIsRefreshingToken] = useState(false);
  const [tokenRefreshMessage, setTokenRefreshMessage] = useState('');
  const [isLoadingPreview, setIsLoadingPreview] = useState(false);
  const [syncPreview, setSyncPreview] = useState<SyncPreview | null>(null);
  const [isPreviewOpen, setIsPreviewOpen] = useState(false);

  const loadSyncState = async () => {
    const [status, progress, conflicts, records, settings, restore] = await Promise.allSettled([
      backend.getSyncStatus(),
      backend.getSyncProgress(),
      backend.getSyncConflicts(),
      backend.getSyncRecords(30),
      backend.getSyncSettings(),
      backend.getPendingDatabaseRestore(),
    ] as const);

    if (status.status === 'fulfilled') setSyncStatus(status.value);
    if (progress.status === 'fulfilled') setSyncProgress(progress.value);
    if (conflicts.status === 'fulfilled') setSyncConflicts(conflicts.value);
    if (records.status === 'fulfilled') setSyncHistory(records.value);
    if (settings.status === 'fulfilled') setSyncSettings(settings.value);
    if (restore.status === 'fulfilled') setDatabaseRestore(restore.value);

    // Keep the dashboard usable even when one cloud-side probe fails.
  };

  useEffect(() => {
    let cancelled = false;

    const load = async () => {
      setIsLoading(true);
      setError(null);
      try {
        await loadSyncState();
      } catch (cause) {
        if (!cancelled) {
          setError(errorToUserMessage(cause, '加载同步状态失败'));
        }
      } finally {
        if (!cancelled) {
          setIsLoading(false);
        }
      }
    };

    void load();

    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => backend.onSyncProgress((progress) => {
    setSyncProgress(progress);
    setSyncStatus((current) => ({
      ...current,
      syncInProgress: isSyncProgressActive(progress.status),
    }));
    if (isSyncProgressTerminal(progress.status)) {
      void loadSyncState().catch(() => {});
    }
  }), []);

  const formatTimestamp = (timestamp?: string | null) => {
    if (!timestamp) {
      return '从未同步';
    }

    const date = new Date(timestamp);
    if (Number.isNaN(date.getTime())) {
      return timestamp;
    }

    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffMins = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMs / 3600000);
    const diffDays = Math.floor(diffMs / 86400000);

    if (diffMins < 1) return '刚刚';
    if (diffMins < 60) return `${diffMins} 分钟前`;
    if (diffHours < 24) return `${diffHours} 小时前`;
    if (diffDays < 7) return `${diffDays} 天前`;
    return date.toLocaleString();
  };

  const handleRefresh = async () => {
    setIsRefreshing(true);
    setError(null);
    try {
      await loadSyncState();
    } catch (cause) {
      setError(errorToUserMessage(cause, '刷新同步状态失败'));
    } finally {
      setIsRefreshing(false);
    }
  };

  const syncIsActive = syncStatus.syncInProgress || isSyncProgressActive(syncProgress.status);

  const handleSyncNow = async () => {
    if (syncIsActive) {
      setError('已有同步任务正在运行，请等待当前任务完成后再开始新的同步。');
      return;
    }

    setIsLoadingPreview(true);
    setError(null);
    setSyncPreview(null);
    try {
      const preview = await backend.getSyncPreview();
      setSyncPreview(preview);
      setIsPreviewOpen(true);
    } catch (cause) {
      setError(errorToUserMessage(cause, '准备同步预检失败'));
    } finally {
      setIsLoadingPreview(false);
    }
  };

  const handleConfirmSync = async () => {
    if (syncIsActive) {
      setError('已有同步任务正在运行，请等待当前任务完成后再开始新的同步。');
      return;
    }

    setIsRefreshing(true);
    setError(null);
    setIsPreviewOpen(false);
    try {
      await backend.triggerSync();
      await loadSyncState();
    } catch (cause) {
      setError(errorToUserMessage(cause, '触发同步失败'));
    } finally {
      setIsRefreshing(false);
    }
  };

  const handleRefreshBaiduToken = async () => {
    setIsRefreshingToken(true);
    setError(null);
    setTokenRefreshMessage('');
    try {
      const status = await backend.refreshBaiduToken();
      setTokenRefreshMessage(status?.message || '百度 access token 已刷新。');
      await loadSyncState();
    } catch (cause) {
      setError(errorToUserMessage(cause, '刷新百度 access token 失败'));
    } finally {
      setIsRefreshingToken(false);
    }
  };

  useEffect(() => {
    if (!syncStatus.syncInProgress && !isSyncProgressActive(syncProgress.status)) {
      return;
    }
    const timer = window.setInterval(() => {
      void loadSyncState().catch(() => {});
    }, 1200);
    return () => window.clearInterval(timer);
  }, [syncProgress.status, syncStatus.syncInProgress]);

  const handleSaveSyncSettings = async (patch: Partial<SyncSettings>) => {
    const previousSettings = syncSettings;
    const nextSettings = { ...syncSettings, ...patch };
    setSyncSettings(nextSettings);
    setIsSavingSyncSettings(true);
    setError(null);
    try {
      const saved = await backend.saveSyncSettings(nextSettings);
      setSyncSettings(saved);
    } catch (cause) {
      setSyncSettings(previousSettings);
      setError(errorToUserMessage(cause, '保存同步设置失败'));
    } finally {
      setIsSavingSyncSettings(false);
    }
  };

  const handleApplyDatabaseRestore = async () => {
    setIsApplyingRestore(true);
    setError(null);
    try {
      const status = await backend.applyPendingDatabaseRestore();
      setDatabaseRestore(status);
      await loadSyncState();
      window.setTimeout(() => window.location.reload(), 250);
    } catch (cause) {
      setError(errorToUserMessage(cause, '应用云端数据库失败'));
    } finally {
      setIsApplyingRestore(false);
    }
  };

  const handleCancelDatabaseRestore = async () => {
    setError(null);
    try {
      await backend.cancelPendingDatabaseRestore();
      setDatabaseRestore(emptyDatabaseRestore);
      await loadSyncState();
    } catch (cause) {
      setError(errorToUserMessage(cause, '取消数据库恢复失败'));
    }
  };

  const handleResolveConflict = async (conflictId: string, resolution: 'local' | 'remote') => {
    setIsRefreshing(true);
    setError(null);
    try {
      await backend.resolveSyncConflict(conflictId, resolution);
      await loadSyncState();
    } catch (cause) {
      setError(errorToUserMessage(cause, '解决冲突失败'));
    } finally {
      setIsRefreshing(false);
    }
  };

  const progressPercent = syncProgress.total > 0
    ? Math.min(100, Math.max(0, Math.round((syncProgress.completed / syncProgress.total) * 100)))
    : 0;

  const dashboardCards = useMemo(() => [
    { label: '总同步成功', value: syncStatus.totalSynced.toString() },
    { label: '待处理文件', value: syncStatus.pendingFiles.toString() },
    { label: '冲突数量', value: syncStatus.conflicts.toString(), accent: syncStatus.conflicts > 0 },
    { label: '同步失败', value: syncStatus.totalFailed.toString(), accent: syncStatus.totalFailed > 0 },
  ], [syncStatus]);

  if (isLoading) {
    return (
      <div className="flex h-full min-h-0 items-center justify-center">
        <div className="de-glass rounded-2xl px-8 py-8 text-center">
          <RefreshCw className="mx-auto mb-3 h-8 w-8 animate-spin text-indigo-600" />
          <p className="text-sm text-slate-600 dark:text-slate-200">正在加载同步状态...</p>
        </div>
      </div>
    );
  }

  return (
    <div className="flex h-full min-h-0 flex-col overflow-hidden bg-slate-50 text-slate-900 dark:bg-slate-950 dark:text-slate-100">
      <div className="border-b border-slate-200/80 bg-white/70 px-6 py-4 backdrop-blur-md dark:border-slate-700/50 dark:bg-slate-900/55">
        <div className="mx-auto flex max-w-6xl flex-wrap items-center justify-between gap-3">
          <div>
            <h2 className="flex items-center gap-2 text-lg font-semibold">
              <Cloud className="h-5 w-5 text-indigo-600" />
              Sync & Polish
            </h2>
            <p className="mt-1 text-sm text-slate-500 dark:text-slate-300">
              真实读取后端同步状态、冲突列表和历史记录。
            </p>
          </div>

          <div className="flex gap-2">
            <button
              onClick={() => void handleRefresh()}
              disabled={isRefreshing}
              className="inline-flex items-center gap-1.5 rounded-xl border border-slate-200 bg-white/80 px-3 py-2 text-sm text-slate-600 transition hover:border-indigo-300 hover:text-indigo-700 disabled:cursor-not-allowed disabled:opacity-60 dark:border-slate-700 dark:bg-slate-900/80 dark:text-slate-200 dark:hover:border-indigo-500/60 dark:hover:text-indigo-300"
            >
              <RefreshCw className={`h-4 w-4 ${isRefreshing ? 'animate-spin' : ''}`} />
              刷新
            </button>
            <button
              onClick={() => void handleRefreshBaiduToken()}
              disabled={isRefreshingToken || isRefreshing || isLoadingPreview || syncIsActive || !syncStatus.enabled}
              className="inline-flex items-center gap-1.5 rounded-xl border border-amber-200 bg-amber-50 px-3 py-2 text-sm font-medium text-amber-800 transition hover:bg-amber-100 disabled:cursor-not-allowed disabled:opacity-60 dark:border-amber-500/40 dark:bg-amber-500/15 dark:text-amber-100"
            >
              <RefreshCw className={`h-4 w-4 ${isRefreshingToken ? 'animate-spin' : ''}`} />
              {isRefreshingToken ? '刷新 token 中...' : 'Refresh Token'}
            </button>
            <button
              onClick={() => void handleSyncNow()}
              disabled={isRefreshing || isRefreshingToken || isLoadingPreview || syncIsActive || !syncStatus.enabled}
              className="inline-flex items-center gap-1.5 rounded-xl bg-indigo-600 px-4 py-2 text-sm font-medium text-white transition hover:bg-indigo-500 disabled:cursor-not-allowed disabled:opacity-60"
            >
              <RefreshCw className={`h-4 w-4 ${isRefreshing ? 'animate-spin' : ''}`} />
              {isRefreshing || syncIsActive ? '同步中...' : isLoadingPreview ? '预检中...' : 'Manual Sync'}
            </button>
          </div>
        </div>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto px-6 py-6">
        <div className="mx-auto max-w-6xl space-y-5">
          {error && (
            <div className="rounded-xl border border-rose-200 bg-rose-50/80 px-4 py-3 text-sm text-rose-700 dark:border-rose-900 dark:bg-rose-500/15 dark:text-rose-200">
              {error}
            </div>
          )}

          {isPreviewOpen && syncPreview && (
            <section className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/45 px-4 backdrop-blur-sm">
              <div className="max-h-[85vh] w-full max-w-3xl overflow-y-auto rounded-3xl border border-slate-200 bg-white p-5 shadow-2xl dark:border-slate-700 dark:bg-slate-950">
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div>
                    <p className="text-xs uppercase tracking-[0.18em] text-indigo-600 dark:text-indigo-300">Sync Preflight</p>
                    <h3 className="mt-1 text-lg font-semibold">确认本次百度云同步</h3>
                    <p className="mt-2 text-sm leading-6 text-slate-600 dark:text-slate-300">
                      DiveEnd 将先上传当前数据库快照、manifest 和已管理的论文 PDF。请确认文件数量和总大小后再开始。
                    </p>
                  </div>
                  <button
                    type="button"
                    onClick={() => setIsPreviewOpen(false)}
                    className="rounded-xl border border-slate-200 bg-white/70 px-3 py-1.5 text-sm text-slate-600 transition hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-200 dark:hover:bg-slate-800"
                  >
                    关闭
                  </button>
                </div>

                {syncPreview.warning && (
                  <div className="mt-4 rounded-xl border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800 dark:border-amber-500/40 dark:bg-amber-500/15 dark:text-amber-100">
                    {syncPreview.warning}
                  </div>
                )}

                <div className="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
                  <div className="rounded-xl border border-slate-200 bg-slate-50 p-3 dark:border-slate-700 dark:bg-slate-900">
                    <div className="text-xs text-slate-500 dark:text-slate-400">文件数</div>
                    <div className="mt-1 text-lg font-semibold">{syncPreview.totalFiles}</div>
                  </div>
                  <div className="rounded-xl border border-slate-200 bg-slate-50 p-3 dark:border-slate-700 dark:bg-slate-900">
                    <div className="text-xs text-slate-500 dark:text-slate-400">总大小</div>
                    <div className="mt-1 text-lg font-semibold">{formatBytes(syncPreview.totalBytes)}</div>
                  </div>
                  <div className="rounded-xl border border-slate-200 bg-slate-50 p-3 dark:border-slate-700 dark:bg-slate-900">
                    <div className="text-xs text-slate-500 dark:text-slate-400">论文 PDF</div>
                    <div className="mt-1 text-lg font-semibold">{syncPreview.paperPdfCount} · {formatBytes(syncPreview.paperPdfBytes)}</div>
                  </div>
                  <div className="rounded-xl border border-slate-200 bg-slate-50 p-3 dark:border-slate-700 dark:bg-slate-900">
                    <div className="text-xs text-slate-500 dark:text-slate-400">数据库快照</div>
                    <div className="mt-1 text-lg font-semibold">{formatBytes(syncPreview.databaseBytes)}</div>
                  </div>
                </div>

                <div className="mt-4 space-y-1 rounded-xl border border-slate-200 bg-slate-50 p-3 text-xs text-slate-600 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-300">
                  <div className="break-all">本地数据：{syncPreview.dataPath}</div>
                  <div className="break-all">远端目录：{syncPreview.remoteRoot}</div>
                  <div>Token 文件：{syncPreview.tokenFile}</div>
                </div>

                <div className="mt-4 max-h-56 overflow-y-auto rounded-xl border border-slate-200 dark:border-slate-700">
                  {(syncPreview.files || []).slice(0, 20).map((file) => (
                    <div key={file.key} className="flex items-center justify-between gap-3 border-b border-slate-100 px-3 py-2 text-xs last:border-b-0 dark:border-slate-800">
                      <div className="min-w-0">
                        <div className="truncate font-medium">{file.key}</div>
                        <div className="truncate text-slate-500 dark:text-slate-400">{file.remotePath}</div>
                      </div>
                      <div className="shrink-0 text-right text-slate-500 dark:text-slate-400">
                        <div>{file.kind}</div>
                        <div>{formatBytes(file.size)}</div>
                      </div>
                    </div>
                  ))}
                  {syncPreview.files.length > 20 && (
                    <div className="px-3 py-2 text-xs text-slate-500 dark:text-slate-400">
                      仅展示前 20 个文件，剩余 {syncPreview.files.length - 20} 个文件会一并同步。
                    </div>
                  )}
                </div>

                <div className="mt-5 flex flex-wrap justify-end gap-2">
                  <button
                    type="button"
                    onClick={() => setIsPreviewOpen(false)}
                    className="rounded-xl border border-slate-200 bg-white px-4 py-2 text-sm text-slate-600 transition hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-200 dark:hover:bg-slate-800"
                  >
                    暂不上传
                  </button>
                  <button
                    type="button"
                    onClick={() => void handleConfirmSync()}
                    disabled={isRefreshing || syncIsActive}
                    className="rounded-xl bg-indigo-600 px-4 py-2 text-sm font-semibold text-white transition hover:bg-indigo-500 disabled:cursor-not-allowed disabled:opacity-60"
                  >
                    确认开始同步
                  </button>
                </div>
              </div>
            </section>
          )}

          {databaseRestore.pending && (
            <section className="rounded-2xl border border-amber-300 bg-amber-50/90 p-4 text-sm text-amber-900 dark:border-amber-500/50 dark:bg-amber-500/15 dark:text-amber-100">
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2 font-semibold">
                    <ShieldAlert className="h-4 w-4" />
                    云端数据库恢复待确认
                  </div>
                  <p className="mt-2 leading-6">
                    {databaseRestore.message || '云端数据库已下载到安全暂存区。应用后会先备份当前本地数据库，再替换为云端数据库。'}
                  </p>
                  <div className="mt-2 space-y-1 break-all text-xs opacity-85">
                    {databaseRestore.remotePath ? <div>云端来源：{databaseRestore.remotePath}</div> : null}
                    {databaseRestore.backupPath ? <div>本地备份：{databaseRestore.backupPath}</div> : null}
                  </div>
                </div>
                <div className="flex shrink-0 flex-wrap gap-2">
                  <button
                    type="button"
                    onClick={() => void handleApplyDatabaseRestore()}
                    disabled={isApplyingRestore}
                    className="rounded-xl bg-amber-700 px-3 py-2 text-xs font-semibold text-white transition hover:bg-amber-600 disabled:opacity-60"
                  >
                    {isApplyingRestore ? '应用中...' : '立即应用并刷新'}
                  </button>
                  <button
                    type="button"
                    onClick={() => void handleCancelDatabaseRestore()}
                    disabled={isApplyingRestore}
                    className="rounded-xl border border-amber-300 bg-white/70 px-3 py-2 text-xs font-medium text-amber-800 transition hover:bg-white disabled:opacity-60 dark:border-amber-400/40 dark:bg-slate-900/40 dark:text-amber-100"
                  >
                    暂不恢复
                  </button>
                </div>
              </div>
            </section>
          )}

          <section className="grid gap-4 lg:grid-cols-[1fr_1fr]">
            <div className="de-glass rounded-2xl p-4">
              <h3 className="text-xs uppercase tracking-[0.18em] text-slate-500 dark:text-slate-300">Local Data</h3>
              <div className="mt-3 grid gap-2 sm:grid-cols-2">
                {dashboardCards.map((card) => (
                  <div key={card.label} className="rounded-xl border border-slate-200 bg-white/80 px-3 py-3 dark:border-slate-700 dark:bg-slate-900/80">
                    <div className="text-xs text-slate-500 dark:text-slate-300">{card.label}</div>
                    <div className={`mt-1 text-lg font-semibold ${card.accent ? 'text-rose-600 dark:text-rose-300' : ''}`}>
                      {card.value}
                    </div>
                  </div>
                ))}
              </div>
            </div>

            <div className="de-glass rounded-2xl p-4">
              <h3 className="text-xs uppercase tracking-[0.18em] text-slate-500 dark:text-slate-300">Baidu Cloud Sync</h3>
              <div className="mt-3 flex items-start justify-between gap-3">
                <div>
                  <div className="flex items-center gap-2">
                    <span className={`h-2.5 w-2.5 rounded-full ${syncStatus.enabled ? 'bg-emerald-500' : 'bg-slate-400'}`} />
                    <span className="text-sm font-medium">{syncStatus.enabled ? 'Connected' : 'Disconnected'}</span>
                  </div>
                  <p className="mt-2 text-xs text-slate-500 dark:text-slate-300">
                    Provider: {syncStatus.provider.replace('_', ' ')} · 最后同步 {formatTimestamp(syncStatus.lastSync)}
                  </p>
                  <p className="mt-1 text-xs text-slate-500 dark:text-slate-300">
                    Token file: baiduyun_token.json · 使用 Refresh Token 可在同步前手动更新 access token。
                  </p>
                  {tokenRefreshMessage && (
                    <p className="mt-2 rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-xs text-emerald-700 dark:border-emerald-500/40 dark:bg-emerald-500/15 dark:text-emerald-100">
                      {tokenRefreshMessage}
                    </p>
                  )}
                </div>
              </div>

              {(syncStatus.syncInProgress || syncProgress.currentFile || syncProgress.message) && (
                <div className="mt-4 rounded-xl border border-slate-200 bg-white/80 p-3 dark:border-slate-700 dark:bg-slate-900/80">
                  <div className="flex items-center justify-between text-xs text-slate-500 dark:text-slate-300">
                    <span>{syncProgress.message || syncProgress.currentFile || '准备中'}</span>
                    <span>{syncProgress.completed}/{syncProgress.total}</span>
                  </div>
                  <div className="mt-2 h-2 rounded-full bg-slate-200 dark:bg-slate-800">
                    <div className="h-2 rounded-full bg-indigo-600 transition-all" style={{ width: `${progressPercent}%` }} />
                  </div>
                </div>
              )}
            </div>
          </section>

          <section className="de-glass rounded-2xl p-4">
            <div className="flex border-b border-slate-200 dark:border-slate-700">
              <button
                onClick={() => setActiveTab('history')}
                className={`px-4 py-2 text-sm font-medium ${
                  activeTab === 'history'
                    ? 'border-b-2 border-indigo-500 text-indigo-600 dark:text-indigo-300'
                    : 'text-slate-500 dark:text-slate-300'
                }`}
              >
                Sync History
              </button>
              <button
                onClick={() => setActiveTab('conflicts')}
                className={`px-4 py-2 text-sm font-medium ${
                  activeTab === 'conflicts'
                    ? 'border-b-2 border-indigo-500 text-indigo-600 dark:text-indigo-300'
                    : 'text-slate-500 dark:text-slate-300'
                }`}
              >
                Conflict Resolution
              </button>
            </div>

            {activeTab === 'history' ? (
              <div className="mt-4 space-y-3">
                {syncHistory.length === 0 ? (
                  <div className="rounded-xl border border-dashed border-slate-300 bg-white/70 p-6 text-sm text-slate-500 dark:border-slate-600 dark:bg-slate-900/70 dark:text-slate-300">
                    暂无同步活动记录。
                  </div>
                ) : (
                  syncHistory.map((item) => (
                    <div key={item.id} className="rounded-xl border border-slate-200 bg-white/80 p-4 dark:border-slate-700 dark:bg-slate-900/80">
                      <div className="flex flex-wrap items-center justify-between gap-2">
                        <p className="text-sm font-medium">{item.fileName}</p>
                        <span className={`rounded-full px-2.5 py-1 text-xs ${
                          item.status === 'success'
                            ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/20 dark:text-emerald-200'
                            : item.status === 'failed'
                              ? 'bg-rose-100 text-rose-700 dark:bg-rose-500/20 dark:text-rose-200'
                              : 'bg-amber-100 text-amber-700 dark:bg-amber-500/20 dark:text-amber-200'
                        }`}
                        >
                          {item.status}
                        </span>
                      </div>
                      <p className="mt-1 text-xs text-slate-500 dark:text-slate-300">
                        {syncRecordMessage(item)} · {item.type} · {formatTimestamp(item.completedAt || item.createdAt)}
                      </p>
                      {item.errorMessage && (
                        <p className="mt-2 text-xs text-rose-600 dark:text-rose-200">{item.errorMessage}</p>
                      )}
                    </div>
                  ))
                )}
              </div>
            ) : (
              <div className="mt-4 space-y-3">
                {syncConflicts.length === 0 ? (
                  <div className="rounded-xl border border-dashed border-slate-300 bg-white/70 p-6 text-sm text-slate-500 dark:border-slate-600 dark:bg-slate-900/70 dark:text-slate-300">
                    暂无待处理冲突。
                  </div>
                ) : (
                  syncConflicts.map((conflict) => (
                    <div key={conflict.id} className="rounded-xl border border-slate-200 bg-white/80 p-4 dark:border-slate-700 dark:bg-slate-900/80">
                      <div className="flex flex-wrap items-center justify-between gap-2">
                        <p className="text-sm font-semibold">{conflict.fileName}</p>
                        <div className="flex flex-wrap gap-2 text-[11px]">
                          <span className="rounded-full bg-slate-100 px-2 py-1 text-slate-600 dark:bg-slate-800 dark:text-slate-200">
                            {conflictKindLabel(conflict.fileKind)}
                          </span>
                          <span className="rounded-full bg-amber-100 px-2 py-1 text-amber-700 dark:bg-amber-500/20 dark:text-amber-200">
                            {conflictNewerLabel(conflict.newerSide)}
                          </span>
                        </div>
                      </div>
                      <p className="mt-1 text-xs text-slate-500 dark:text-slate-300">
                        Diff View · 大小差异 {formatBytes(Math.abs((conflict.remoteSize || 0) - (conflict.localSize || 0)))} · 解决前会把被覆盖版本保留到 .sync-conflicts
                      </p>
                      <div className="mt-3 grid gap-3 md:grid-cols-2">
                        <div className="rounded-lg border border-slate-200 bg-slate-50 p-3 text-xs dark:border-slate-700 dark:bg-slate-800/70">
                          <div className="font-medium text-slate-700 dark:text-slate-100">Local Version</div>
                          <div className="mt-2 space-y-1 text-slate-500 dark:text-slate-300">
                            <div>修改时间：{formatTimestamp(conflict.localTime)}</div>
                            <div>文件大小：{formatBytes(conflict.localSize)}</div>
                            <div className="break-all">{conflict.localPath || '(无路径信息)'}</div>
                          </div>
                        </div>
                        <div className="rounded-lg border border-slate-200 bg-slate-50 p-3 text-xs dark:border-slate-700 dark:bg-slate-800/70">
                          <div className="font-medium text-slate-700 dark:text-slate-100">Cloud Version</div>
                          <div className="mt-2 space-y-1 text-slate-500 dark:text-slate-300">
                            <div>修改时间：{formatTimestamp(conflict.remoteTime)}</div>
                            <div>文件大小：{formatBytes(conflict.remoteSize)}</div>
                            <div className="break-all">{conflict.remotePath || '(无路径信息)'}</div>
                          </div>
                        </div>
                      </div>
                      <div className="mt-3 flex flex-wrap gap-2">
                        <button
                          onClick={() => void handleResolveConflict(conflict.id, 'local')}
                          className="rounded-xl border border-slate-300 bg-white/70 px-3 py-1.5 text-xs text-slate-600 transition hover:bg-slate-50 dark:border-slate-600 dark:bg-slate-900/70 dark:text-slate-200 dark:hover:bg-slate-800"
                        >
                          ← Keep Local
                        </button>
                        <button
                          onClick={() => void handleResolveConflict(conflict.id, 'remote')}
                          className="rounded-xl bg-indigo-600 px-3 py-1.5 text-xs font-medium text-white transition hover:bg-indigo-500"
                        >
                          Keep Cloud →
                        </button>
                      </div>
                    </div>
                  ))
                )}
              </div>
            )}
          </section>

          <section className="de-glass rounded-2xl p-4">
            <h3 className="text-sm font-semibold">Settings</h3>
            <div className="mt-3 grid gap-3 md:grid-cols-5">
              <label className="flex items-center justify-between rounded-xl border border-slate-200 bg-white/80 px-3 py-3 text-sm dark:border-slate-700 dark:bg-slate-900/80">
                <span>Auto-sync on startup</span>
                <input
                  type="checkbox"
                  checked={syncSettings.syncOnStartup}
                  disabled={isSavingSyncSettings}
                  onChange={(event) => void handleSaveSyncSettings({ syncOnStartup: event.target.checked })}
                  className="h-4 w-4"
                />
              </label>
              <label className="flex items-center justify-between rounded-xl border border-slate-200 bg-white/80 px-3 py-3 text-sm dark:border-slate-700 dark:bg-slate-900/80">
                <span>Sync before exit</span>
                <input
                  type="checkbox"
                  checked={syncSettings.syncBeforeExit}
                  disabled={isSavingSyncSettings}
                  onChange={(event) => void handleSaveSyncSettings({ syncBeforeExit: event.target.checked })}
                  className="h-4 w-4"
                />
              </label>
              <label className="flex items-center justify-between rounded-xl border border-slate-200 bg-white/80 px-3 py-3 text-sm dark:border-slate-700 dark:bg-slate-900/80">
                <span>Auto-sync pending changes</span>
                <input
                  type="checkbox"
                  checked={syncSettings.autoSync}
                  disabled={isSavingSyncSettings}
                  onChange={(event) => void handleSaveSyncSettings({ autoSync: event.target.checked })}
                  className="h-4 w-4"
                />
              </label>
              <label className="flex items-center justify-between gap-3 rounded-xl border border-slate-200 bg-white/80 px-3 py-3 text-sm dark:border-slate-700 dark:bg-slate-900/80">
                <span>Interval (min)</span>
                <select
                  value={syncSettings.syncInterval}
                  disabled={isSavingSyncSettings || !syncSettings.autoSync}
                  onChange={(event) => void handleSaveSyncSettings({ syncInterval: Number(event.target.value) || defaultSyncSettings.syncInterval })}
                  className="w-24 rounded-lg border border-slate-200 bg-white px-2 py-1 text-sm dark:border-slate-600 dark:bg-slate-950"
                >
                  {(syncIntervalOptions.includes(syncSettings.syncInterval)
                    ? syncIntervalOptions
                    : [syncSettings.syncInterval, ...syncIntervalOptions]
                  ).map((minutes) => (
                    <option key={minutes} value={minutes}>
                      {minutes}
                    </option>
                  ))}
                </select>
              </label>
              <label className="flex items-center justify-between gap-3 rounded-xl border border-slate-200 bg-white/80 px-3 py-3 text-sm dark:border-slate-700 dark:bg-slate-900/80">
                <span>Conflict strategy</span>
                <select
                  value={syncSettings.conflictResolution}
                  disabled={isSavingSyncSettings}
                  onChange={(event) => void handleSaveSyncSettings({
                    conflictResolution: event.target.value as SyncSettings['conflictResolution'],
                  })}
                  className="w-28 rounded-lg border border-slate-200 bg-white px-2 py-1 text-sm dark:border-slate-600 dark:bg-slate-950"
                >
                  <option value="timestamp">Newest</option>
                  <option value="manual">Manual</option>
                  <option value="local">Local</option>
                  <option value="remote">Cloud</option>
                </select>
              </label>
            </div>

            <div className="mt-3 rounded-xl border border-amber-200 bg-amber-50/80 p-3 text-xs text-amber-800 dark:border-amber-500/40 dark:bg-amber-500/15 dark:text-amber-200">
              <div className="flex items-center gap-2 font-semibold">
                <ShieldAlert className="h-3.5 w-3.5" />
                同步能力说明
              </div>
              <p className="mt-1 leading-6">
                本页读取真实同步状态、历史和冲突，并支持手动同步；设置会保存到本地配置。“自动同步待上传项”会按间隔检查本地变化，并在退出时检测到待上传项时提示或执行关闭前兜底同步。冲突策略为 Manual 时保留冲突列表给你选择；其他策略会尝试自动处理。数据库云端覆盖会先进入安全暂存区，不会在应用运行时直接替换当前数据库。
              </p>
            </div>
          </section>
        </div>
      </div>
    </div>
  );
};
