import type {
  AppConfig,
  ConfigSecretPrefill,
  DeepStartAnalysis,
  DeepStartSessionDetail,
  DeepStartSessionSummary,
  EnhancedSearchResult,
  ExtractProgress,
  Folder,
  InitialState,
  Paper,
  SaveConfigResult,
  ScreeningDecisionNode,
  ScreeningSession,
  ScreeningSessionDetail,
  SearchPaper,
  SyncConflict,
  SyncProgress,
  SyncRecord,
  SyncStatus,
  TranslationRecord,
} from '../types';
import { defaultConfig, defaultInitialState } from '../types';
import { CanResolveFilePaths, EventsOff, EventsOn, ResolveFilePaths } from '../../wailsjs/runtime/runtime';

declare const __DIVEEND_ROOT__: string;

declare global {
  interface Window {
    go?: {
      main?: {
        App?: {
          GetInitialState(): Promise<InitialState>;
          GetSecretPrefill(): Promise<ConfigSecretPrefill>;
          SaveConfig(config: AppConfig): Promise<SaveConfigResult>;
          SearchPapers(query: string, limit: number): Promise<SearchPaper[]>;
          EnhancedSearchPapers(
            query: string,
            limit: number,
            offset: number,
            yearStart: number,
            yearEnd: number,
            sortBy: string
          ): Promise<EnhancedSearchResult>;
          ListDeepStartSessions(): Promise<DeepStartSessionSummary[]>;
          GetDeepStartSession(sessionId: string): Promise<DeepStartSessionDetail>;
          StartDeepStartSession(prompt: string, targetFolderId: string): Promise<DeepStartSessionDetail>;
          ReplyDeepStartSession(sessionId: string, message: string): Promise<DeepStartSessionDetail>;
          RerunDeepStartSearch(sessionId: string, query: string): Promise<DeepStartSessionDetail>;
          UpdateDeepStartSelections(
            sessionId: string,
            selectedPaperIds: string[],
            targetFolderId: string
          ): Promise<DeepStartSessionDetail>;
          GetFolders(): Promise<Folder[]>;
          CreateFolder(name: string): Promise<Folder>;
          GetPapers(folderId: string): Promise<Paper[]>;
          ImportPapers(folderId: string, papers: SearchPaper[]): Promise<Paper[]>;
          DeletePaper(id: string): Promise<void>;
          TranslatePaperSection(
            paperId: string,
            section: string,
            text: string
          ): Promise<TranslationRecord>;
          GetTranslations(paperId: string): Promise<TranslationRecord[]>;

          // Screening API
          CreateScreeningSession(title: string): Promise<ScreeningSession>;
          UploadScreeningFiles(sessionId: string, filePaths: string[]): Promise<ScreeningSessionDetail>;
          ExtractPaperContent(sessionId: string): Promise<ExtractProgress>;
          GetExtractProgress(sessionId: string): Promise<ExtractProgress>;
          AnalyzePapers(sessionId: string): Promise<ScreeningDecisionNode>;
          ApplyScreeningChoice(sessionId: string, selectedOptions: string[]): Promise<ScreeningDecisionNode>;
          CompleteScreening(sessionId: string, targetFolderId: string): Promise<Paper[]>;
          ListScreeningSessions(): Promise<ScreeningSession[]>;
          GetScreeningSession(sessionId: string): Promise<ScreeningSessionDetail>;
          CancelScreening(sessionId: string): Promise<void>;

          // Sync API
          GetSyncStatus(): Promise<SyncStatus>;
          TriggerSync(): Promise<SyncProgress>;
          GetSyncProgress(): Promise<SyncProgress>;
          GetSyncConflicts(): Promise<SyncConflict[]>;
          GetSyncRecords(limit: number): Promise<SyncRecord[]>;
          ResolveSyncConflict(conflictId: string, resolution: 'local' | 'remote'): Promise<void>;
        };
      };
    };
    runtime?: {
      EventsOn(eventName: string, callback: (...args: any[]) => void): () => void;
      EventsOff(eventName: string, ...args: string[]): void;
      CanResolveFilePaths(): boolean;
      ResolveFilePaths(files: File[]): string[];
    };
  }
}

const runtimeApp = () => window.go?.main?.App;
const hasWailsRuntime = () => Boolean(window.go?.main?.App && window.runtime);

const emptySecretPrefill: ConfigSecretPrefill = {
  strongLLMApiKey: '',
  hasStrongLLMApiKey: false,
  weakLLMApiKey: '',
  hasWeakLLMApiKey: false,
  baiduToken: '',
  hasBaiduToken: false,
};

type MockLLMSeed = {
  provider?: string;
  model?: string;
  api_key?: string;
  base_url?: string;
};

type MockBaiduSeed = {
  access_token?: string;
};

function normalizeArray<T>(value: T[] | null | undefined): T[] {
  return Array.isArray(value) ? value : [];
}

