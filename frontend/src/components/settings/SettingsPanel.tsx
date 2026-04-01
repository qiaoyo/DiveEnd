import { useEffect, useState } from 'react';
import * as Switch from '@radix-ui/react-switch';
import {
  Bot,
  ChevronLeft,
  ChevronRight,
  Cloud,
  Key,
  Palette,
  Save,
  Server,
  Settings,
  Shield,
  Trash2,
} from 'lucide-react';
import { saveConfig } from '../../lib/backend';
import { useAppStore } from '../../stores/appStore';
import type { AppConfig, LLMConfig } from '../../types';

const providerPresets: Record<'openai_compatible' | 'anthropic', Partial<LLMConfig>> = {
  openai_compatible: {
    providerId: 'openai',
    providerName: 'OpenAI',
    providerType: 'openai_compatible',
    baseUrl: 'https://api.openai.com/v1',
    wireApi: 'responses',
    requiresOpenAIAuth: true,
    model: 'gpt-4o-mini',
    disableResponseStorage: true,
  },
  anthropic: {
    providerId: 'anthropic',
    providerName: 'Anthropic',
    providerType: 'anthropic',
    baseUrl: 'https://api.anthropic.com/v1',
    wireApi: 'anthropic_messages',
    requiresOpenAIAuth: false,
    model: 'claude-3-5-sonnet-20241022',
    disableResponseStorage: true,
  },
};

function applyProviderPreset(current: AppConfig, providerType: 'openai_compatible' | 'anthropic'): AppConfig {
  const preset = providerPresets[providerType];
  return {
    ...current,
    llm: {
      ...current.llm,
      ...preset,
      providerType,
      apiKey: '',
      clearApiKey: false,
      hasApiKey: providerType === current.llm.providerType ? current.llm.hasApiKey : false,
      reasoningEffort: providerType === 'openai_compatible' ? current.llm.reasoningEffort : '',
    },
  };
}

