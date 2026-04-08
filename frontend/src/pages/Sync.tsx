import React, { useEffect, useState } from 'react';
import { Cloud, RefreshCw, Settings2, ShieldAlert } from 'lucide-react';

interface SyncStatus {
  enabled: boolean;
  provider: string;
  lastSync: string | null;
  syncInProgress: boolean;
  pendingFiles: number;
  conflicts: number;
}

interface SyncHistoryItem {
  id: string;
  timestamp: string;
  type: 'upload' | 'download' | 'conflict';
  fileName: string;
  status: 'success' | 'failed' | 'pending';
}

export const Sync: React.FC = () => {
  const [syncStatus, setSyncStatus] = useState<SyncStatus>({
    enabled: false,
    provider: 'baidu_cloud',
    lastSync: null,
    syncInProgress: false,
    pendingFiles: 0,
    conflicts: 0,
  });
  const [syncHistory, setSyncHistory] = useState<SyncHistoryItem[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [showSettings, setShowSettings] = useState(false);
  const [syncSettings, setSyncSettings] = useState({
    autoSync: true,
    syncOnStartup: true,
    syncInterval: 30,
    conflictResolution: 'manual',
  });

  useEffect(() => {
    setIsLoading(true);
    const timer = window.setTimeout(() => {
      setSyncStatus({
        enabled: true,
        provider: 'baidu_cloud',
        lastSync: new Date(Date.now() - 3600000).toISOString(),
        syncInProgress: false,
        pendingFiles: 3,
        conflicts: 1,
      });
      setSyncHistory([
        {
          id: '1',
          timestamp: new Date(Date.now() - 3600000).toISOString(),
          type: 'upload',
          fileName: 'paper1.pdf',
          status: 'success',
        },
        {
          id: '2',
          timestamp: new Date(Date.now() - 7200000).toISOString(),
          type: 'download',
          fileName: 'paper2.pdf',
          status: 'success',
        },
        {
          id: '3',
          timestamp: new Date(Date.now() - 86400000).toISOString(),
          type: 'conflict',
          fileName: 'notes.md',
          status: 'pending',
        },
      ]);
      setIsLoading(false);
    }, 600);

    return () => window.clearTimeout(timer);
  }, []);

  const formatTimestamp = (timestamp: string) => {
    const date = new Date(timestamp);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffMins = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMs / 3600000);
    const diffDays = Math.floor(diffMs / 86400000);

    if (diffMins < 1) return '刚刚';
    if (diffMins < 60) return `${diffMins} 分钟前`;
    if (diffHours < 24) return `${diffHours} 小时前`;
    if (diffDays < 7) return `${diffDays} 天前`;
    return date.toLocaleDateString();
  };

  const handleSyncNow = async () => {
    setSyncStatus((prev) => ({ ...prev, syncInProgress: true }));
    window.setTimeout(() => {
      setSyncStatus((prev) => ({
        ...prev,
        syncInProgress: false,
        lastSync: new Date().toISOString(),
        pendingFiles: 0,
      }));
    }, 2000);
  };

  const statusCards = [
    { label: '同步提供方', value: syncStatus.provider.replace('_', ' '), accent: '' },
    { label: '最近同步', value: syncStatus.lastSync ? formatTimestamp(syncStatus.lastSync) : '从未同步', accent: '' },
    { label: '待处理文件', value: String(syncStatus.pendingFiles), accent: '' },
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
              当前页面先聚焦状态可视化与凭据落位，真实同步链路仍在整理中。
            </p>
          </div>
          <button
            onClick={() => setShowSettings(true)}
            className="inline-flex items-center gap-2 rounded-2xl border border-stone-200 bg-white px-4 py-2 text-sm text-stone-600 transition hover:border-emerald-400 hover:text-emerald-700 dark:border-stone-700 dark:bg-stone-900 dark:text-stone-300"
          >
            <Settings2 className="h-4 w-4" />
            同步设置
          </button>
        </div>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto p-6">
        <div className="mx-auto max-w-5xl space-y-6">
          <div className="rounded-[2rem] border border-stone-200 bg-white/80 p-6 shadow-sm dark:border-stone-800 dark:bg-stone-900/70">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div>
                <div className="flex items-center gap-3">
                  <span className={`h-3 w-3 rounded-full ${syncStatus.enabled ? 'bg-emerald-500' : 'bg-stone-400'}`} />
                  <p className="text-lg font-semibold">{syncStatus.enabled ? '同步已启用' : '同步未启用'}</p>
                </div>
                <p className="mt-2 text-sm text-stone-500 dark:text-stone-400">
                  目前仅保存百度网盘凭据与策略；真正的增量同步与冲突处理仍处于过渡开发阶段。
                </p>
              </div>
              <button
                onClick={() => void handleSyncNow()}
                disabled={syncStatus.syncInProgress || !syncStatus.enabled}
                className="inline-flex items-center gap-2 rounded-2xl bg-emerald-600 px-4 py-2.5 text-sm font-medium text-white transition hover:bg-emerald-700 disabled:cursor-not-allowed disabled:opacity-60"
              >
                <RefreshCw className={`h-4 w-4 ${syncStatus.syncInProgress ? 'animate-spin' : ''}`} />
                {syncStatus.syncInProgress ? '同步中...' : '立即同步'}
              </button>
            </div>

            <div className="mt-5 grid grid-cols-1 gap-3 md:grid-cols-2">
              {statusCards.map((item) => (
                <div key={item.label} className="rounded-2xl border border-stone-200 bg-stone-50/80 p-4 dark:border-stone-800 dark:bg-stone-950/60">
                  <div className="text-xs uppercase tracking-[0.18em] text-stone-400 dark:text-stone-500">{item.label}</div>
                  <div className={`mt-2 text-base font-semibold capitalize ${item.accent}`}>{item.value}</div>
                </div>
              ))}
            </div>
          </div>

          <div className="rounded-[2rem] border border-stone-200 bg-white/80 p-6 shadow-sm dark:border-stone-800 dark:bg-stone-900/70">
            <div className="flex items-center gap-2">
              <ShieldAlert className="h-5 w-5 text-amber-500" />
              <h3 className="text-lg font-semibold">当前已知限制</h3>
            </div>
            <ul className="mt-4 space-y-2 text-sm leading-7 text-stone-600 dark:text-stone-300">
              <li>真实同步任务、冲突解决弹窗和后端状态回传仍未闭环。</li>
              <li>此页当前更适合作为“状态仪表盘”和“后续接入占位”，不应误解为已完成的同步产品。</li>
              <li>百度网盘 token 已在设置栏统一管理，这里不再重复暴露敏感字段。</li>
            </ul>
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
                        {item.type} · {formatTimestamp(item.timestamp)}
                      </p>
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
                  <option value="manual">手动确认</option>
                  <option value="local">优先本地版本</option>
                  <option value="remote">优先远端版本</option>
                  <option value="timestamp">按更新时间判断</option>
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
              <button
                onClick={() => setShowSettings(false)}
                className="rounded-2xl bg-stone-900 px-4 py-2 text-sm font-medium text-white transition hover:bg-stone-700 dark:bg-stone-100 dark:text-stone-900 dark:hover:bg-white"
              >
                保存占位设置
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