function normalizeAnalysis(analysis: DeepStartAnalysis | null | undefined): DeepStartAnalysis | null {
  if (!analysis) {
    return null;
  }

  return {
    ...analysis,
    directions: normalizeArray(analysis.directions),
    paperNotes: normalizeArray(analysis.paperNotes),
    followUpQuestions: normalizeArray(analysis.followUpQuestions),
    suggestedQueries: normalizeArray(analysis.suggestedQueries),
    recommendedPaperIds: normalizeArray(analysis.recommendedPaperIds),
  };
}

function normalizeSessionDetail(
  detail: DeepStartSessionDetail | null | undefined,
): DeepStartSessionDetail | null {
  if (!detail) {
    return null;
  }

  return {
    ...detail,
    messages: normalizeArray(detail.messages),
    currentResults: normalizeArray(detail.currentResults),
    currentAnalysis: normalizeAnalysis(detail.currentAnalysis),
    selectedPaperIds: normalizeArray(detail.selectedPaperIds),
  };
}

function normalizeConfig(config: Partial<AppConfig> | null | undefined): AppConfig {
  return {
    ...defaultConfig,
    ...config,
    llm: {
      ...defaultConfig.llm,
      ...config?.llm,
    },
    weakLLM: {
      ...defaultConfig.weakLLM,
      ...config?.weakLLM,
    },
    search: {
      ...defaultConfig.search,
      ...config?.search,
    },
    baiduCloud: {
      ...defaultConfig.baiduCloud,
      ...config?.baiduCloud,
    },
  };
}

function normalizeSecretPrefill(
  prefill: Partial<ConfigSecretPrefill> | null | undefined,
): ConfigSecretPrefill {
  return {
    strongLLMApiKey: prefill?.strongLLMApiKey?.trim() ?? '',
    hasStrongLLMApiKey: Boolean(prefill?.hasStrongLLMApiKey || prefill?.strongLLMApiKey?.trim()),
    weakLLMApiKey: prefill?.weakLLMApiKey?.trim() ?? '',
    hasWeakLLMApiKey: Boolean(prefill?.hasWeakLLMApiKey || prefill?.weakLLMApiKey?.trim()),
    baiduToken: prefill?.baiduToken?.trim() ?? '',
    hasBaiduToken: Boolean(prefill?.hasBaiduToken || prefill?.baiduToken?.trim()),
  };
}

function normalizeInitialState(state: Partial<InitialState> | null | undefined): InitialState {
  return {
    ...defaultInitialState,
    ...state,
    config: normalizeConfig(state?.config),
    folders: normalizeArray(state?.folders),
    papers: normalizeArray(state?.papers),
    activeFolderId: state?.activeFolderId ?? '',
    deepStartSessions: normalizeArray(state?.deepStartSessions),
    activeDeepStartSession: normalizeSessionDetail(state?.activeDeepStartSession),
  };
}

function normalizeScreeningDecisionNode(
  node: Partial<ScreeningDecisionNode> | null | undefined,
): ScreeningDecisionNode | null {
  if (!node) {
    return null;
  }

  return {
    id: node.id ?? '',
    nodeType: node.nodeType === 'complete' ? 'complete' : 'branch',
    message: node.message ?? '',
    dimension: node.dimension ?? '',
    options: normalizeArray(node.options).map((option) => ({
      key: option.key ?? '',
      label: option.label ?? '',
      paperIds: normalizeArray(option.paperIds),
      count: option.count ?? normalizeArray(option.paperIds).length,
    })),
    allowMultiSelect: Boolean(node.allowMultiSelect),
    allowSkip: Boolean(node.allowSkip),
    remainingPaperIds: normalizeArray(node.remainingPaperIds),
  };
}

function normalizeScreeningSessionDetail(
  detail: Partial<ScreeningSessionDetail> | null | undefined,
): ScreeningSessionDetail {
  return {
    session: {
      id: detail?.session?.id ?? '',
      title: detail?.session?.title ?? '',
      status: detail?.session?.status ?? 'upload',
      totalPapers: detail?.session?.totalPapers ?? 0,
      currentNodeJson: detail?.session?.currentNodeJson ?? '',
      selectedOptionsJson: detail?.session?.selectedOptionsJson ?? '',
      pathHistoryJson: detail?.session?.pathHistoryJson ?? '',
      createdAt: detail?.session?.createdAt ?? new Date().toISOString(),
      updatedAt: detail?.session?.updatedAt ?? new Date().toISOString(),
    },
    papers: normalizeArray(detail?.papers),
    currentNode: normalizeScreeningDecisionNode(detail?.currentNode),
    pathHistory: normalizeArray(detail?.pathHistory),
  };
}

function normalizeExtractProgress(
  progress: Partial<ExtractProgress> | null | undefined,
): ExtractProgress {
  return {
    sessionId: progress?.sessionId ?? '',
    total: progress?.total ?? 0,
    completed: progress?.completed ?? 0,
    currentFile: progress?.currentFile ?? '',
    status: progress?.status === 'completed' || progress?.status === 'error' ? progress.status : 'processing',
    errorMessage: progress?.errorMessage ?? '',
  };
}

