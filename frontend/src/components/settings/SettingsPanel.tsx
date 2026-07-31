import { useEffect, useState } from 'react';
import * as Switch from '@radix-ui/react-switch';
import {
  Bot,
  Cloud,
  Eye,
  EyeOff,
  Gauge,
  Key,
  Save,
  Search,
  Server,
  Settings,
  Trash2,
} from 'lucide-react';
import { getLLMUsage, getSecretPrefill, saveConfig } from '../../lib/backend';
import { errorToUserMessage } from '../../lib/errors';
import { useAppStore } from '../../stores/appStore';
import type { AppConfig, ConfigSecretPrefill, LLMConfig, LLMUsageSnapshot } from '../../types';

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
      apiKey: config.llm.apiKey,
      hasApiKey: secretPrefill.hasStrongLLMApiKey || config.llm.hasApiKey,
    },
    weakLLM: {
      ...config.weakLLM,
      apiKey: config.weakLLM.apiKey,
      hasApiKey: secretPrefill.hasWeakLLMApiKey || config.weakLLM.hasApiKey,
    },
    baiduCloud: {
      ...config.baiduCloud,
      token: config.baiduCloud.token,
      hasToken: secretPrefill.hasBaiduToken || config.baiduCloud.hasToken,
    },
  };
}

function secretPrefillFromConfig(config: AppConfig): ConfigSecretPrefill {
  return {
    strongLLMApiKey: '',
    hasStrongLLMApiKey: config.llm.hasApiKey || config.llm.apiKey.trim().length > 0,
    weakLLMApiKey: '',
    hasWeakLLMApiKey: config.weakLLM.hasApiKey || config.weakLLM.apiKey.trim().length > 0,
    baiduToken: '',
    hasBaiduToken: config.baiduCloud.hasToken || config.baiduCloud.token.trim().length > 0,
  };
}

