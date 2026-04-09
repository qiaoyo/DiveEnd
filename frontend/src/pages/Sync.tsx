import React, { useEffect, useState } from 'react';
import { Cloud, RefreshCw, Settings2, ShieldAlert } from 'lucide-react';
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

export const Sync: React.FC = () => {
  const [syncStatus, setSyncStatus] = useState<SyncStatus>(emptyStatus);
  const [syncProgress, setSyncProgress] = useState<SyncProgress>(emptyProgress);
  const [syncHistory, setSyncHistory] = useState<SyncRecord[]>([]);
  const [syncConflicts, setSyncConflicts] = useState<SyncConflict[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [showSettings, setShowSettings] = useState(false);
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
      backend.getSyncRecords(20),
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

  const statusCards = [
    { label: '同步提供方', value: syncStatus.provider.replace('_', ' '), accent: '' },
    { label: '最近同步', value: formatTimestamp(syncStatus.lastSync), accent: '' },
    { label: '待处理文件', value: String(syncStatus.pendingFiles), accent: '' },
    { label: '累计成功', value: String(syncStatus.totalSynced), accent: '' },
    {
      label: '累计失败',
      value: String(syncStatus.totalFailed),
      accent: syncStatus.totalFailed > 0 ? 'text-rose-600 dark:text-rose-300' : '',
    },
    {
      label: '冲突数量',
      value: String(syncStatus.conflicts),
      accent: syncStatus.conflicts > 0 ? 'text-rose-600 dark:text-rose-300' : '',
    },
  ];

  if (isLoading) {
    return (
      <div className="flex h-full min-h-0 items-center justify-center bg-[#f9f6f0] p-8 dark:bg-[#141414]">
        <div className="rounded-[2rem] border border-stone-200 bg-white/80 px-8 py-10 text-center shadow-sm dark:border-stone-800 dark:bg-stone-900/70">
          <RefreshCw className="mx-auto mb-4 h-10 w-10 animate-spin text-emerald-600" />
          <p className="text-sm text-stone-500 dark:text-stone-400">正在加载同步状态...</p>
        </div>
      </div>
    );
  }

  return (
    <div className="flex h-full min-h-0 flex-col bg-[#f9f6f0] dark:bg-[#141414]">
      <div className="border-b border-stone-200 p-6 dark:border-stone-800">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div>
            <h2 className="flex items-center gap-2 text-lg font-semibold">
              <Cloud className="h-5 w-5 text-emerald-600" />
              Sync - 云同步
            </h2>
            <p className="mt-1 text-sm text-stone-500 dark:text-stone-400">
              这一页现在直接读取真实后端状态，不再展示定时器伪造的数据。
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            <button
              onClick={() => void handleRefresh()}
              disabled={isRefreshing}
              className="inline-flex items-center gap-2 rounded-2xl border border-stone-200 bg-white px-4 py-2 text-sm text-stone-600 transition hover:border-emerald-400 hover:text-emerald-700 disabled:cursor-not-allowed disabled:opacity-60 dark:border-stone-700 dark:bg-stone-900 dark:text-stone-300"
            >
              <RefreshCw className={`h-4 w-4 ${isRefreshing ? 'animate-spin' : ''}`} />
              刷新状态
            </button>
            <button
              onClick={() => setShowSettings(true)}
              className="inline-flex items-center gap-2 rounded-2xl border border-stone-200 bg-white px-4 py-2 text-sm text-stone-600 transition hover:border-emerald-400 hover:text-emerald-700 dark:border-stone-700 dark:bg-stone-900 dark:text-stone-300"
            >
              <Settings2 className="h-4 w-4" />
              同步设置
            </button>
          </div>
        </div>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto p-6">
        <div className="mx-auto max-w-5xl space-y-6">
          {error && (
            <div className="rounded-2xl border border-rose-200 bg-rose-50/80 px-4 py-3 text-sm text-rose-700 dark:border-rose-900 dark:bg-rose-950/20 dark:text-rose-200">
              {error}
            </div>
          )}

          <div className="rounded-[2rem] border border-stone-200 bg-white/80 p-6 shadow-sm dark:border-stone-800 dark:bg-stone-900/70">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div>
                <div className="flex items-center gap-3">
                  <span className={`h-3 w-3 rounded-full ${syncStatus.enabled ? 'bg-emerald-500' : 'bg-stone-400'}`} />
                  <p className="text-lg font-semibold">{syncStatus.enabled ? '同步已启用' : '同步未启用'}</p>
                </div>
                <p className="mt-2 text-sm text-stone-500 dark:text-stone-400">
                  需要在设置中启用百度同步并提供可用 token 后，手动同步入口才会真正工作。
                </p>
              </div>
              <button
                onClick={() => void handleSyncNow()}
                disabled={isRefreshing || !syncStatus.enabled}
                className="inline-flex items-center gap-2 rounded-2xl bg-emerald-600 px-4 py-2.5 text-sm font-medium text-white transition hover:bg-emerald-700 disabled:cursor-not-allowed disabled:opacity-60"
              >
                <RefreshCw className={`h-4 w-4 ${isRefreshing ? 'animate-spin' : ''}`} />
                {isRefreshing ? '同步中...' : '立即同步'}
              </button>
            </div>

            <div className="mt-5 grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3">
              {statusCards.map((item) => (
                <div key={item.label} className="rounded-2xl border border-stone-200 bg-stone-50/80 p-4 dark:border-stone-800 dark:bg-stone-950/60">
                  <div className="text-xs uppercase tracking-[0.18em] text-stone-400 dark:text-stone-500">{item.label}</div>
                  <div className={`mt-2 text-base font-semibold capitalize ${item.accent}`}>{item.value}</div>
                </div>
              ))}
            </div>

            {(syncStatus.syncInProgress || syncProgress.currentFile || syncProgress.message) && (
              <div className="mt-6 rounded-2xl border border-stone-200 bg-stone-50/80 p-4 dark:border-stone-800 dark:bg-stone-950/60">
                <div className="flex flex-wrap items-center justify-between gap-3">
                  <div>
                    <div className="font-medium">当前进度</div>
                    <div className="mt-1 text-sm text-stone-500 dark:text-stone-400">
                      {syncProgress.message || syncProgress.currentFile || '等待同步任务启动'}
                    </div>
                  </div>
                  <div className="text-sm text-stone-500 dark:text-stone-400">
                    {syncProgress.completed} / {syncProgress.total}
                  </div>
                </div>
                <div className="mt-3 h-2 rounded-full bg-stone-200 dark:bg-stone-800">
                  <div className="h-2 rounded-full bg-emerald-600 transition-all" style={{ width: `${progressPercent}%` }} />
                </div>
              </div>
            )}
          </div>

          <div className="rounded-[2rem] border border-stone-200 bg-white/80 p-6 shadow-sm dark:border-stone-800 dark:bg-stone-900/70">
            <div className="flex items-center gap-2">
              <ShieldAlert className="h-5 w-5 text-amber-500" />
              <h3 className="text-lg font-semibold">当前已知边界</h3>
            </div>
            <ul className="mt-4 space-y-2 text-sm leading-7 text-stone-600 dark:text-stone-300">
              <li>当前实现优先做“真实状态面板 + 手动触发同步”，还没有完成完整的双向自动同步策略。</li>
              <li>冲突解决目前只会记录与标记选择，不会自动替你执行本地或远端覆盖。</li>
              <li>如果百度 token 失效或缺少完整 OAuth 信息，后端会如实返回错误，不会再静默伪装成同步成功。</li>
            </ul>
          </div>

          <div className="rounded-[2rem] border border-stone-200 bg-white/80 p-6 shadow-sm dark:border-stone-800 dark:bg-stone-900/70">
            <h3 className="text-lg font-semibold">冲突列表</h3>
            {syncConflicts.length === 0 ? (
              <div className="mt-4 rounded-2xl border border-dashed border-stone-300 bg-stone-50/80 p-6 text-sm text-stone-500 dark:border-stone-700 dark:bg-stone-950/50 dark:text-stone-400">
                暂无待处理冲突。
              </div>
            ) : (
              <div className="mt-4 space-y-3">
                {syncConflicts.map((conflict) => (
                  <div key={conflict.id} className="rounded-2xl border border-stone-200 bg-stone-50/80 p-4 dark:border-stone-800 dark:bg-stone-950/60">
                    <div className="flex flex-wrap items-center justify-between gap-3">
                      <div>
                        <div className="font-medium">{conflict.fileName}</div>
                        <div className="mt-1 text-sm text-stone-500 dark:text-stone-400">
                          本地：{formatTimestamp(conflict.localTime)} · 远端：{formatTimestamp(conflict.remoteTime)}
                        </div>
                      </div>
                      <div className="flex flex-wrap gap-2">
                        <button
                          onClick={() => void handleResolveConflict(conflict.id, 'local')}
                          className="rounded-2xl border border-stone-200 px-3 py-1.5 text-sm text-stone-600 transition hover:bg-stone-100 dark:border-stone-700 dark:text-stone-300 dark:hover:bg-stone-800"
                        >
                          保留本地
                        </button>
                        <button
                          onClick={() => void handleResolveConflict(conflict.id, 'remote')}
                          className="rounded-2xl bg-emerald-600 px-3 py-1.5 text-sm font-medium text-white transition hover:bg-emerald-700"
                        >
                          保留远端
                        </button>
                      </div>
                    </div>
                    {conflict.resolution && (
                      <div className="mt-3 text-sm text-stone-500 dark:text-stone-400">
                        当前记录的处理结果：{conflict.resolution}
                      </div>
                    )}
                  </div>
                ))}
              </div>
            )}
          </div>

          <div className="rounded-[2rem] border border-stone-200 bg-white/80 p-6 shadow-sm dark:border-stone-800 dark:bg-stone-900/70">
            <h3 className="text-lg font-semibold">最近活动</h3>
            {syncHistory.length === 0 ? (
              <div className="mt-4 rounded-2xl border border-dashed border-stone-300 bg-stone-50/80 p-6 text-sm text-stone-500 dark:border-stone-700 dark:bg-stone-950/50 dark:text-stone-400">
                暂无同步活动记录。
              </div>
            ) : (
              <div className="mt-4 space-y-3">
                {syncHistory.map((item) => (
                  <div key={item.id} className="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-stone-200 bg-stone-50/80 p-4 dark:border-stone-800 dark:bg-stone-950/60">
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2">
                        <span className="text-lg">
                          {item.type === 'upload' && '⬆️'}
                          {item.type === 'download' && '⬇️'}
                          {item.type === 'conflict' && '⚠️'}
                        </span>
                        <p className="truncate font-medium">{item.fileName}</p>
                      </div>
                      <p className="mt-1 text-sm text-stone-500 dark:text-stone-400">
                        {item.type} · {formatTimestamp(item.completedAt || item.createdAt)}
                      </p>
                      {item.errorMessage && (
                        <p className="mt-2 text-sm text-rose-600 dark:text-rose-300">{item.errorMessage}</p>
                      )}
                    </div>
                    <span
                      className={`rounded-full px-3 py-1 text-xs font-medium capitalize ${
                        item.status === 'success'
                          ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300'
                          : item.status === 'failed'
                            ? 'bg-rose-100 text-rose-700 dark:bg-rose-950 dark:text-rose-300'
                            : 'bg-amber-100 text-amber-700 dark:bg-amber-950 dark:text-amber-300'
                      }`}
                    >
                      {item.status}
                    </span>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      </div>

      {showSettings && (
        <div className="absolute inset-0 z-50 flex items-center justify-center bg-black/45 p-4">
          <div className="w-full max-w-lg rounded-[2rem] border border-stone-200 bg-white p-6 shadow-xl dark:border-stone-800 dark:bg-stone-900">
            <h3 className="text-lg font-semibold">同步策略</h3>
            <p className="mt-2 text-sm text-stone-500 dark:text-stone-400">
              当前弹窗只保留策略草图，真正生效的凭据与开关仍由设置栏配置管理。
            </p>
            <div className="mt-5 space-y-4">
              <div className="flex items-center justify-between rounded-2xl border border-stone-200 px-4 py-3 dark:border-stone-800">
                <span className="font-medium">自动同步</span>
                <input
                  type="checkbox"
                  checked={syncSettings.autoSync}
                  onChange={(event) => setSyncSettings((prev) => ({ ...prev, autoSync: event.target.checked }))}
                  className="h-5 w-5"
                />
              </div>
              <div className="flex items-center justify-between rounded-2xl border border-stone-200 px-4 py-3 dark:border-stone-800">
                <span className="font-medium">启动时同步</span>
                <input
                  type="checkbox"
                  checked={syncSettings.syncOnStartup}
                  onChange={(event) => setSyncSettings((prev) => ({ ...prev, syncOnStartup: event.target.checked }))}
                  className="h-5 w-5"
                />
              </div>
              <div>
                <label className="mb-1 block text-sm font-medium">同步间隔（分钟）</label>
                <input
                  type="number"
                  min="5"
                  max="1440"
                  value={syncSettings.syncInterval}
                  onChange={(event) =>
                    setSyncSettings((prev) => ({ ...prev, syncInterval: Number(event.target.value) || 30 }))
                  }
                  className="w-full rounded-xl border border-stone-200 px-3 py-2 text-sm outline-none transition focus:border-emerald-500 dark:border-stone-700 dark:bg-stone-950"
                />
              </div>
              <div>
                <label className="mb-1 block text-sm font-medium">冲突处理策略</label>
                <select
                  value={syncSettings.conflictResolution}
                  onChange={(event) => setSyncSettings((prev) => ({ ...prev, conflictResolution: event.target.value }))}
                  className="w-full rounded-xl border border-stone-200 px-3 py-2 text-sm outline-none transition focus:border-emerald-500 dark:border-stone-700 dark:bg-stone-950"
                >
                  <option value="timestamp">按更新时间判断</option>
                  <option value="local">优先本地版本</option>
                  <option value="remote">优先远端版本</option>
                </select>
              </div>
            </div>
            <div className="mt-6 flex justify-end gap-2">
              <button
                onClick={() => setShowSettings(false)}
                className="rounded-2xl border border-stone-200 px-4 py-2 text-sm text-stone-600 transition hover:bg-stone-50 dark:border-stone-700 dark:text-stone-300 dark:hover:bg-stone-800"
              >
                关闭
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