function normalizeSyncStatus(status: Partial<SyncStatus> | null | undefined): SyncStatus {
  return {
    enabled: Boolean(status?.enabled),
    provider: status?.provider ?? 'baidu_cloud',
    lastSync: status?.lastSync ?? null,
    syncInProgress: Boolean(status?.syncInProgress),
    pendingFiles: status?.pendingFiles ?? 0,
    conflicts: status?.conflicts ?? 0,
    totalSynced: status?.totalSynced ?? 0,
    totalFailed: status?.totalFailed ?? 0,
  };
}

function normalizeSyncProgress(progress: Partial<SyncProgress> | null | undefined): SyncProgress {
  return {
    total: progress?.total ?? 0,
    completed: progress?.completed ?? 0,
    currentFile: progress?.currentFile ?? '',
    status: progress?.status ?? 'idle',
    message: progress?.message ?? '',
  };
}

const mockFolders: Folder[] = [
  {
    id: 'mock-inbox',
    name: 'Inbox',
    createdAt: new Date().toISOString(),
  },
];

let mockConfig: AppConfig = defaultConfig;
let mockPapers: Paper[] = [];
const mockTranslations = new Map<string, TranslationRecord[]>();
let mockDeepStartSessions: DeepStartSessionSummary[] = [];
const mockDeepStartDetails = new Map<string, DeepStartSessionDetail>();

function redactConfig(config: AppConfig): AppConfig {
  return {
    ...config,
    llm: {
      ...config.llm,
      apiKey: '',
      hasApiKey: config.llm.apiKey.trim().length > 0 || config.llm.hasApiKey,
      clearApiKey: false,
    },
    weakLLM: {
      ...config.weakLLM,
      apiKey: '',
      hasApiKey: config.weakLLM.apiKey.trim().length > 0 || config.weakLLM.hasApiKey,
      clearApiKey: false,
    },
    search: {
      ...config.search,
      semanticScholarApiKey: '',
      hasSemanticScholarApiKey:
        config.search.semanticScholarApiKey.trim().length > 0 || config.search.hasSemanticScholarApiKey,
      clearSemanticScholarApiKey: false,
    },
    baiduCloud: {
      ...config.baiduCloud,
      token: '',
      hasToken: config.baiduCloud.token.trim().length > 0 || config.baiduCloud.hasToken,
      clearToken: false,
    },
  };
}

const sampleSearchResults: SearchPaper[] = [
  {
    id: '2403.10001',
    title: 'Agentic Code Intelligence for Long-Horizon Software Tasks',
    authors: 'A. Researcher, B. Engineer',
    abstract:
      'We study agentic workflows for software engineering tasks and compare retrieval, planning and tool-use strategies.',
    year: 2024,
    journal: 'arXiv',
    url: 'https://arxiv.org/abs/2403.10001',
    category: 'code agent',
    tags: ['agent', 'software engineering'],
    source: 'mock',
  },
  {
    id: '2402.20002',
    title: 'Survey of LLM-Based Scientific Reading Assistants',
    authors: 'C. Author, D. Scholar',
    abstract:
      'This survey reviews AI-assisted paper reading systems, translation support, note synthesis and retrieval workflows.',
    year: 2024,
    journal: 'arXiv',
    url: 'https://arxiv.org/abs/2402.20002',
    category: 'paper reading',
    tags: ['survey', 'reading assistant'],
    source: 'mock',
  },
];

function mockInitialState(): InitialState {
  return {
    ...defaultInitialState,
    config: mockConfig,
    folders: [...mockFolders],
    papers: mockPapers.filter((paper) => paper.folderId === mockFolders[0].id),
    activeFolderId: mockFolders[0]?.id ?? '',
    deepStartSessions: [...mockDeepStartSessions],
    activeDeepStartSession: mockDeepStartSessions[0]
      ? mockDeepStartDetails.get(mockDeepStartSessions[0].id) ?? null
      : null,
  };
}

