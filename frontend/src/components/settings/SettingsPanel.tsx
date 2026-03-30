import { useState } from 'react';
import { Settings, Cloud, Key, Palette, ChevronLeft, ChevronRight } from 'lucide-react';
import { useAppStore } from '../../stores/appStore';
import * as Switch from '@radix-ui/react-switch';

export function SettingsPanel() {
  const { config, setConfig, toggleTheme, theme, toggleLeftPanel, leftPanelCollapsed } = useAppStore();
  const [activeTab, setActiveTab] = useState<'api' | 'cloud' | 'ui'>('api');

  return (
    <div className="h-full flex flex-col bg-surface border-r border-border">
      {/* Header */}
      <div className="flex items-center justify-between p-4 border-b border-border">
        <div className="flex items-center gap-2">
          <Settings className="w-5 h-5 text-primary" />
          <span className="font-semibold">设置</span>
        </div>
        <button 
          onClick={toggleLeftPanel}
          className="p-1 hover:bg-border rounded"
        >
          {leftPanelCollapsed ? <ChevronRight className="w-4 h-4" /> : <ChevronLeft className="w-4 h-4" />}
        </button>
      </div>

      {/* Tabs */}
      <div className="flex border-b border-border">
        <button
          onClick={() => setActiveTab('api')}
          className={`flex-1 py-2 text-sm font-medium ${activeTab === 'api' ? 'text-primary border-b-2 border-primary' : 'text-text-muted'}`}
        >
          <Key className="w-4 h-4 inline-block mr-1" />
          API
        </button>
        <button
          onClick={() => setActiveTab('cloud')}
          className={`flex-1 py-2 text-sm font-medium ${activeTab === 'cloud' ? 'text-primary border-b-2 border-primary' : 'text-text-muted'}`}
        >
          <Cloud className="w-4 h-4 inline-block mr-1" />
          云同步
        </button>
        <button
          onClick={() => setActiveTab('ui')}
          className={`flex-1 py-2 text-sm font-medium ${activeTab === 'ui' ? 'text-primary border-b-2 border-primary' : 'text-text-muted'}`}
        >
          <Palette className="w-4 h-4 inline-block mr-1" />
          界面
        </button>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto p-4">
        {activeTab === 'api' && (
          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium mb-1">选择模型</label>
              <select
                value={config.selectedModel}
                onChange={(e) => setConfig({ selectedModel: e.target.value as 'openai' | 'anthropic' })}
                className="w-full p-2 border border-border rounded bg-background"
              >
                <option value="anthropic">Anthropic Claude</option>
                <option value="openai">OpenAI GPT</option>
              </select>
            </div>

            <div>
              <label className="block text-sm font-medium mb-1">
                {config.selectedModel === 'anthropic' ? 'Anthropic API Key' : 'OpenAI API Key'}
              </label>
              <input
                type="password"
                value={config.selectedModel === 'anthropic' ? config.anthropicApiKey : config.openaiApiKey}
                onChange={(e) => setConfig(
                  config.selectedModel === 'anthropic' 
                    ? { anthropicApiKey: e.target.value } 
                    : { openaiApiKey: e.target.value }
                )}
                placeholder="sk-..."
                className="w-full p-2 border border-border rounded bg-background text-sm"
              />
            </div>

            <div>
              <label className="block text-sm font-medium mb-1">Semantic Scholar API Key (可选)</label>
              <input
                type="password"
                value={config.semanticScholarApiKey}
                onChange={(e) => setConfig({ semanticScholarApiKey: e.target.value })}
                placeholder="用于提升搜索限额"
                className="w-full p-2 border border-border rounded bg-background text-sm"
              />
            </div>
          </div>
        )}

        {activeTab === 'cloud' && (
          <div className="space-y-4">
            <div className="flex items-center justify-between">
              <span className="font-medium">启用百度网盘同步</span>
              <Switch.Root
                checked={config.baiduCloud.enabled}
                onCheckedChange={(checked) => setConfig({ 
                  baiduCloud: { ...config.baiduCloud, enabled: checked } 
                })}
                className="w-10 h-5 bg-border rounded-full relative data-[state=checked]:bg-primary"
              >
                <Switch.Thumb className="block w-4 h-4 bg-white rounded-full transition-transform translate-x-0.5 data-[state=checked]:translate-x-5" />
              </Switch.Root>
            </div>

            {config.baiduCloud.enabled && (
              <div>
                <label className="block text-sm font-medium mb-1">百度网盘 Access Token</label>
                <input
                  type="password"
                  value={config.baiduCloud.token}
                  onChange={(e) => setConfig({ 
                    baiduCloud: { ...config.baiduCloud, token: e.target.value } 
                  })}
                  placeholder="OAuth 获取的 token"
                  className="w-full p-2 border border-border rounded bg-background text-sm"
                />
              </div>
            )}
          </div>
        )}

        {activeTab === 'ui' && (
          <div className="space-y-4">
            <div className="flex items-center justify-between">
              <span className="font-medium">深色模式</span>
              <Switch.Root
                checked={theme === 'dark'}
                onCheckedChange={toggleTheme}
                className="w-10 h-5 bg-border rounded-full relative data-[state=checked]:bg-primary"
              >
                <Switch.Thumb className="block w-4 h-4 bg-white rounded-full transition-transform translate-x-0.5 data-[state=checked]:translate-x-5" />
              </Switch.Root>
            </div>

            <div>
              <label className="block text-sm font-medium mb-1">数据存储路径</label>
              <input
                type="text"
                value={config.dataPath}
                onChange={(e) => setConfig({ dataPath: e.target.value })}
                className="w-full p-2 border border-border rounded bg-background text-sm"
              />
            </div>
          </div>
        )}
      </div>
    </div>
  );
}