function SecretVisibilityButton({ visible, onToggle }: { visible: boolean; onToggle: () => void }) {
  return (
    <button
      type="button"
      onClick={onToggle}
      className="de-button-secondary p-2 text-[var(--de-ink-muted)]"
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
    <div className="de-panel p-4">
      <div className="flex items-start gap-3">
        <Bot className="mt-0.5 h-5 w-5 text-[var(--de-accent)]" />
        <div className="min-w-0 flex-1 space-y-4">
          <div>
            <p className="text-sm font-medium">{title}</p>
            <p className="mt-1 text-xs leading-5 text-[var(--de-ink-muted)]">{description}</p>
          </div>

          <div>
            <label className="mb-1 block text-sm font-medium">Provider 类型</label>
            <select
              value={value.providerType}
              onChange={(event) => onProviderTypeChange(event.target.value as 'openai_compatible' | 'anthropic')}
              className="de-field w-full px-3 py-2 text-sm"
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
              className="de-field w-full px-3 py-2 text-sm"
            />
          </div>

          <div>
            <label className="mb-1 block text-sm font-medium">显示名称</label>
            <input
              type="text"
              value={value.providerName}
              onChange={(event) => onChange({ providerName: event.target.value })}
              className="de-field w-full px-3 py-2 text-sm"
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
                className="de-field w-full py-2 pl-10 pr-3 text-sm"
              />
            </div>
          </div>

          <div>
            <label className="mb-1 block text-sm font-medium">Wire API</label>
            <select
              value={value.wireApi}
              onChange={(event) => onChange({ wireApi: event.target.value as LLMConfig['wireApi'] })}
              className="de-field w-full px-3 py-2 text-sm"
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
              className="de-field w-full px-3 py-2 text-sm"
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
                className="de-field w-full px-3 py-2 text-sm"
              />
            </div>
          )}

          <div className="space-y-3 border-y border-[var(--de-rule)] bg-[var(--de-surface-muted)] px-4 py-3">
            {value.providerType === 'openai_compatible' && (
              <div className="flex items-center justify-between gap-3">
                <div>
                  <p className="text-sm font-medium">OpenAI Bearer Auth</p>
                  <p className="text-xs leading-5 text-[var(--de-ink-muted)]">代理接口通常需要打开。</p>
                </div>
                <Switch.Root
                  checked={value.requiresOpenAIAuth}
                  onCheckedChange={(checked) => onChange({ requiresOpenAIAuth: checked })}
                  className="relative h-6 w-11 rounded-full bg-[var(--de-rule-strong)] transition data-[state=checked]:bg-[var(--de-accent)]"
                >
                  <Switch.Thumb className="block h-5 w-5 translate-x-0.5 rounded-full bg-white transition-transform data-[state=checked]:translate-x-5" />
                </Switch.Root>
              </div>
            )}

            <div className="flex items-center justify-between gap-3">
              <div>
                <p className="text-sm font-medium">禁用响应存储</p>
                <p className="text-xs leading-5 text-[var(--de-ink-muted)]">优先关闭三方平台的服务端持久化。</p>
              </div>
              <Switch.Root
                checked={value.disableResponseStorage}
                onCheckedChange={(checked) => onChange({ disableResponseStorage: checked })}
                className="relative h-6 w-11 rounded-full bg-[var(--de-rule-strong)] transition data-[state=checked]:bg-[var(--de-accent)]"
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
    <div className="de-panel p-4">
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="text-sm font-medium">{title}</p>
          <p className="mt-1 text-xs leading-5 text-[var(--de-ink-muted)]">{description}</p>
        </div>
        {configured && (
          <span className="rounded-[var(--de-radius)] border border-[var(--de-accent)] bg-[var(--de-accent-soft)] px-2.5 py-1 text-[11px] font-medium text-[var(--de-accent)]">
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
          className="de-field min-w-0 flex-1 px-3 py-2 text-sm"
        />
        <SecretVisibilityButton visible={visible} onToggle={onToggleVisible} />
        <button
          type="button"
          onClick={onClear}
          className="de-button-secondary p-2 text-[var(--de-danger)]"
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
    setConfig,
    setError,
    setSavingConfig,
  } = useAppStore();
  const [activeTab, setActiveTab] = useState<'workspace' | 'credentials' | 'cloud'>('workspace');
  const [secretPrefill, setSecretPrefill] = useState<ConfigSecretPrefill>(emptySecretPrefill);
  const [draftConfig, setDraftConfig] = useState<AppConfig>(() => mergeConfigWithSecretPrefill(config, emptySecretPrefill));
  const [saveMessage, setSaveMessage] = useState('');
  const [showStrongKey, setShowStrongKey] = useState(false);
  const [showWeakKey, setShowWeakKey] = useState(false);
  const [llmUsage, setLLMUsage] = useState<LLMUsageSnapshot | null>(null);

  useEffect(() => {
    setDraftConfig(mergeConfigWithSecretPrefill(config, secretPrefill));
  }, [config]);

  useEffect(() => {
    setDraftConfig((current) => mergeConfigWithSecretPrefill(current, secretPrefill));
  }, [secretPrefill]);

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

  useEffect(() => {
    let cancelled = false;
    void getLLMUsage()
      .then((usage) => {
        if (!cancelled) setLLMUsage(usage);
      })
      .catch(() => {
        if (!cancelled) setLLMUsage(null);
      });
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

  const handleSave = async () => {
    setSavingConfig(true);
    setSaveMessage('');
    try {
      const result = await saveConfig(draftConfig);
      setSecretPrefill(secretPrefillFromConfig(draftConfig));
      setConfig(result.config);
      setSaveMessage(result.restartRequired ? '配置已保存，数据路径改动将在重启后生效。' : '配置已保存。');
    } catch (error) {
      const message = errorToUserMessage(error, '保存配置失败');
      setError(message);
      setSaveMessage(message);
    } finally {
      setSavingConfig(false);
    }
  };

  return (
    <div className="flex h-full min-h-0 flex-col bg-[var(--de-surface)] text-[var(--de-ink)]">
      <div className="border-b border-[var(--de-rule)] px-6 py-5">
        <div className="flex items-center justify-between gap-3">
          <div>
            <div className="flex items-center gap-2">
              <Settings className="h-5 w-5 text-[var(--de-accent)]" />
              <h1 className="de-display text-xl font-semibold">工作区设置</h1>
            </div>
            <p className="mt-1 text-sm text-[var(--de-ink-muted)]">
              管理模型、研究数据和同步凭据。
            </p>
          </div>
        </div>
      </div>

      <div className="flex border-b border-[var(--de-rule)] px-6" role="tablist" aria-label="设置分类">
        {[
          { key: 'workspace', icon: <Settings className="mr-1 inline-block h-4 w-4" />, label: '软件' },
          { key: 'credentials', icon: <Key className="mr-1 inline-block h-4 w-4" />, label: '凭据' },
          { key: 'cloud', icon: <Cloud className="mr-1 inline-block h-4 w-4" />, label: '云同步' },
        ].map((item) => (
          <button
            key={item.key}
            type="button"
            role="tab"
            aria-selected={activeTab === item.key}
            onClick={() => setActiveTab(item.key as 'workspace' | 'credentials' | 'cloud')}
            className={`px-4 py-3 text-sm font-medium transition-colors ${
              activeTab === item.key
                ? 'border-b-2 border-[var(--de-accent)] text-[var(--de-ink)]'
                : 'border-b-2 border-transparent text-[var(--de-ink-muted)] hover:text-[var(--de-ink)]'
            }`}
          >
            {item.icon}
            {item.label}
          </button>
        ))}
      </div>

      <div className="flex-1 overflow-y-auto px-6 py-5">
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

            <div className="border-y border-[var(--de-rule)] py-4">
              <div className="flex items-start gap-3">
                <Gauge className="mt-0.5 h-5 w-5 text-[var(--de-accent)]" aria-hidden="true" />
                <div className="min-w-0 flex-1">
                  <label className="block text-sm font-medium" htmlFor="daily-llm-budget">
                    每日 LLM token 上限
                  </label>
                  <p className="mt-1 text-xs leading-5 text-[var(--de-ink-muted)]">
                    强模型与弱模型共享额度；达到上限后，新请求会在发送前停止。
                  </p>
                  <input
                    id="daily-llm-budget"
                    type="number"
                    min={1}
                    step={1_000_000}
                    value={draftConfig.dailyLLMTokenBudget}
                    onChange={(event) =>
                      updateDraft({
                        dailyLLMTokenBudget: Math.max(1, Number(event.target.value) || 1),
                      })
                    }
                    className="de-field mt-3 w-full px-3 py-2 text-sm"
                  />
                  <div className="mt-2 flex justify-between gap-3 text-xs text-[var(--de-ink-muted)]">
                    <span>今日已用 {llmUsage?.usedTokens.toLocaleString() ?? '读取中'}</span>
                    <span>剩余 {llmUsage?.remaining.toLocaleString() ?? '读取中'}</span>
                  </div>
                </div>
              </div>
            </div>

            <LLMProfileSection
              title="弱模型配置"
              description="用于更轻量、更频繁的调用场景。默认会从 `weak_llm.json` 预填，便于后续分层使用。"
              value={draftConfig.weakLLM}
              onChange={(patch) => updateLLMProfile('weakLLM', patch)}
              onProviderTypeChange={(providerType) =>
                setDraftConfig((current) => applyProviderPreset(current, 'weakLLM', providerType))
              }
            />

            <div className="de-panel p-4">
              <div className="flex items-start gap-3">
                <Search className="mt-0.5 h-5 w-5 text-[var(--de-accent)]" />
                <div className="space-y-3">
                  <div>
                    <p className="text-sm font-medium">论文搜索方案</p>
                    <p className="mt-1 text-xs leading-5 text-[var(--de-ink-muted)]">
                      OpenAlex、arXiv、OpenReview 与 DBLP 使用同一套查询、去重和排序流程。OpenAlex key 保存在本地忽略文件 `config/openalex.json`。
                    </p>
                  </div>
                  <div className="border-y border-[var(--de-rule)] bg-[var(--de-surface-muted)] px-4 py-3 text-sm leading-6 text-[var(--de-ink-muted)]">
                    任一来源失败时继续保留其他结果；DOI、arXiv ID、OpenReview forum 和题名信息用于跨来源合并。
                  </div>
                </div>
              </div>
            </div>

            <div className="de-panel p-4">
              <div className="space-y-4">
                <div className="flex items-center justify-between border-b border-[var(--de-rule)] bg-[var(--de-surface-muted)] px-4 py-3">
                  <span className="font-medium">深色模式</span>
                  <Switch.Root
                    checked={draftConfig.theme === 'dark'}
                    onCheckedChange={(checked) => updateDraft({ theme: checked ? 'dark' : 'light' })}
                    className="relative h-6 w-11 rounded-full bg-[var(--de-rule-strong)] transition data-[state=checked]:bg-[var(--de-accent)]"
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
                    className="de-field w-full px-3 py-2 text-sm"
                  />
                  <p className="mt-2 text-xs leading-6 text-[var(--de-ink-muted)]">
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
              description="对应 `strong_llm.json` 和强模型配置，适合更重的分析任务。出于安全考虑只显示是否已配置；输入新值会替换现有密钥。"
              configured={draftConfig.llm.hasApiKey}
              visible={showStrongKey}
              value={draftConfig.llm.apiKey}
              placeholder={draftConfig.llm.hasApiKey ? '已配置；留空保存会保留原密钥，输入新值则替换' : 'sk-...'}
              onToggleVisible={() => setShowStrongKey((current) => !current)}
              onChange={(value) => {
                updateLLMProfile('llm', { apiKey: value, clearApiKey: false, hasApiKey: value.trim().length > 0 });
              }}
              onClear={() => {
                updateLLMProfile('llm', { apiKey: '', clearApiKey: true, hasApiKey: false });
              }}
            />

            <SecretField
              title="弱模型 API Key"
              description="对应 `weak_llm.json` 和轻量模型配置，后续可用于频繁、低成本调用。出于安全考虑只显示是否已配置；输入新值会替换现有密钥。"
              configured={draftConfig.weakLLM.hasApiKey}
              visible={showWeakKey}
              value={draftConfig.weakLLM.apiKey}
              placeholder={draftConfig.weakLLM.hasApiKey ? '已配置；留空保存会保留原密钥，输入新值则替换' : 'sk-...'}
              onToggleVisible={() => setShowWeakKey((current) => !current)}
              onChange={(value) => {
                updateLLMProfile('weakLLM', { apiKey: value, clearApiKey: false, hasApiKey: value.trim().length > 0 });
              }}
              onClear={() => {
                updateLLMProfile('weakLLM', { apiKey: '', clearApiKey: true, hasApiKey: false });
              }}
            />
          </div>
        )}

        {activeTab === 'cloud' && (
          <div className="space-y-4">
            <div className="de-panel flex items-center justify-between px-4 py-3">
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
                className="relative h-6 w-11 rounded-full bg-[var(--de-rule-strong)] transition data-[state=checked]:bg-[var(--de-accent)]"
              >
                <Switch.Thumb className="block h-5 w-5 translate-x-0.5 rounded-full bg-white transition-transform data-[state=checked]:translate-x-5" />
              </Switch.Root>
            </div>

            {draftConfig.baiduCloud.enabled && (
              <div className="de-panel p-4 text-sm">
                <div className="flex flex-wrap items-center justify-between gap-3">
                  <div>
                    <p className="font-semibold">百度网盘凭据由 token 文件统一管理</p>
                    <p className="mt-1 text-xs leading-6 opacity-85">
                      当前版本固定读取项目根目录的 <code>baiduyun_token.json</code>。该文件应同时包含 access_token、refresh_token、client_id 和 client_secret；设置页不再允许只保存单独的 access token，避免后续无法刷新。
                    </p>
                  </div>
                  <span className={`rounded-[var(--de-radius)] border px-2.5 py-1 text-xs font-medium ${
                    draftConfig.baiduCloud.hasToken
                      ? 'border-[var(--de-accent)] bg-[var(--de-accent-soft)] text-[var(--de-accent)]'
                      : 'border-[var(--de-rule-strong)] bg-[var(--de-surface-muted)] text-[var(--de-warning)]'
                  }`}
                  >
                    {draftConfig.baiduCloud.hasToken ? 'Token file configured' : 'Token file missing'}
                  </span>
                </div>
                <div className="mt-3 border-y border-[var(--de-rule)] bg-[var(--de-surface-muted)] px-3 py-2 text-xs leading-6 text-[var(--de-ink-muted)]">
                  <div>Token 文件：baiduyun_token.json</div>
                  <div>刷新 access token：请前往同步页面点击“更新凭证”。</div>
                  <div>如果你更新了 token 文件，请重启 DiveEnd 或重新保存配置以刷新运行时同步管理器。</div>
                </div>
              </div>
            )}

            <div className="border-y border-[var(--de-rule)] bg-[var(--de-surface-muted)] p-4 text-xs leading-6 text-[var(--de-ink-muted)]">
              启用百度网盘同步后，可以在 Sync 页面查看真实同步状态、先执行同步预检、手动触发同步、刷新 access token，并分别配置启动同步、周期同步、退出前同步和冲突策略。
            </div>
          </div>
        )}
      </div>

      <div className="border-t border-[var(--de-rule)] px-6 py-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="text-xs text-[var(--de-ink-muted)]">{saveMessage || '修改会保存到本机。'}</div>
          <button
            onClick={() => void handleSave()}
            disabled={isSavingConfig}
            className="inline-flex items-center gap-2 rounded-[5px] bg-[var(--de-accent)] px-4 py-2.5 text-sm font-medium text-white transition-colors hover:brightness-95 disabled:cursor-not-allowed disabled:opacity-60"
          >
            <Save className="h-4 w-4" />
            {isSavingConfig ? '保存中...' : '保存配置'}
          </button>
        </div>
      </div>
    </div>
  );
}