function normalizeMockConfigWithSeeds(
  config: AppConfig,
  strongSeed: MockLLMSeed | null,
  weakSeed: MockLLMSeed | null,
  baiduSeed: MockBaiduSeed | null,
): AppConfig {
  const nextConfig: AppConfig = {
    ...config,
    llm: {
      ...config.llm,
    },
    weakLLM: {
      ...config.weakLLM,
    },
    search: {
      ...config.search,
    },
    baiduCloud: {
      ...config.baiduCloud,
    },
  };

  if (strongSeed) {
    const baseUrl = strongSeed.base_url?.trim() ?? '';
    const model = strongSeed.model?.trim() ?? '';
    const apiKey = strongSeed.api_key?.trim() ?? '';
    const provider = strongSeed.provider?.trim().toLowerCase() ?? '';

    if (provider === 'anthropic') {
      nextConfig.llm.providerType = 'anthropic';
      nextConfig.llm.providerId = 'anthropic';
      nextConfig.llm.providerName = 'Anthropic';
      nextConfig.llm.wireApi = 'anthropic_messages';
      nextConfig.llm.requiresOpenAIAuth = false;
    } else {
      nextConfig.llm.providerType = 'openai_compatible';
      nextConfig.llm.wireApi = 'responses';
      nextConfig.llm.requiresOpenAIAuth = true;

      if (baseUrl.includes('duckcoding.ai')) {
        nextConfig.llm.providerId = 'duckcoding';
        nextConfig.llm.providerName = 'DuckCoding';
      } else if (baseUrl.includes('ark.cn-beijing.volces.com')) {
        nextConfig.llm.providerId = 'volcengine-coding';
        nextConfig.llm.providerName = 'Volcengine Coding';
      }
    }

    if (baseUrl) {
      nextConfig.llm.baseUrl = baseUrl;
    }
    if (model) {
      nextConfig.llm.model = model;
    }
    if (apiKey) {
      nextConfig.llm.apiKey = apiKey;
      nextConfig.llm.hasApiKey = true;
    }
  }

  if (weakSeed) {
    const baseUrl = weakSeed.base_url?.trim() ?? '';
    const model = weakSeed.model?.trim() ?? '';
    const apiKey = weakSeed.api_key?.trim() ?? '';
    const provider = weakSeed.provider?.trim().toLowerCase() ?? '';

    if (provider === 'anthropic') {
      nextConfig.weakLLM.providerType = 'anthropic';
      nextConfig.weakLLM.providerId = 'weak-anthropic';
      nextConfig.weakLLM.providerName = 'Weak Anthropic';
      nextConfig.weakLLM.wireApi = 'anthropic_messages';
      nextConfig.weakLLM.requiresOpenAIAuth = false;
    } else {
      nextConfig.weakLLM.providerType = 'openai_compatible';
      nextConfig.weakLLM.wireApi = 'responses';
      nextConfig.weakLLM.requiresOpenAIAuth = true;

      if (baseUrl.includes('duckcoding.ai')) {
        nextConfig.weakLLM.providerId = 'weak-duckcoding';
        nextConfig.weakLLM.providerName = 'Weak DuckCoding';
      } else if (baseUrl.includes('ark.cn-beijing.volces.com')) {
        nextConfig.weakLLM.providerId = 'weak-volcengine-coding';
        nextConfig.weakLLM.providerName = 'Weak Volcengine Coding';
      }
    }

    if (baseUrl) {
      nextConfig.weakLLM.baseUrl = baseUrl;
    }
    if (model) {
      nextConfig.weakLLM.model = model;
    }
    if (apiKey) {
      nextConfig.weakLLM.apiKey = apiKey;
      nextConfig.weakLLM.hasApiKey = true;
    }
  }

  const baiduToken = baiduSeed?.access_token?.trim() ?? '';
  if (baiduToken) {
    nextConfig.baiduCloud.enabled = true;
    nextConfig.baiduCloud.token = baiduToken;
    nextConfig.baiduCloud.hasToken = true;
  }

  return normalizeConfig(nextConfig);
}

async function fetchMockJSON<T>(relativePath: string): Promise<T | null> {
  if (!import.meta.env.DEV || !__DIVEEND_ROOT__) {
    return null;
  }

  const root = __DIVEEND_ROOT__.replace(/\\/g, '/').replace(/\/$/, '');
  const url = `/@fs${encodeURI(`${root}/${relativePath}`)}`;

  try {
    const response = await fetch(url);
    if (!response.ok) {
      return null;
    }
    return (await response.json()) as T;
  } catch {
    return null;
  }
}

async function loadMockSecretPrefill(): Promise<ConfigSecretPrefill> {
  const [strongSeed, weakSeed, baiduSeed] = await Promise.all([
    fetchMockJSON<MockLLMSeed>('config/strong_llm.json'),
    fetchMockJSON<MockLLMSeed>('config/weak_llm.json'),
    fetchMockJSON<MockBaiduSeed>('baiduyun_token.json'),
  ]);

  return normalizeSecretPrefill({
    strongLLMApiKey: strongSeed?.api_key ?? '',
    hasStrongLLMApiKey: Boolean(strongSeed?.api_key?.trim()),
    weakLLMApiKey: weakSeed?.api_key ?? '',
    hasWeakLLMApiKey: Boolean(weakSeed?.api_key?.trim()),
    baiduToken: baiduSeed?.access_token ?? '',
    hasBaiduToken: Boolean(baiduSeed?.access_token?.trim()),
  });
}

export async function getInitialState(): Promise<InitialState> {
  const app = runtimeApp();
  if (app?.GetInitialState) {
    return normalizeInitialState(await app.GetInitialState());
  }

  const [strongSeed, weakSeed, baiduSeed] = await Promise.all([
    fetchMockJSON<MockLLMSeed>('config/strong_llm.json'),
    fetchMockJSON<MockLLMSeed>('config/weak_llm.json'),
    fetchMockJSON<MockBaiduSeed>('baiduyun_token.json'),
  ]);
  mockConfig = normalizeMockConfigWithSeeds(mockConfig, strongSeed, weakSeed, baiduSeed);
  return mockInitialState();
}

export async function getSecretPrefill(): Promise<ConfigSecretPrefill> {
  const app = runtimeApp();
  if (app?.GetSecretPrefill) {
    return normalizeSecretPrefill(await app.GetSecretPrefill());
  }

  return loadMockSecretPrefill();
}

export async function saveConfig(config: AppConfig): Promise<SaveConfigResult> {
  const app = runtimeApp();
  if (app?.SaveConfig) {
    return app.SaveConfig(config);
  }
  mockConfig = redactConfig(config);
  return { config: mockConfig, restartRequired: false };
}