export function SettingsPanel() {
  const {
    config,
    isSavingConfig,
    leftPanelCollapsed,
    setConfig,
    setError,
    setSavingConfig,
    toggleLeftPanel,
  } = useAppStore();
  const [activeTab, setActiveTab] = useState<'api' | 'cloud' | 'ui'>('api');
  const [draftConfig, setDraftConfig] = useState<AppConfig>(config);
  const [saveMessage, setSaveMessage] = useState('');

  useEffect(() => {
    setDraftConfig(config);
  }, [config]);

  const updateDraft = (patch: Partial<AppConfig>) => {
    setDraftConfig((current) => ({ ...current, ...patch }));
  };

  const updateLLM = (patch: Partial<LLMConfig>) => {
    setDraftConfig((current) => ({
      ...current,
      llm: {
        ...current.llm,
        ...patch,
      },
    }));
  };

  const handleProviderTypeChange = (providerType: 'openai_compatible' | 'anthropic') => {
    setDraftConfig((current) => applyProviderPreset(current, providerType));
  };

  const handleSave = async () => {
    setSavingConfig(true);
    setSaveMessage('');
    try {
      const result = await saveConfig(draftConfig);
      setConfig(result.config);
      setSaveMessage(
        result.restartRequired ? '配置已保存，数据路径改动将在重启后生效。' : '配置已保存。'
      );
    } catch (error) {
      const message = error instanceof Error ? error.message : '保存配置失败';
      setError(message);
      setSaveMessage(message);
    } finally {
      setSavingConfig(false);
    }
  };

  if (leftPanelCollapsed) {
    return (
      <div className="flex h-full flex-col items-center justify-between border-r border-stone-200 bg-[#efe9de] py-4 dark:border-stone-800 dark:bg-[#1c1c1a]">
        <div className="flex flex-col items-center gap-4">
          <div className="rounded-2xl bg-white/80 p-3 shadow-sm dark:bg-stone-900/80">
            <Settings className="h-5 w-5 text-emerald-600" />
          </div>
          <span className="text-xs uppercase tracking-[0.3em] text-stone-500 [writing-mode:vertical-rl] dark:text-stone-400">
            Setup
          </span>
        </div>
        <button
          onClick={toggleLeftPanel}
          className="rounded-xl border border-stone-200 p-2 text-stone-600 transition hover:bg-white dark:border-stone-700 dark:text-stone-300 dark:hover:bg-stone-900"
          title="展开设置栏"
        >
          <ChevronRight className="h-4 w-4" />
        </button>
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col border-r border-stone-200 bg-[#efe9de] dark:border-stone-800 dark:bg-[#1c1c1a]">
      <div className="border-b border-stone-200 p-4 dark:border-stone-800">
        <div className="flex items-center justify-between">
          <div>
            <div className="flex items-center gap-2">
              <Settings className="h-5 w-5 text-emerald-600" />
              <span className="font-semibold">工作区设置</span>
            </div>
            <p className="mt-1 text-xs text-stone-500 dark:text-stone-400">
              API、同步和界面偏好会保存在本机配置目录；密钥保存后只会显示“已配置”，不会回显原文。
            </p>
          </div>
          <button
            onClick={toggleLeftPanel}
            className="rounded-xl border border-stone-200 p-2 text-stone-600 transition hover:bg-white dark:border-stone-700 dark:text-stone-300 dark:hover:bg-stone-900"
            title="收起设置栏"
          >
            <ChevronLeft className="h-4 w-4" />
          </button>
        </div>
      </div>

      <div className="flex border-b border-stone-200 dark:border-stone-800">
        <button
          onClick={() => setActiveTab('api')}
          className={`flex-1 py-3 text-sm font-medium transition ${
            activeTab === 'api'
              ? 'border-b-2 border-emerald-600 text-emerald-700 dark:text-emerald-400'
              : 'text-stone-500 dark:text-stone-400'
          }`}
        >
          <Key className="mr-1 inline-block h-4 w-4" />
          API
        </button>
        <button
          onClick={() => setActiveTab('cloud')}
          className={`flex-1 py-3 text-sm font-medium transition ${
            activeTab === 'cloud'
              ? 'border-b-2 border-emerald-600 text-emerald-700 dark:text-emerald-400'
              : 'text-stone-500 dark:text-stone-400'
          }`}
        >
          <Cloud className="mr-1 inline-block h-4 w-4" />
          云同步
        </button>
        <button
          onClick={() => setActiveTab('ui')}
          className={`flex-1 py-3 text-sm font-medium transition ${
            activeTab === 'ui'
              ? 'border-b-2 border-emerald-600 text-emerald-700 dark:text-emerald-400'
              : 'text-stone-500 dark:text-stone-400'
          }`}
        >
          <Palette className="mr-1 inline-block h-4 w-4" />
          界面
        </button>
      </div>

      <div className="flex-1 space-y-4 overflow-y-auto p-4">
        {activeTab === 'api' && (
          <div className="space-y-4">
            <div className="rounded-2xl border border-stone-200 bg-white/80 p-4 dark:border-stone-800 dark:bg-stone-900/70">
              <div className="flex items-start gap-3">
                <Bot className="mt-0.5 h-5 w-5 text-emerald-600" />
                <div className="space-y-4">
                  <div>
                    <p className="text-sm font-medium">LLM Provider</p>
                    <p className="mt-1 text-xs leading-5 text-stone-500 dark:text-stone-400">
                      支持 OpenAI-compatible 与 Anthropic 两类接口。像 `duckcoding` 这类代理，通常选
                      OpenAI-compatible，再填自定义 `base URL` 与 `model`。
                    </p>
                  </div>

                  <div>
                    <label className="mb-1 block text-sm font-medium">Provider 类型</label>
                    <select
                      value={draftConfig.llm.providerType}
                      onChange={(event) =>
                        handleProviderTypeChange(
                          event.target.value as AppConfig['llm']['providerType']
                        )
                      }
                      className="w-full rounded-xl border border-stone-200 bg-white px-3 py-2 text-sm shadow-sm outline-none transition focus:border-emerald-500 dark:border-stone-700 dark:bg-stone-900"
                    >
                      <option value="openai_compatible">OpenAI-compatible</option>
                      <option value="anthropic">Anthropic Messages</option>
                    </select>
                  </div>

                  <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
                    <div>
                      <label className="mb-1 block text-sm font-medium">Provider ID</label>
                      <input
                        type="text"
                        value={draftConfig.llm.providerId}
                        onChange={(event) => updateLLM({ providerId: event.target.value })}
                        placeholder="例如 duckcoding"
                        className="w-full rounded-xl border border-stone-200 bg-white px-3 py-2 text-sm shadow-sm outline-none transition focus:border-emerald-500 dark:border-stone-700 dark:bg-stone-900"
                      />
                    </div>

                    <div>
                      <label className="mb-1 block text-sm font-medium">显示名称</label>
                      <input
                        type="text"
                        value={draftConfig.llm.providerName}
                        onChange={(event) => updateLLM({ providerName: event.target.value })}
                        placeholder="例如 DuckCoding"
                        className="w-full rounded-xl border border-stone-200 bg-white px-3 py-2 text-sm shadow-sm outline-none transition focus:border-emerald-500 dark:border-stone-700 dark:bg-stone-900"
                      />
                    </div>
                  </div>

                  <div>
                    <label className="mb-1 block text-sm font-medium">Base URL</label>
                    <div className="relative">
                      <Server className="pointer-events-none absolute left-3 top-2.5 h-4 w-4 text-stone-400" />
                      <input
                        type="text"
                        value={draftConfig.llm.baseUrl}
                        onChange={(event) => updateLLM({ baseUrl: event.target.value })}
                        placeholder="https://api.duckcoding.ai/v1"
                        className="w-full rounded-xl border border-stone-200 bg-white py-2 pl-10 pr-3 text-sm shadow-sm outline-none transition focus:border-emerald-500 dark:border-stone-700 dark:bg-stone-900"
                      />
                    </div>
                  </div>

                  <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
                    <div>
                      <label className="mb-1 block text-sm font-medium">Wire API</label>
                      <select
                        value={draftConfig.llm.wireApi}
                        onChange={(event) =>
                          updateLLM({
                            wireApi: event.target.value as AppConfig['llm']['wireApi'],
                          })
                        }
                        className="w-full rounded-xl border border-stone-200 bg-white px-3 py-2 text-sm shadow-sm outline-none transition focus:border-emerald-500 dark:border-stone-700 dark:bg-stone-900"
                      >
                        {draftConfig.llm.providerType === 'openai_compatible' ? (
                          <>
                            <option value="responses">Responses API</option>
                            <option value="chat_completions">Chat Completions</option>
                          </>
                        ) : (
                          <option value="anthropic_messages">Anthropic Messages</option>
                        )}
                      </select>
                    </div>

                    <div>
                      <label className="mb-1 block text-sm font-medium">模型名</label>
                      <input
                        type="text"
                        value={draftConfig.llm.model}
                        onChange={(event) => updateLLM({ model: event.target.value })}
                        placeholder="例如 gpt-5.3-codex"
                        className="w-full rounded-xl border border-stone-200 bg-white px-3 py-2 text-sm shadow-sm outline-none transition focus:border-emerald-500 dark:border-stone-700 dark:bg-stone-900"
                      />
                    </div>
                  </div>

                  <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
                    <div>
                      <label className="mb-1 block text-sm font-medium">Reasoning Effort</label>
                      <input
                        type="text"
                        value={draftConfig.llm.reasoningEffort}
                        onChange={(event) => updateLLM({ reasoningEffort: event.target.value })}
                        placeholder="例如 xhigh / high / medium"
                        className="w-full rounded-xl border border-stone-200 bg-white px-3 py-2 text-sm shadow-sm outline-none transition focus:border-emerald-500 dark:border-stone-700 dark:bg-stone-900"
                      />
                      <p className="mt-2 text-xs leading-5 text-stone-500 dark:text-stone-400">
                        仅在 `Responses API` 路径下发送；支持你直接输入自定义值，比如 `xhigh`。
                      </p>
                    </div>

                    <div className="space-y-3 rounded-2xl border border-stone-200 bg-white/70 px-4 py-3 dark:border-stone-800 dark:bg-stone-900/60">
                      {draftConfig.llm.providerType === 'openai_compatible' && (
                        <div className="flex items-center justify-between gap-3">
                          <div>
                            <p className="text-sm font-medium">OpenAI Bearer Auth</p>
                            <p className="text-xs leading-5 text-stone-500 dark:text-stone-400">
                              对应你配置里的 `requires_openai_auth = true`
                            </p>
                          </div>
                          <Switch.Root
                            checked={draftConfig.llm.requiresOpenAIAuth}
                            onCheckedChange={(checked) => updateLLM({ requiresOpenAIAuth: checked })}
                            className="relative h-6 w-11 rounded-full bg-stone-300 transition data-[state=checked]:bg-emerald-600 dark:bg-stone-700"
                          >
                            <Switch.Thumb className="block h-5 w-5 translate-x-0.5 rounded-full bg-white transition-transform data-[state=checked]:translate-x-5" />
                          </Switch.Root>
                        </div>
                      )}

                      <div className="flex items-center justify-between gap-3">
                        <div>
                          <p className="text-sm font-medium">禁用响应留存</p>
                          <p className="text-xs leading-5 text-stone-500 dark:text-stone-400">
                            开启后会尽量把请求按“不要存储响应”的方式发送。
                          </p>
                        </div>
                        <Switch.Root
                          checked={draftConfig.llm.disableResponseStorage}
                          onCheckedChange={(checked) => updateLLM({ disableResponseStorage: checked })}
                          className="relative h-6 w-11 rounded-full bg-stone-300 transition data-[state=checked]:bg-emerald-600 dark:bg-stone-700"
                        >
                          <Switch.Thumb className="block h-5 w-5 translate-x-0.5 rounded-full bg-white transition-transform data-[state=checked]:translate-x-5" />
                        </Switch.Root>
                      </div>
                    </div>
                  </div>

                  <div className="rounded-2xl border border-stone-200 bg-[#f8f5ef] p-4 dark:border-stone-800 dark:bg-stone-950/60">
                    <div className="flex items-start justify-between gap-3">
                      <div>
                        <div className="flex items-center gap-2">
                          <Shield className="h-4 w-4 text-emerald-600" />
                          <p className="text-sm font-medium">LLM API Key</p>
                        </div>
                        <p className="mt-1 text-xs leading-5 text-stone-500 dark:text-stone-400">
                          保存后不会回显原始内容；如果需要更换，直接贴入新值再保存。
                        </p>
                      </div>
                      {draftConfig.llm.hasApiKey && (
                        <span className="rounded-full bg-emerald-100 px-2.5 py-1 text-[11px] font-medium text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300">
                          已配置
                        </span>
                      )}
                    </div>
                    <div className="mt-3 flex gap-2">
                      <input
                        type="password"
                        value={draftConfig.llm.apiKey}
                        onChange={(event) =>
                          updateLLM({ apiKey: event.target.value, clearApiKey: false })
                        }
                        autoComplete="off"
                        spellCheck={false}
                        placeholder={
                          draftConfig.llm.hasApiKey ? '已保存，如需替换请直接粘贴新 key' : 'sk-...'
                        }
                        className="flex-1 rounded-xl border border-stone-200 bg-white px-3 py-2 text-sm shadow-sm outline-none transition focus:border-emerald-500 dark:border-stone-700 dark:bg-stone-900"
                      />
                      <button
                        type="button"
                        onClick={() => updateLLM({ apiKey: '', clearApiKey: true, hasApiKey: false })}
                        className="rounded-xl border border-stone-200 px-3 py-2 text-sm text-stone-600 transition hover:bg-white dark:border-stone-700 dark:text-stone-300 dark:hover:bg-stone-900"
                      >
                        <Trash2 className="h-4 w-4" />
                      </button>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <div className="rounded-2xl border border-stone-200 bg-white/80 p-4 dark:border-stone-800 dark:bg-stone-900/70">
              <div className="flex items-start justify-between gap-3">
                <div>
                  <p className="text-sm font-medium">Semantic Scholar API Key</p>
                  <p className="mt-1 text-xs leading-5 text-stone-500 dark:text-stone-400">
                    检索论文时可选，用于提升稳定性和限额。
                  </p>
                </div>
                {draftConfig.search.hasSemanticScholarApiKey && (
                  <span className="rounded-full bg-emerald-100 px-2.5 py-1 text-[11px] font-medium text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300">
                    已配置
                  </span>
                )}
              </div>
              <div className="mt-3 flex gap-2">
                <input
                  type="password"
                  value={draftConfig.search.semanticScholarApiKey}
                  onChange={(event) =>
                    updateDraft({
                      search: {
                        ...draftConfig.search,
                        semanticScholarApiKey: event.target.value,
                        clearSemanticScholarApiKey: false,
                      },
                    })
                  }
                  autoComplete="off"
                  spellCheck={false}
                  placeholder={
                    draftConfig.search.hasSemanticScholarApiKey
                      ? '已保存，如需替换请直接粘贴新 key'
                      : '可选'
                  }
                  className="flex-1 rounded-xl border border-stone-200 bg-white px-3 py-2 text-sm shadow-sm outline-none transition focus:border-emerald-500 dark:border-stone-700 dark:bg-stone-900"
                />
                <button
                  type="button"
                  onClick={() =>
                    updateDraft({
                      search: {
                        ...draftConfig.search,
                        semanticScholarApiKey: '',
                        clearSemanticScholarApiKey: true,
                        hasSemanticScholarApiKey: false,
                      },
                    })
                  }
                  className="rounded-xl border border-stone-200 px-3 py-2 text-sm text-stone-600 transition hover:bg-white dark:border-stone-700 dark:text-stone-300 dark:hover:bg-stone-900"
                >
                  <Trash2 className="h-4 w-4" />
                </button>
              </div>
            </div>
          </div>
        )}

        {activeTab === 'cloud' && (
          <div className="space-y-4">
            <div className="flex items-center justify-between rounded-2xl border border-stone-200 bg-white/70 px-4 py-3 dark:border-stone-800 dark:bg-stone-900/60">
              <span className="font-medium">启用百度网盘同步</span>
              <Switch.Root
                checked={draftConfig.baiduCloud.enabled}
                onCheckedChange={(checked) =>
                  updateDraft({
                    baiduCloud: { ...draftConfig.baiduCloud, enabled: checked },
                  })
                }
                className="relative h-6 w-11 rounded-full bg-stone-300 transition data-[state=checked]:bg-emerald-600 dark:bg-stone-700"
              >
                <Switch.Thumb className="block h-5 w-5 translate-x-0.5 rounded-full bg-white transition-transform data-[state=checked]:translate-x-5" />
              </Switch.Root>
            </div>

            {draftConfig.baiduCloud.enabled && (
              <div className="rounded-2xl border border-stone-200 bg-white/80 p-4 dark:border-stone-800 dark:bg-stone-900/70">
                <div className="flex items-start justify-between gap-3">
                  <div>
                    <p className="text-sm font-medium">百度网盘 Access Token</p>
                    <p className="mt-1 text-xs leading-5 text-stone-500 dark:text-stone-400">
                      当前只保存配置，不会自动执行同步。token 保存后不会回显。
                    </p>
                  </div>
                  {draftConfig.baiduCloud.hasToken && (
                    <span className="rounded-full bg-emerald-100 px-2.5 py-1 text-[11px] font-medium text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300">
                      已配置
                    </span>
                  )}
                </div>
                <div className="mt-3 flex gap-2">
                  <input
                    type="password"
                    value={draftConfig.baiduCloud.token}
                    onChange={(event) =>
                      updateDraft({
                        baiduCloud: {
                          ...draftConfig.baiduCloud,
                          token: event.target.value,
                          clearToken: false,
                        },
                      })
                    }
                    autoComplete="off"
                    spellCheck={false}
                    placeholder={
                      draftConfig.baiduCloud.hasToken
                        ? '已保存，如需替换请直接粘贴新 token'
                        : 'OAuth 获取的 token'
                    }
                    className="flex-1 rounded-xl border border-stone-200 bg-white px-3 py-2 text-sm shadow-sm outline-none transition focus:border-emerald-500 dark:border-stone-700 dark:bg-stone-900"
                  />
                  <button
                    type="button"
                    onClick={() =>
                      updateDraft({
                        baiduCloud: {
                          ...draftConfig.baiduCloud,
                          token: '',
                          clearToken: true,
                          hasToken: false,
                        },
                      })
                    }
                    className="rounded-xl border border-stone-200 px-3 py-2 text-sm text-stone-600 transition hover:bg-white dark:border-stone-700 dark:text-stone-300 dark:hover:bg-stone-900"
                  >
                    <Trash2 className="h-4 w-4" />
                  </button>
                </div>
              </div>
            )}

            <p className="text-xs leading-6 text-stone-500 dark:text-stone-400">
              本轮只保留配置入口，不会自动执行云同步。后续接入百度网盘 API 后再补完整流程。
            </p>
          </div>
        )}

        {activeTab === 'ui' && (
          <div className="space-y-4">
            <div className="flex items-center justify-between rounded-2xl border border-stone-200 bg-white/70 px-4 py-3 dark:border-stone-800 dark:bg-stone-900/60">
              <span className="font-medium">深色模式</span>
              <Switch.Root
                checked={draftConfig.theme === 'dark'}
                onCheckedChange={(checked) => updateDraft({ theme: checked ? 'dark' : 'light' })}
                className="relative h-6 w-11 rounded-full bg-stone-300 transition data-[state=checked]:bg-emerald-600 dark:bg-stone-700"
              >
                <Switch.Thumb className="block h-5 w-5 translate-x-0.5 rounded-full bg-white transition-transform data-[state=checked]:translate-x-5" />
              </Switch.Root>
            </div>

            <div>
              <label className="mb-1 block text-sm font-medium">数据存储路径</label>
              <input
                type="text"
                value={draftConfig.dataPath}
                onChange={(event) => updateDraft({ dataPath: event.target.value })}
                className="w-full rounded-xl border border-stone-200 bg-white px-3 py-2 text-sm shadow-sm outline-none transition focus:border-emerald-500 dark:border-stone-700 dark:bg-stone-900"
              />
              <p className="mt-2 text-xs leading-6 text-stone-500 dark:text-stone-400">
                修改后会写入配置文件；如果你更换了数据目录，新的数据库路径将在下次启动时生效。
              </p>
            </div>
          </div>
        )}
      </div>

      <div className="space-y-2 border-t border-stone-200 p-4 dark:border-stone-800">
        <button
          onClick={handleSave}
          disabled={isSavingConfig}
          className="flex w-full items-center justify-center gap-2 rounded-2xl bg-emerald-600 px-4 py-3 text-sm font-medium text-white transition hover:bg-emerald-700 disabled:cursor-not-allowed disabled:opacity-60"
        >
          <Save className="h-4 w-4" />
          {isSavingConfig ? '保存中...' : '保存配置'}
        </button>
        {saveMessage && (
          <p className="text-xs leading-5 text-stone-500 dark:text-stone-400">{saveMessage}</p>
        )}
      </div>
    </div>
  );
}
