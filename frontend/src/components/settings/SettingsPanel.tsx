import { useEffect, useState } from 'react';
import * as Switch from '@radix-ui/react-switch';
import {
  Bot,
  ChevronLeft,
  ChevronRight,
  Cloud,
  Eye,
  EyeOff,
  Key,
  Save,
  Search,
  Server,
  Settings,
  Trash2,
} from 'lucide-react';
import { getSecretPrefill, saveConfig } from '../../lib/backend';
import { useAppStore } from '../../stores/appStore';
import type { AppConfig, ConfigSecretPrefill, LLMConfig } from '../../types';

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

const emptySecretPrefill: ConfigSecretPrefill = {
  strongLLMApiKey: '',
  hasStrongLLMApiKey: false,
  weakLLMApiKey: '',
  hasWeakLLMApiKey: false,
  baiduToken: '',
  hasBaiduToken: false,
};

function applyProviderPreset(current: AppConfig, key: 'llm' | 'weakLLM', providerType: 'openai_compatible' | 'anthropic'): AppConfig {
  const preset = providerPresets[providerType];
  const existing = current[key];

  return {
    ...current,
    [key]: {
      ...existing,
      ...preset,
      providerType,
      apiKey: existing.apiKey,
      clearApiKey: false,
      hasApiKey: existing.hasApiKey || existing.apiKey.trim().length > 0,
      reasoningEffort: providerType === 'openai_compatible' ? existing.reasoningEffort : '',
    },
  } as AppConfig;
}

function mergeConfigWithSecretPrefill(config: AppConfig, secretPrefill: ConfigSecretPrefill): AppConfig {
  return {
    ...config,
    llm: {
      ...config.llm,
      apiKey: secretPrefill.strongLLMApiKey || config.llm.apiKey,
      hasApiKey: secretPrefill.hasStrongLLMApiKey || config.llm.hasApiKey,
    },
    weakLLM: {
      ...config.weakLLM,
      apiKey: secretPrefill.weakLLMApiKey || config.weakLLM.apiKey,
      hasApiKey: secretPrefill.hasWeakLLMApiKey || config.weakLLM.hasApiKey,
    },
    baiduCloud: {
      ...config.baiduCloud,
      token: secretPrefill.baiduToken || config.baiduCloud.token,
      hasToken: secretPrefill.hasBaiduToken || config.baiduCloud.hasToken,
    },
  };
}

function secretPrefillFromConfig(config: AppConfig): ConfigSecretPrefill {
  return {
    strongLLMApiKey: config.llm.apiKey,
    hasStrongLLMApiKey: config.llm.hasApiKey || config.llm.apiKey.trim().length > 0,
    weakLLMApiKey: config.weakLLM.apiKey,
    hasWeakLLMApiKey: config.weakLLM.hasApiKey || config.weakLLM.apiKey.trim().length > 0,
    baiduToken: config.baiduCloud.token,
    hasBaiduToken: config.baiduCloud.hasToken || config.baiduCloud.token.trim().length > 0,
  };
}