export async function searchPapers(query: string, limit: number): Promise<SearchPaper[]> {
  const app = runtimeApp();
  if (app?.SearchPapers) {
    return normalizeArray(await app.SearchPapers(query, limit));
  }

  const normalized = query.trim().toLowerCase();
  return sampleSearchResults
    .filter((paper) => {
      const haystack = `${paper.title} ${paper.abstract} ${paper.authors}`.toLowerCase();
      return haystack.includes(normalized);
    })
    .slice(0, limit);
}

// 新增：增强搜索API
export async function searchPapersEnhanced(
  query: string,
  limit: number = 100,
  offset: number = 0,
  yearStart?: number,
  yearEnd?: number,
  sortBy?: 'relevance' | 'year_desc' | 'year_asc'
): Promise<EnhancedSearchResult> {
  const app = runtimeApp();
  if (app?.EnhancedSearchPapers) {
    const result = await app.EnhancedSearchPapers(query, limit, offset, yearStart || 0, yearEnd || 0, sortBy || 'relevance');
    return {
      query: result.query,
      limit: result.limit,
      offset: result.offset,
      total: result.total,
      hasMore: result.hasMore,
      papers: normalizeArray(result.papers),
      sources: result.sources,
      yearStart: result.yearStart,
      yearEnd: result.yearEnd,
      sortBy: result.sortBy,
    };
  }

  // 降级到普通搜索
  const papers = await searchPapers(query, limit);
  return {
    query,
    limit,
    offset,
    total: papers.length,
    hasMore: false,
    papers,
    sources: [{ name: 'mock', success: true, count: papers.length }],
  };
}


function buildMockDeepStartAnalysis(query: string, results: SearchPaper[]): DeepStartAnalysis {
  const recommendedPaperIds = results.slice(0, 2).map((paper) => paper.id);
  return {
    overview:
      results.length > 0
        ? `我先把“${query}”这一轮候选论文整理成两层：优先建立领域地图的代表作，以及适合补充阅读的扩展材料。`
        : `这一轮还没有拿到稳定结果，你可以换一个更具体的查询，或者点击建议 query 再试一轮。`,
    directions:
      results.length > 0
        ? [
            {
              id: 'priority',
              name: '优先建立地图',
              summary: '适合先快速判断这个方向的主线和代表性问题。',
              why: '先看这组更容易形成整体认知。',
              paperIds: results.slice(0, 2).map((paper) => paper.id),
            },
            {
              id: 'extended',
              name: '扩展阅读',
              summary: '适合在主线确定后继续往细分主题里钻。',
              why: '避免第一次筛选时信息过载。',
              paperIds: results.slice(2).map((paper) => paper.id),
            },
          ].filter((direction) => direction.paperIds.length > 0)
        : [
            {
              id: 'retry',
              name: '换一轮查询',
              summary: '先把问题改成 survey、benchmark 或 recent progress 这类组合。',
              why: '这样更容易拿到稳定候选。',
              paperIds: [],
            },
          ],
    paperNotes: results.map((paper, index) => ({
      paperId: paper.id,
      tier: index === 0 ? 'core' : index < 3 ? 'important' : 'optional',
      reason:
        index === 0
          ? '适合作为这轮探索的起点。'
          : index < 3
            ? '适合补主线。'
            : '更适合作为补充阅读。',
      directionIds: index < 2 ? ['priority'] : ['extended'],
    })),
    followUpQuestions: [
      '先帮我保留 survey / overview 论文，其他先放一边。',
      '我更关心 benchmark 和评测范式。',
      '先只看最近两年的代表性论文。',
    ],
    suggestedQueries: [`${query} survey`, `${query} benchmark`, `${query} recent progress`],
    recommendedPaperIds,
  };
}

function upsertMockDeepStartSession(detail: DeepStartSessionDetail) {
  mockDeepStartDetails.set(detail.summary.id, detail);
  mockDeepStartSessions = [
    detail.summary,
    ...mockDeepStartSessions.filter((session) => session.id !== detail.summary.id),
  ].sort((a, b) => b.updatedAt.localeCompare(a.updatedAt));
}

export async function listDeepStartSessions(): Promise<DeepStartSessionSummary[]> {
  const app = runtimeApp();
  if (app?.ListDeepStartSessions) {
    return normalizeArray(await app.ListDeepStartSessions());
  }
  return [...mockDeepStartSessions];
}

export async function getDeepStartSession(sessionId: string): Promise<DeepStartSessionDetail> {
  const app = runtimeApp();
  if (app?.GetDeepStartSession) {
    const detail = await app.GetDeepStartSession(sessionId);
    const normalized = normalizeSessionDetail(detail);
    if (!normalized) {
      throw new Error('会话不存在');
    }
    return normalized;
  }

  const detail = mockDeepStartDetails.get(sessionId);
  if (!detail) {
    throw new Error('会话不存在');
  }
  return detail;
}

