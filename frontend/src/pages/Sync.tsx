import React, { useEffect, useMemo, useState } from 'react';
import { Cloud, RefreshCw, ShieldAlert } from 'lucide-react';
import type { SyncConflict, SyncProgress, SyncRecord, SyncStatus } from '../types';
import * as backend from '../lib/backend';

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

type SyncTab = 'history' | 'conflicts';

export const Sync: React.FC = () => {
  const [syncStatus, setSyncStatus] = useState<SyncStatus>(emptyStatus);
  const [syncProgress, setSyncProgress] = useState<SyncProgress>(emptyProgress);
  const [syncHistory, setSyncHistory] = useState<SyncRecord[]>([]);
  const [syncConflicts, setSyncConflicts] = useState<SyncConflict[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<SyncTab>('history');
  const [syncSettings, setSyncSettings] = useState({
    autoSync: true,
    syncOnStartup: true,
    syncInterval: 30,
    conflictResolution: 'timestamp',
  });

  const loadSyncState = async () => {
    const [status, progress, conflicts, records] = await Promise.all([
      backend.getSyncStatus(),
      backend.getSyncProgress(),
      backend.getSyncConflicts(),
      backend.getSyncRecords(30),
    ]);

    setSyncStatus(status);
    setSyncProgress(progress);
    setSyncConflicts(conflicts);
    setSyncHistory(records);
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
          setError(cause instanceof Error ? cause.message : '加载同步状态失败');
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
      setError(cause instanceof Error ? cause.message : '刷新同步状态失败');
    } finally {
      setIsRefreshing(false);
    }
  };

  const handleSyncNow = async () => {
    setIsRefreshing(true);
    setError(null);
    try {
      await backend.triggerSync();
      await loadSyncState();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '触发同步失败');
    } finally {
      setIsRefreshing(false);
    }
  };

  const handleResolveConflict = async (conflictId: string, resolution: 'local' | 'remote') => {
    setIsRefreshing(true);
    setError(null);
    try {
      await backend.resolveSyncConflict(conflictId, resolution);
      await loadSyncState();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '解决冲突失败');
    } finally {
      setIsRefreshing(false);
    }
  };

  const progressPercent = syncProgress.total > 0
    ? Math.round((syncProgress.completed / syncProgress.total) * 100)
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
              onClick={() => void handleSyncNow()}
              disabled={isRefreshing || !syncStatus.enabled}
              className="inline-flex items-center gap-1.5 rounded-xl bg-indigo-600 px-4 py-2 text-sm font-medium text-white transition hover:bg-indigo-500 disabled:cursor-not-allowed disabled:opacity-60"
            >
              <RefreshCw className={`h-4 w-4 ${isRefreshing ? 'animate-spin' : ''}`} />
              {isRefreshing ? '同步中...' : 'Manual Sync'}
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
                        {item.type} · {formatTimestamp(item.completedAt || item.createdAt)}
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
                      <p className="text-sm font-semibold">{conflict.fileName}</p>
                      <p className="mt-1 text-xs text-slate-500 dark:text-slate-300">
                        Diff View · 本地 {formatTimestamp(conflict.localTime)} / 云端 {formatTimestamp(conflict.remoteTime)}
                      </p>
                      <div className="mt-3 grid gap-3 md:grid-cols-2">
                        <div className="rounded-lg border border-slate-200 bg-slate-50 p-3 text-xs dark:border-slate-700 dark:bg-slate-800/70">
                          <div className="font-medium text-slate-700 dark:text-slate-100">Local Version</div>
                          <div className="mt-2 text-slate-500 dark:text-slate-300">{conflict.localPath || '(无路径信息)'}</div>
                        </div>
                        <div className="rounded-lg border border-slate-200 bg-slate-50 p-3 text-xs dark:border-slate-700 dark:bg-slate-800/70">
                          <div className="font-medium text-slate-700 dark:text-slate-100">Cloud Version</div>
                          <div className="mt-2 text-slate-500 dark:text-slate-300">{conflict.remotePath || '(无路径信息)'}</div>
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
            <div className="mt-3 grid gap-3 md:grid-cols-2">
              <label className="flex items-center justify-between rounded-xl border border-slate-200 bg-white/80 px-3 py-3 text-sm dark:border-slate-700 dark:bg-slate-900/80">
                <span>Auto-sync on startup</span>
                <input
                  type="checkbox"
                  checked={syncSettings.syncOnStartup}
                  onChange={(event) => setSyncSettings((prev) => ({ ...prev, syncOnStartup: event.target.checked }))}
                  className="h-4 w-4"
                />
              </label>
              <label className="flex items-center justify-between rounded-xl border border-slate-200 bg-white/80 px-3 py-3 text-sm dark:border-slate-700 dark:bg-slate-900/80">
                <span>Check unsynced before exit</span>
                <input
                  type="checkbox"
                  checked={syncSettings.autoSync}
                  onChange={(event) => setSyncSettings((prev) => ({ ...prev, autoSync: event.target.checked }))}
                  className="h-4 w-4"
                />
              </label>
            </div>

            <div className="mt-3 rounded-xl border border-amber-200 bg-amber-50/80 p-3 text-xs text-amber-800 dark:border-amber-500/40 dark:bg-amber-500/15 dark:text-amber-200">
              <div className="flex items-center gap-2 font-semibold">
                <ShieldAlert className="h-3.5 w-3.5" />
                当前阶段说明
              </div>
              <p className="mt-1 leading-6">
                本页已是“真实状态 + 手动触发同步”，完整自动双向同步和冲突自动合并仍在下一阶段。
              </p>
            </div>
          </section>
        </div>
      </div>
    </div>
  );
};