function SecretVisibilityButton({ visible, onToggle }: { visible: boolean; onToggle: () => void }) {
  return (
    <button
      type="button"
      onClick={onToggle}
      className="rounded-xl border border-stone-200 px-3 py-2 text-sm text-stone-600 transition hover:bg-white dark:border-stone-700 dark:text-stone-300 dark:hover:bg-stone-900"
      title={visible ? '隐藏内容' : '显示内容'}
    >
      {visible ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
    </button>
  );
}

function LLMProfileSection({
  title,
  description,
  value,
  onChange,
  onProviderTypeChange,
}: {
  title: string;
  description: string;
  value: LLMConfig;
  onChange: (patch: Partial<LLMConfig>) => void;
  onProviderTypeChange: (providerType: 'openai_compatible' | 'anthropic') => void;
}) {
  return (
    <div className="rounded-2xl border border-stone-200 bg-white/80 p-4 dark:border-stone-800 dark:bg-stone-900/70">
      <div className="flex items-start gap-3">
        <Bot className="mt-0.5 h-5 w-5 text-emerald-600" />
        <div className="min-w-0 flex-1 space-y-4">
          <div>
            <p className="text-sm font-medium">{title}</p>
            <p className="mt-1 text-xs leading-5 text-stone-500 dark:text-stone-400">{description}</p>
          </div>

          <div>
            <label className="mb-1 block text-sm font-medium">Provider 类型</label>
            <select
              value={value.providerType}
              onChange={(event) => onProviderTypeChange(event.target.value as 'openai_compatible' | 'anthropic')}
              className="w-full rounded-xl border border-stone-200 bg-white px-3 py-2 text-sm shadow-sm outline-none transition focus:border-emerald-500 dark:border-stone-700 dark:bg-stone-900"
            >
              <option value="openai_compatible">OpenAI-compatible</option>
              <option value="anthropic">Anthropic Messages</option>
            </select>
          </div>

          <div>
            <label className="mb-1 block text-sm font-medium">Provider ID</label>
            <input
              type="text"
              value={value.providerId}
              onChange={(event) => onChange({ providerId: event.target.value })}
              className="w-full rounded-xl border border-stone-200 bg-white px-3 py-2 text-sm shadow-sm outline-none transition focus:border-emerald-500 dark:border-stone-700 dark:bg-stone-900"
            />
          </div>

          <div>
            <label className="mb-1 block text-sm font-medium">显示名称</label>
            <input
              type="text"
              value={value.providerName}
              onChange={(event) => onChange({ providerName: event.target.value })}
              className="w-full rounded-xl border border-stone-200 bg-white px-3 py-2 text-sm shadow-sm outline-none transition focus:border-emerald-500 dark:border-stone-700 dark:bg-stone-900"
            />
          </div>

          <div>
            <label className="mb-1 block text-sm font-medium">Base URL</label>
            <div className="relative">
              <Server className="pointer-events-none absolute left-3 top-2.5 h-4 w-4 text-stone-400" />
              <input
                type="text"
                value={value.baseUrl}
                onChange={(event) => onChange({ baseUrl: event.target.value })}
                className="w-full rounded-xl border border-stone-200 bg-white py-2 pl-10 pr-3 text-sm shadow-sm outline-none transition focus:border-emerald-500 dark:border-stone-700 dark:bg-stone-900"
              />
            </div>
          </div>

          <div>
            <label className="mb-1 block text-sm font-medium">Wire API</label>
            <select
              value={value.wireApi}
              onChange={(event) => onChange({ wireApi: event.target.value as LLMConfig['wireApi'] })}
              className="w-full rounded-xl border border-stone-200 bg-white px-3 py-2 text-sm shadow-sm outline-none transition focus:border-emerald-500 dark:border-stone-700 dark:bg-stone-900"
            >
              {value.providerType === 'openai_compatible' ? (
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
              value={value.model}
              onChange={(event) => onChange({ model: event.target.value })}
              className="w-full rounded-xl border border-stone-200 bg-white px-3 py-2 text-sm shadow-sm outline-none transition focus:border-emerald-500 dark:border-stone-700 dark:bg-stone-900"
            />
          </div>

          {value.providerType === 'openai_compatible' && (
            <div>
              <label className="mb-1 block text-sm font-medium">Reasoning Effort</label>
              <input
                type="text"
                value={value.reasoningEffort}
                onChange={(event) => onChange({ reasoningEffort: event.target.value })}
                placeholder="例如 low / medium / high / xhigh"
                className="w-full rounded-xl border border-stone-200 bg-white px-3 py-2 text-sm shadow-sm outline-none transition focus:border-emerald-500 dark:border-stone-700 dark:bg-stone-900"
              />
            </div>
          )}

          <div className="space-y-3 rounded-2xl border border-stone-200 bg-stone-50/80 px-4 py-3 dark:border-stone-800 dark:bg-stone-950/60">
            {value.providerType === 'openai_compatible' && (
              <div className="flex items-center justify-between gap-3">
                <div>
                  <p className="text-sm font-medium">OpenAI Bearer Auth</p>
                  <p className="text-xs leading-5 text-stone-500 dark:text-stone-400">代理接口通常需要打开。</p>
                </div>
                <Switch.Root
                  checked={value.requiresOpenAIAuth}
                  onCheckedChange={(checked) => onChange({ requiresOpenAIAuth: checked })}
                  className="relative h-6 w-11 rounded-full bg-stone-300 transition data-[state=checked]:bg-emerald-600 dark:bg-stone-700"
                >
                  <Switch.Thumb className="block h-5 w-5 translate-x-0.5 rounded-full bg-white transition-transform data-[state=checked]:translate-x-5" />
                </Switch.Root>
              </div>
            )}

            <div className="flex items-center justify-between gap-3">
              <div>
                <p className="text-sm font-medium">禁用响应存储</p>
                <p className="text-xs leading-5 text-stone-500 dark:text-stone-400">优先关闭三方平台的服务端持久化。</p>
              </div>
              <Switch.Root
                checked={value.disableResponseStorage}
                onCheckedChange={(checked) => onChange({ disableResponseStorage: checked })}
                className="relative h-6 w-11 rounded-full bg-stone-300 transition data-[state=checked]:bg-emerald-600 dark:bg-stone-700"
              >
                <Switch.Thumb className="block h-5 w-5 translate-x-0.5 rounded-full bg-white transition-transform data-[state=checked]:translate-x-5" />
              </Switch.Root>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

function SecretField({
  title,
  description,
  configured,
  visible,
  value,
  placeholder,
  onToggleVisible,
  onChange,
  onClear,
}: {
  title: string;
  description: string;
  configured: boolean;
  visible: boolean;
  value: string;
  placeholder: string;
  onToggleVisible: () => void;
  onChange: (value: string) => void;
  onClear: () => void;
}) {
  return (
    <div className="rounded-2xl border border-stone-200 bg-white/80 p-4 dark:border-stone-800 dark:bg-stone-900/70">
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="text-sm font-medium">{title}</p>
          <p className="mt-1 text-xs leading-5 text-stone-500 dark:text-stone-400">{description}</p>
        </div>
        {configured && (
          <span className="rounded-full bg-emerald-100 px-2.5 py-1 text-[11px] font-medium text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300">
            已配置
          </span>
        )}
      </div>

      <div className="mt-3 flex flex-wrap gap-2">
        <input
          type={visible ? 'text' : 'password'}
          value={value}
          onChange={(event) => onChange(event.target.value)}
          autoComplete="off"
          spellCheck={false}
          placeholder={placeholder}
          className="min-w-0 flex-1 rounded-xl border border-stone-200 bg-white px-3 py-2 text-sm shadow-sm outline-none transition focus:border-emerald-500 dark:border-stone-700 dark:bg-stone-900"
        />
        <SecretVisibilityButton visible={visible} onToggle={onToggleVisible} />
        <button
          type="button"
          onClick={onClear}
          className="rounded-xl border border-stone-200 px-3 py-2 text-sm text-stone-600 transition hover:bg-white dark:border-stone-700 dark:text-stone-300 dark:hover:bg-stone-900"
        >
          <Trash2 className="h-4 w-4" />
        </button>
      </div>
    </div>
  );
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
  const [activeTab, setActiveTab] = useState<'workspace' | 'credentials' | 'cloud'>('workspace');
  const [secretPrefill, setSecretPrefill] = useState<ConfigSecretPrefill>(emptySecretPrefill);
  const [draftConfig, setDraftConfig] = useState<AppConfig>(() => mergeConfigWithSecretPrefill(config, emptySecretPrefill));
  const [saveMessage, setSaveMessage] = useState('');
  const [showStrongKey, setShowStrongKey] = useState(false);
  const [showWeakKey, setShowWeakKey] = useState(false);
  const [showBaiduToken, setShowBaiduToken] = useState(false);

  useEffect(() => {
    setDraftConfig(mergeConfigWithSecretPrefill(config, secretPrefill));
  }, [config, secretPrefill]);

  useEffect(() => {
    let cancelled = false;

    const loadPrefill = async () => {
      try {
        const prefill = await getSecretPrefill();
        if (!cancelled) {
          setSecretPrefill(prefill);
        }
      } catch {
        if (!cancelled) {
          setSecretPrefill(emptySecretPrefill);
        }
      }
    };

    void loadPrefill();
    return () => {
      cancelled = true;
    };
  }, []);

  const updateDraft = (patch: Partial<AppConfig>) => {
    setDraftConfig((current) => ({ ...current, ...patch }));
  };

  const updateLLMProfile = (key: 'llm' | 'weakLLM', patch: Partial<LLMConfig>) => {
    setDraftConfig((current) => ({
      ...current,
      [key]: {
        ...current[key],
        ...patch,
      },
    } as AppConfig));
  };

  const updateSecretPrefill = (patch: Partial<ConfigSecretPrefill>) => {
    setSecretPrefill((current) => ({ ...current, ...patch }));
  };

  const handleSave = async () => {
    setSavingConfig(true);
    setSaveMessage('');
    try {
      const result = await saveConfig(draftConfig);
      setSecretPrefill(secretPrefillFromConfig(draftConfig));
      setConfig(result.config);
      setSaveMessage(result.restartRequired ? '配置已保存，数据路径改动将在重启后生效。' : '配置已保存。');
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
    <div className="flex h-full min-h-0 flex-col border-r border-stone-200 bg-[#efe9de] dark:border-stone-800 dark:bg-[#1c1c1a]">
      <div className="border-b border-stone-200 p-4 dark:border-stone-800">
        <div className="flex items-center justify-between gap-3">
          <div>
            <div className="flex items-center gap-2">
              <Settings className="h-5 w-5 text-emerald-600" />
              <span className="font-semibold">工作区设置</span>
            </div>
            <p className="mt-1 text-xs text-stone-500 dark:text-stone-400">
              默认先展示非敏感的软件配置；强弱模型密钥会自动预填充，但初始保持隐藏状态。
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
        {[
          { key: 'workspace', icon: <Settings className="mr-1 inline-block h-4 w-4" />, label: '软件' },
          { key: 'credentials', icon: <Key className="mr-1 inline-block h-4 w-4" />, label: '凭据' },
          { key: 'cloud', icon: <Cloud className="mr-1 inline-block h-4 w-4" />, label: '云同步' },
        ].map((item) => (
          <button
            key={item.key}
            onClick={() => setActiveTab(item.key as 'workspace' | 'credentials' | 'cloud')}
            className={`flex-1 py-3 text-sm font-medium transition ${
              activeTab === item.key
                ? 'border-b-2 border-emerald-600 text-emerald-700 dark:text-emerald-400'
                : 'text-stone-500 dark:text-stone-400'
            }`}
          >
            {item.icon}
            {item.label}
          </button>
        ))}
      </div>

      <div className="flex-1 space-y-4 overflow-y-auto p-4">
        {activeTab === 'workspace' && (
          <div className="space-y-4">
            <LLMProfileSection
              title="强模型配置"
              description="用于 DeepStart 的结构化分析和更高质量的长上下文推理。当前保留完整的 provider / model 配置能力。"
              value={draftConfig.llm}
              onChange={(patch) => updateLLMProfile('llm', patch)}
              onProviderTypeChange={(providerType) =>
                setDraftConfig((current) => applyProviderPreset(current, 'llm', providerType))
              }
            />

            <LLMProfileSection
              title="弱模型配置"
              description="用于更轻量、更频繁的调用场景。默认会从 `weak_llm.json` 预填，便于后续分层使用。"
              value={draftConfig.weakLLM}
              onChange={(patch) => updateLLMProfile('weakLLM', patch)}
              onProviderTypeChange={(providerType) =>
                setDraftConfig((current) => applyProviderPreset(current, 'weakLLM', providerType))
              }
            />

            <div className="rounded-2xl border border-stone-200 bg-white/80 p-4 dark:border-stone-800 dark:bg-stone-900/70">
              <div className="flex items-start gap-3">
                <Search className="mt-0.5 h-5 w-5 text-emerald-600" />
                <div className="space-y-3">
                  <div>
                    <p className="text-sm font-medium">论文搜索方案</p>
                    <p className="mt-1 text-xs leading-5 text-stone-500 dark:text-stone-400">
                      当前后端固定并行检索 `Semantic Scholar + arXiv` 后去重汇总，未暴露搜索源切换，也不再要求你额外准备 Semantic Scholar key。
                    </p>
                  </div>
                  <div className="rounded-2xl border border-stone-200 bg-stone-50/80 px-4 py-3 text-sm leading-6 text-stone-600 dark:border-stone-800 dark:bg-stone-950/60 dark:text-stone-300">
                    现在的方案偏向“能直接搜索就先用”，而不是先堆配置；等搜索策略真正拆成可配置能力后，再单独加搜索源设置。
                  </div>
                </div>
              </div>
            </div>

            <div className="rounded-2xl border border-stone-200 bg-white/80 p-4 dark:border-stone-800 dark:bg-stone-900/70">
              <div className="space-y-4">
                <div className="flex items-center justify-between rounded-2xl border border-stone-200 bg-stone-50/80 px-4 py-3 dark:border-stone-800 dark:bg-stone-950/60">
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
                    修改后会写入配置文件；如果更换了数据目录，新的数据库路径将在下次启动时生效。
                  </p>
                </div>
              </div>
            </div>
          </div>
        )}

        {activeTab === 'credentials' && (
          <div className="space-y-4">
            <SecretField
              title="强模型 API Key"
              description="对应 `strong_llm.json` 和强模型配置，适合更重的分析任务。会自动预填充，但默认隐藏。"
              configured={draftConfig.llm.hasApiKey}
              visible={showStrongKey}
              value={draftConfig.llm.apiKey}
              placeholder={draftConfig.llm.hasApiKey ? '已填充，可直接修改后保存' : 'sk-...'}
              onToggleVisible={() => setShowStrongKey((current) => !current)}
              onChange={(value) => {
                updateLLMProfile('llm', { apiKey: value, clearApiKey: false, hasApiKey: value.trim().length > 0 });
                updateSecretPrefill({ strongLLMApiKey: value, hasStrongLLMApiKey: value.trim().length > 0 });
              }}
              onClear={() => {
                updateLLMProfile('llm', { apiKey: '', clearApiKey: true, hasApiKey: false });
                updateSecretPrefill({ strongLLMApiKey: '', hasStrongLLMApiKey: false });
              }}
            />

            <SecretField
              title="弱模型 API Key"
              description="对应 `weak_llm.json` 和轻量模型配置，后续可用于频繁、低成本调用。会自动预填充，但默认隐藏。"
              configured={draftConfig.weakLLM.hasApiKey}
              visible={showWeakKey}
              value={draftConfig.weakLLM.apiKey}
              placeholder={draftConfig.weakLLM.hasApiKey ? '已填充，可直接修改后保存' : 'sk-...'}
              onToggleVisible={() => setShowWeakKey((current) => !current)}
              onChange={(value) => {
                updateLLMProfile('weakLLM', { apiKey: value, clearApiKey: false, hasApiKey: value.trim().length > 0 });
                updateSecretPrefill({ weakLLMApiKey: value, hasWeakLLMApiKey: value.trim().length > 0 });
              }}
              onClear={() => {
                updateLLMProfile('weakLLM', { apiKey: '', clearApiKey: true, hasApiKey: false });
                updateSecretPrefill({ weakLLMApiKey: '', hasWeakLLMApiKey: false });
              }}
            />
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
                    baiduCloud: {
                      ...draftConfig.baiduCloud,
                      enabled: checked,
                    },
                  })
                }
                className="relative h-6 w-11 rounded-full bg-stone-300 transition data-[state=checked]:bg-emerald-600 dark:bg-stone-700"
              >
                <Switch.Thumb className="block h-5 w-5 translate-x-0.5 rounded-full bg-white transition-transform data-[state=checked]:translate-x-5" />
              </Switch.Root>
            </div>

            {draftConfig.baiduCloud.enabled && (
              <SecretField
                title="百度网盘 Access Token"
                description="默认会从本地 token 文件预填充，并以隐藏状态展示；当前先保留配置入口，不自动执行云同步。"
                configured={draftConfig.baiduCloud.hasToken}
                visible={showBaiduToken}
                value={draftConfig.baiduCloud.token}
                placeholder={draftConfig.baiduCloud.hasToken ? '已填充，可直接修改后保存' : 'OAuth 获取的 token'}
                onToggleVisible={() => setShowBaiduToken((current) => !current)}
                onChange={(value) => {
                  updateDraft({
                    baiduCloud: {
                      ...draftConfig.baiduCloud,
                      token: value,
                      clearToken: false,
                      hasToken: value.trim().length > 0,
                    },
                  });
                  updateSecretPrefill({ baiduToken: value, hasBaiduToken: value.trim().length > 0 });
                }}
                onClear={() => {
                  updateDraft({
                    baiduCloud: {
                      ...draftConfig.baiduCloud,
                      token: '',
                      clearToken: true,
                      hasToken: false,
                    },
                  });
                  updateSecretPrefill({ baiduToken: '', hasBaiduToken: false });
                }}
              />
            )}

            <div className="rounded-2xl border border-stone-200 bg-white/80 p-4 text-xs leading-6 text-stone-500 dark:border-stone-800 dark:bg-stone-900/70 dark:text-stone-400">
              当前项目的 Sync 页面仍处于过渡阶段：这里只保存云同步所需凭据，不会自动发起真实同步任务。
            </div>
          </div>
        )}
      </div>

      <div className="border-t border-stone-200 p-4 dark:border-stone-800">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="text-xs text-stone-500 dark:text-stone-400">{saveMessage || '修改后会持久化到本机配置文件。'}</div>
          <button
            onClick={() => void handleSave()}
            disabled={isSavingConfig}
            className="inline-flex items-center gap-2 rounded-2xl bg-stone-900 px-4 py-2.5 text-sm font-medium text-white transition hover:bg-stone-700 disabled:cursor-not-allowed disabled:opacity-60 dark:bg-stone-100 dark:text-stone-900 dark:hover:bg-white"
          >
            <Save className="h-4 w-4" />
            {isSavingConfig ? '保存中...' : '保存配置'}
          </button>
        </div>
      </div>
    </div>
  );
}