export async function startDeepStartSession(
  prompt: string,
  targetFolderId: string
): Promise<DeepStartSessionDetail> {
  const app = runtimeApp();
  if (app?.StartDeepStartSession) {
    const detail = await app.StartDeepStartSession(prompt, targetFolderId);
    const normalized = normalizeSessionDetail(detail);
    if (!normalized) {
      throw new Error('创建探索会话失败');
    }
    return normalized;
  }

  const currentResults = await searchPapers(prompt, 20);
  const now = new Date().toISOString();
  const detail: DeepStartSessionDetail = {
    summary: {
      id: `mock-session-${Date.now()}`,
      title: prompt,
      rootPrompt: prompt,
      currentQuery: prompt,
      targetFolderId: targetFolderId || mockFolders[0]?.id || '',
      createdAt: now,
      updatedAt: now,
    },
    messages: [
      {
        id: `mock-msg-user-${Date.now()}`,
        sessionId: `mock-session-${Date.now()}`,
        role: 'user',
        content: prompt,
        createdAt: now,
      },
    ],
    currentResults,
    currentAnalysis: buildMockDeepStartAnalysis(prompt, currentResults),
    selectedPaperIds: [],
  };
  detail.messages.push({
    id: `mock-msg-assistant-${Date.now()}`,
    sessionId: detail.summary.id,
    role: 'assistant',
    content: detail.currentAnalysis?.overview ?? '我已经整理好这轮探索结果。',
    createdAt: now,
  });
  detail.messages[0].sessionId = detail.summary.id;
  upsertMockDeepStartSession(detail);
  return detail;
}

export async function replyDeepStartSession(
  sessionId: string,
  message: string
): Promise<DeepStartSessionDetail> {
  const app = runtimeApp();
  if (app?.ReplyDeepStartSession) {
    const detail = await app.ReplyDeepStartSession(sessionId, message);
    const normalized = normalizeSessionDetail(detail);
    if (!normalized) {
      throw new Error('会话不存在');
    }
    return normalized;
  }

  const existing = mockDeepStartDetails.get(sessionId);
  if (!existing) {
    throw new Error('会话不存在');
  }

  const now = new Date().toISOString();
  const detail: DeepStartSessionDetail = {
    ...existing,
    summary: { ...existing.summary, updatedAt: now },
    messages: [
      ...existing.messages,
      {
        id: `mock-msg-user-${Date.now()}`,
        sessionId,
        role: 'user',
        content: message,
        createdAt: now,
      },
      {
        id: `mock-msg-assistant-${Date.now()}`,
        sessionId,
        role: 'assistant',
        content: '我已经根据你的补充偏好，重新强调了更适合作为主线的论文和下一步问题。',
        createdAt: now,
      },
    ],
    currentAnalysis: buildMockDeepStartAnalysis(existing.summary.currentQuery, existing.currentResults),
  };
  upsertMockDeepStartSession(detail);
  return detail;
}

export async function rerunDeepStartSearch(
  sessionId: string,
  query: string
): Promise<DeepStartSessionDetail> {
  const app = runtimeApp();
  if (app?.RerunDeepStartSearch) {
    const detail = await app.RerunDeepStartSearch(sessionId, query);
    const normalized = normalizeSessionDetail(detail);
    if (!normalized) {
      throw new Error('会话不存在');
    }
    return normalized;
  }

  const existing = mockDeepStartDetails.get(sessionId);
  if (!existing) {
    throw new Error('会话不存在');
  }

  const currentResults = await searchPapers(query, 20);
  const now = new Date().toISOString();
  const detail: DeepStartSessionDetail = {
    ...existing,
    summary: {
      ...existing.summary,
      currentQuery: query,
      updatedAt: now,
    },
    messages: [
      ...existing.messages,
      {
        id: `mock-msg-user-${Date.now()}`,
        sessionId,
        role: 'user',
        content: `重新检索：${query}`,
        createdAt: now,
      },
      {
        id: `mock-msg-assistant-${Date.now()}`,
        sessionId,
        role: 'assistant',
        content: `我已经按新查询“${query}”重新整理了一轮候选论文。`,
        createdAt: now,
      },
    ],
    currentResults,
    currentAnalysis: buildMockDeepStartAnalysis(query, currentResults),
    selectedPaperIds: existing.selectedPaperIds.filter((paperId) =>
      currentResults.some((paper) => paper.id === paperId)
    ),
  };
  upsertMockDeepStartSession(detail);
  return detail;
}

export async function updateDeepStartSelections(
  sessionId: string,
  selectedPaperIds: string[],
  targetFolderId: string
): Promise<DeepStartSessionDetail> {
  const app = runtimeApp();
  if (app?.UpdateDeepStartSelections) {
    const detail = await app.UpdateDeepStartSelections(sessionId, selectedPaperIds, targetFolderId);
    const normalized = normalizeSessionDetail(detail);
    if (!normalized) {
      throw new Error('会话不存在');
    }
    return normalized;
  }

  const existing = mockDeepStartDetails.get(sessionId);
  if (!existing) {
    throw new Error('会话不存在');
  }

  const now = new Date().toISOString();
  const detail: DeepStartSessionDetail = {
    ...existing,
    summary: {
      ...existing.summary,
      targetFolderId: targetFolderId || existing.summary.targetFolderId,
      updatedAt: now,
    },
    selectedPaperIds,
  };
  upsertMockDeepStartSession(detail);
  return detail;
}

