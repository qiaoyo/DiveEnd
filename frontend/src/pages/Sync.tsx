import React, { useState, useEffect } from 'react';

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
    conflictResolution: 'manual', // 'manual' | 'local' | 'remote' | 'timestamp'
  });

  // Mock loading sync status
  useEffect(() => {
    setIsLoading(true);
    // Simulate API call
    setTimeout(() => {
      setSyncStatus({
        enabled: true,
        provider: 'baidu_cloud',
        lastSync: new Date(Date.now() - 3600000).toISOString(), // 1 hour ago
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
    }, 1000);
  }, []);

  const handleSyncNow = async () => {
    setSyncStatus(prev => ({ ...prev, syncInProgress: true }));
    // TODO: Call backend sync API
    setTimeout(() => {
      setSyncStatus(prev => ({
        ...prev,
        syncInProgress: false,
        lastSync: new Date().toISOString(),
        pendingFiles: 0,
      }));
    }, 3000);
  };

  const handleResolveConflict = (itemId: string) => {
    // TODO: Open conflict resolution dialog
    console.log('Resolve conflict:', itemId);
  };

  const formatTimestamp = (timestamp: string) => {
    const date = new Date(timestamp);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffMins = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMs / 3600000);
    const diffDays = Math.floor(diffMs / 86400000);

    if (diffMins < 1) return 'just now';
    if (diffMins < 60) return `${diffMins}m ago`;
    if (diffHours < 24) return `${diffHours}h ago`;
    if (diffDays < 7) return `${diffDays}d ago`;
    return date.toLocaleDateString();
  };

  if (isLoading) {
    return (
      <div className="h-full flex items-center justify-center">
        <div className="text-center">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-500 mx-auto mb-4"></div>
          <p className="text-gray-600">Loading sync status...</p>
        </div>
      </div>
    );
  }

  return (
    <div className="h-full p-6 overflow-auto">
      <div className="max-w-4xl mx-auto">
        <h1 className="text-2xl font-bold mb-2">Cloud Sync</h1>
        <p className="text-gray-600 mb-6">
          Synchronize your papers and data with Baidu Cloud.
        </p>

        {/* Status Card */}
        <div className="bg-white rounded-lg shadow-sm border p-6 mb-6">
          <div className="flex items-center justify-between mb-4">
            <div className="flex items-center gap-3">
              <div className={`w-3 h-3 rounded-full ${syncStatus.enabled ? 'bg-green-500' : 'bg-gray-400'}`} />
              <h2 className="text-lg font-semibold">
                {syncStatus.enabled ? 'Sync Enabled' : 'Sync Disabled'}
              </h2>
            </div>
            <button
              onClick={() => setShowSettings(!showSettings)}
              className="px-4 py-2 text-gray-600 hover:text-gray-800 transition-colors"
            >
              Settings
            </button>
          </div>

          <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-4">
            <div className="p-3 bg-gray-50 rounded-lg">
              <div className="text-sm text-gray-600">Provider</div>
              <div className="font-medium capitalize">{syncStatus.provider.replace('_', ' ')}</div>
            </div>
            <div className="p-3 bg-gray-50 rounded-lg">
              <div className="text-sm text-gray-600">Last Sync</div>
              <div className="font-medium">
                {syncStatus.lastSync ? formatTimestamp(syncStatus.lastSync) : 'Never'}
              </div>
            </div>
            <div className="p-3 bg-gray-50 rounded-lg">
              <div className="text-sm text-gray-600">Pending Files</div>
              <div className="font-medium">{syncStatus.pendingFiles}</div>
            </div>
            <div className="p-3 bg-gray-50 rounded-lg">
              <div className="text-sm text-gray-600">Conflicts</div>
              <div className={`font-medium ${syncStatus.conflicts > 0 ? 'text-red-600' : ''}`}>
                {syncStatus.conflicts}
              </div>
            </div>
          </div>

          <div className="flex gap-3">
            <button
              onClick={handleSyncNow}
              disabled={syncStatus.syncInProgress || !syncStatus.enabled}
              className="flex-1 px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600 disabled:bg-gray-300 disabled:cursor-not-allowed transition-colors flex items-center justify-center gap-2"
            >
              {syncStatus.syncInProgress ? (
                <>
                  <svg className="animate-spin h-4 w-4" viewBox="0 0 24 24">
                    <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none" />
                    <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
                  </svg>
                  Syncing...
                </>
              ) : (
                'Sync Now'
              )}
            </button>
            <button
              onClick={() => setShowSettings(true)}
              className="px-4 py-2 border border-gray-300 rounded hover:bg-gray-50 transition-colors"
            >
              Configure
            </button>
          </div>
        </div>

        {/* Sync History */}
        <div className="bg-white rounded-lg shadow-sm border p-6 mb-6">
          <h2 className="text-lg font-semibold mb-4">Recent Activity</h2>

          {syncHistory.length === 0 ? (
            <p className="text-gray-500 text-center py-4">No recent activity</p>
          ) : (
            <div className="space-y-2">
              {syncHistory.map((item) => (
                <div
                  key={item.id}
                  className="flex items-center justify-between p-3 bg-gray-50 rounded-lg"
                >
                  <div className="flex items-center gap-3">
                    <span className="text-2xl">
                      {item.type === 'upload' && '⬆️'}
                      {item.type === 'download' && '⬇️'}
                      {item.type === 'conflict' && '⚠️'}
                    </span>
                    <div>
                      <div className="font-medium">{item.fileName}</div>
                      <div className="text-sm text-gray-500 capitalize">
                        {item.type} • {formatTimestamp(item.timestamp)}
                      </div>
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    <span
                      className={`px-2 py-1 rounded text-sm capitalize ${
                        item.status === 'success'
                          ? 'bg-green-100 text-green-800'
                          : item.status === 'failed'
                          ? 'bg-red-100 text-red-800'
                          : 'bg-yellow-100 text-yellow-800'
                      }`}
                    >
                      {item.status}
                    </span>
                    {item.type === 'conflict' && item.status === 'pending' && (
                      <button
                        onClick={() => handleResolveConflict(item.id)}
                        className="px-3 py-1 bg-blue-500 text-white rounded text-sm hover:bg-blue-600"
                      >
                        Resolve
                      </button>
                    )}
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Settings Modal */}
        {showSettings && (
          <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
            <div className="bg-white rounded-lg p-6 w-full max-w-md">
              <h2 className="text-xl font-bold mb-4">Sync Settings</h2>

              <div className="space-y-4">
                <div className="flex items-center justify-between">
                  <label className="font-medium">Enable Auto-Sync</label>
                  <input
                    type="checkbox"
                    checked={syncSettings.autoSync}
                    onChange={(e) => setSyncSettings(prev => ({ ...prev, autoSync: e.target.checked }))}
                    className="w-5 h-5"
                  />
                </div>

                <div className="flex items-center justify-between">
                  <label className="font-medium">Sync on Startup</label>
                  <input
                    type="checkbox"
                    checked={syncSettings.syncOnStartup}
                    onChange={(e) => setSyncSettings(prev => ({ ...prev, syncOnStartup: e.target.checked }))}
                    className="w-5 h-5"
                  />
                </div>

                <div>
                  <label className="block font-medium mb-1">Sync Interval (minutes)</label>
                  <input
                    type="number"
                    value={syncSettings.syncInterval}
                    onChange={(e) => setSyncSettings(prev => ({ ...prev, syncInterval: parseInt(e.target.value) }))}
                    min="5"
                    max="1440"
                    className="w-full px-3 py-2 border rounded"
                  />
                </div>

                <div>
                  <label className="block font-medium mb-1">Conflict Resolution</label>
                  <select
                    value={syncSettings.conflictResolution}
                    onChange={(e) => setSyncSettings(prev => ({ ...prev, conflictResolution: e.target.value }))}
                    className="w-full px-3 py-2 border rounded"
                  >
                    <option value="manual">Manual (Ask me)</option>
                    <option value="local">Always use local version</option>
                    <option value="remote">Always use remote version</option>
                    <option value="timestamp">Use newest by timestamp</option>
                  </select>
                </div>
              </div>

              <div className="flex justify-end gap-2 mt-6">
                <button
                  onClick={() => setShowSettings(false)}
                  className="px-4 py-2 text-gray-600 hover:text-gray-800"
                >
                  Cancel
                </button>
                <button
                  onClick={() => {
                    // TODO: Save settings to backend
                    setShowSettings(false);
                  }}
                  className="px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600"
                >
                  Save Settings
                </button>
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
};