export async function getFolders(): Promise<Folder[]> {
  const app = runtimeApp();
  if (app?.GetFolders) {
    return normalizeArray(await app.GetFolders());
  }
  return [...mockFolders];
}

export async function createFolder(name: string): Promise<Folder> {
  const app = runtimeApp();
  if (app?.CreateFolder) {
    return app.CreateFolder(name);
  }

  const existing = mockFolders.find((folder) => folder.name === name);
  if (existing) {
    return existing;
  }

  const folder: Folder = {
    id: `mock-folder-${Date.now()}`,
    name,
    createdAt: new Date().toISOString(),
  };
  mockFolders.push(folder);
  return folder;
}

export async function getPapers(folderId: string): Promise<Paper[]> {
  const app = runtimeApp();
  if (app?.GetPapers) {
    return normalizeArray(await app.GetPapers(folderId));
  }
  return mockPapers.filter((paper) => paper.folderId === folderId);
}

export async function importPapers(folderId: string, papers: SearchPaper[]): Promise<Paper[]> {
  const app = runtimeApp();
  if (app?.ImportPapers) {
    return normalizeArray(await app.ImportPapers(folderId, papers));
  }

  const imported = papers.map<Paper>((paper) => ({
    id: paper.id,
    title: paper.title,
    authors: paper.authors,
    abstract: paper.abstract,
    year: paper.year,
    journal: paper.journal,
    url: paper.url,
    folderId,
    category: paper.category,
    tags: paper.tags,
    addedAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
  }));

  for (const paper of imported) {
    mockPapers = [paper, ...mockPapers.filter((item) => item.id !== paper.id)];
  }

  return imported;
}

export async function deletePaper(id: string): Promise<void> {
  const app = runtimeApp();
  if (app?.DeletePaper) {
    return app.DeletePaper(id);
  }

  mockPapers = mockPapers.filter((paper) => paper.id !== id);
  mockTranslations.delete(id);
}

export async function translatePaperSection(
  paperId: string,
  section: string,
  text: string
): Promise<TranslationRecord> {
  const app = runtimeApp();
  if (app?.TranslatePaperSection) {
    return app.TranslatePaperSection(paperId, section, text);
  }

  const record: TranslationRecord = {
    id: `mock-translation-${Date.now()}`,
    paperId,
    section,
    originalText: text,
    translatedText: `[演示模式] ${text}`,
    summary: '演示模式下返回本地假数据，用于前端联调。',
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
  };

  const history = mockTranslations.get(paperId) ?? [];
  mockTranslations.set(paperId, [record, ...history]);
  return record;
}

export async function getTranslations(paperId: string): Promise<TranslationRecord[]> {
  const app = runtimeApp();
  if (app?.GetTranslations) {
    return normalizeArray(await app.GetTranslations(paperId));
  }
  return mockTranslations.get(paperId) ?? [];
}

// ============ Screening API ============

export function canResolveFilePaths(): boolean {
  if (!hasWailsRuntime()) {
    return false;
  }
  return CanResolveFilePaths();
}

export function resolveFilePaths(files: File[]): string[] {
  if (!hasWailsRuntime()) {
    return [];
  }

  return (ResolveFilePaths(files) as unknown as string[]) ?? [];
}

export function onExtractProgress(callback: (progress: ExtractProgress) => void): () => void {
  if (!hasWailsRuntime()) {
    return () => undefined;
  }

  const unsubscribe = EventsOn('extract-progress', (progress: ExtractProgress) => {
    callback(normalizeExtractProgress(progress));
  });

  return () => {
    unsubscribe?.();
    EventsOff('extract-progress');
  };
}

export async function createScreeningSession(title: string): Promise<ScreeningSession> {
  const app = runtimeApp();
  if (app?.CreateScreeningSession) {
    return app.CreateScreeningSession(title);
  }
  return {
    id: `mock-session-${Date.now()}`,
    title,
    status: 'upload',
    totalPapers: 0,
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
  };
}

export async function uploadScreeningFiles(sessionId: string, filePaths: string[]): Promise<ScreeningSessionDetail> {
  const app = runtimeApp();
  if (app?.UploadScreeningFiles) {
    return normalizeScreeningSessionDetail(await app.UploadScreeningFiles(sessionId, filePaths));
  }

  return normalizeScreeningSessionDetail({
    session: {
      id: sessionId,
      title: 'Mock Screening Session',
      status: 'extract',
      totalPapers: filePaths.length,
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    },
    papers: filePaths.map((filePath, index) => ({
      id: `mock-paper-${index}`,
      sessionId,
      fileName: filePath.split('/').pop() ?? filePath,
      filePath,
      fileSize: 0,
      status: 'pending',
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    })),
    currentNode: null,
    pathHistory: [],
  });
}

export async function extractPaperContent(sessionId: string): Promise<ExtractProgress> {
  const app = runtimeApp();
  if (app?.ExtractPaperContent) {
    return normalizeExtractProgress(await app.ExtractPaperContent(sessionId));
  }

  return new Promise((resolve) => {
    setTimeout(() => {
      resolve(
        normalizeExtractProgress({
          sessionId,
          status: 'completed',
          total: 2,
          completed: 2,
          currentFile: '',
        }),
      );
    }, 500);
  });
}

export async function getExtractProgress(sessionId: string): Promise<ExtractProgress> {
  const app = runtimeApp();
  if (app?.GetExtractProgress) {
    return normalizeExtractProgress(await app.GetExtractProgress(sessionId));
  }
  return normalizeExtractProgress({ sessionId, total: 0, completed: 0, currentFile: '', status: 'processing' });
}

export async function analyzePapers(sessionId: string): Promise<ScreeningDecisionNode> {
  const app = runtimeApp();
  if (app?.AnalyzePapers) {
    return normalizeScreeningDecisionNode(await app.AnalyzePapers(sessionId)) as ScreeningDecisionNode;
  }

  return normalizeScreeningDecisionNode({
    id: 'node-1',
    nodeType: 'branch',
    message: '你最想先保留哪一类论文？',
    dimension: '研究方向',
    options: [
      { key: 'survey', label: '综述与综览', paperIds: ['mock-paper-1'], count: 1 },
      { key: 'benchmark', label: '基准与实验', paperIds: ['mock-paper-2'], count: 1 },
    ],
    allowMultiSelect: true,
    allowSkip: false,
    remainingPaperIds: ['mock-paper-1', 'mock-paper-2'],
  }) as ScreeningDecisionNode;
}

export async function applyScreeningChoice(
  sessionId: string,
  selectedOptions: string[],
): Promise<ScreeningDecisionNode> {
  const app = runtimeApp();
  if (app?.ApplyScreeningChoice) {
    return normalizeScreeningDecisionNode(await app.ApplyScreeningChoice(sessionId, selectedOptions)) as ScreeningDecisionNode;
  }

  return normalizeScreeningDecisionNode({
    id: 'node-complete',
    nodeType: 'complete',
    message: '筛选完成，请确认导入剩余论文。',
    dimension: '结果确认',
    options: [],
    allowMultiSelect: false,
    allowSkip: false,
    remainingPaperIds: ['mock-paper-1'],
  }) as ScreeningDecisionNode;
}

export async function completeScreening(sessionId: string, targetFolderId: string): Promise<Paper[]> {
  const app = runtimeApp();
  if (app?.CompleteScreening) {
    return normalizeArray(await app.CompleteScreening(sessionId, targetFolderId));
  }
  return [];
}

export async function listScreeningSessions(): Promise<ScreeningSession[]> {
  const app = runtimeApp();
  if (app?.ListScreeningSessions) {
    return normalizeArray(await app.ListScreeningSessions());
  }
  return [];
}

export async function getScreeningSession(sessionId: string): Promise<ScreeningSessionDetail> {
  const app = runtimeApp();
  if (app?.GetScreeningSession) {
    return normalizeScreeningSessionDetail(await app.GetScreeningSession(sessionId));
  }
  return normalizeScreeningSessionDetail({
    session: {
      id: sessionId,
      title: 'Mock Screening Session',
      status: 'upload',
      totalPapers: 0,
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    },
    papers: [],
    currentNode: null,
    pathHistory: [],
  });
}

export async function cancelScreening(sessionId: string): Promise<void> {
  const app = runtimeApp();
  if (app?.CancelScreening) {
    return app.CancelScreening(sessionId);
  }
}

// ============ Sync API ============

export async function getSyncStatus(): Promise<SyncStatus> {
  const app = runtimeApp();
  if (app?.GetSyncStatus) {
    return normalizeSyncStatus(await app.GetSyncStatus());
  }
  return normalizeSyncStatus({ enabled: false, provider: 'baidu_cloud', lastSync: null });
}

export async function triggerSync(): Promise<SyncProgress> {
  const app = runtimeApp();
  if (app?.TriggerSync) {
    return normalizeSyncProgress(await app.TriggerSync());
  }
  return normalizeSyncProgress({ status: 'completed', total: 0, completed: 0, currentFile: '' });
}

export async function getSyncProgress(): Promise<SyncProgress> {
  const app = runtimeApp();
  if (app?.GetSyncProgress) {
    return normalizeSyncProgress(await app.GetSyncProgress());
  }
  return normalizeSyncProgress({ total: 0, completed: 0, currentFile: '', status: 'idle' });
}

export async function getSyncConflicts(): Promise<SyncConflict[]> {
  const app = runtimeApp();
  if (app?.GetSyncConflicts) {
    return normalizeArray(await app.GetSyncConflicts());
  }
  return [];
}

export async function getSyncRecords(limit = 20): Promise<SyncRecord[]> {
  const app = runtimeApp();
  if (app?.GetSyncRecords) {
    return normalizeArray(await app.GetSyncRecords(limit));
  }
  return [];
}

export async function resolveSyncConflict(conflictId: string, resolution: 'local' | 'remote'): Promise<void> {
  const app = runtimeApp();
  if (app?.ResolveSyncConflict) {
    return app.ResolveSyncConflict(conflictId, resolution);
  }
}
