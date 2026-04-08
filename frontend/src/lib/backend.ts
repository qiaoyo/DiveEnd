import type {
  AppConfig,
  DeepStartAnalysis,
  DeepStartSessionDetail,
  DeepStartSessionSummary,
  Folder,
  InitialState,
  Paper,
  SaveConfigResult,
  SearchPaper,
  TranslationRecord,
} from '../types';
import { defaultConfig, defaultInitialState } from '../types';

declare global {
  interface Window {
    go?: {
      main?: {
        App?: {
          GetInitialState(): Promise<InitialState>;
          SaveConfig(config: AppConfig): Promise<SaveConfigResult>;
          SearchPapers(query: string, limit: number): Promise<SearchPaper[]>;
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
          CreateScreeningSession(title: string): Promise<any>;
          UploadScreeningFiles(sessionId: string, filePaths: string[]): Promise<any>;
          ExtractPaperContent(sessionId: string): Promise<any>;
          AnalyzePapers(sessionId: string): Promise<any>;
          ApplyScreeningChoice(sessionId: string, selectedOptions: string[]): Promise<any>;
          CompleteScreening(sessionId: string, targetFolderId: string): Promise<any>;
          ListScreeningSessions(): Promise<any[]>;
          GetScreeningSession(sessionId: string): Promise<any>;
          CancelScreening(sessionId: string): Promise<void>;

          // Sync API
          GetSyncStatus(): Promise<any>;
          TriggerSync(): Promise<any>;
          GetSyncProgress(): Promise<any>;
          GetSyncConflicts(): Promise<any[]>;
          ResolveSyncConflict(conflictId: string, resolution: 'local' | 'remote'): Promise<void>;
        };
      };
    };
  }
}

const runtimeApp = () => window.go?.main?.App;

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

export async function getInitialState(): Promise<InitialState> {
  const app = runtimeApp();
  if (app?.GetInitialState) {
    return app.GetInitialState();
  }
  return mockInitialState();
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
    return app.SearchPapers(query, limit);
  }

  const normalized = query.trim().toLowerCase();
  return sampleSearchResults
    .filter((paper) => {
      const haystack = `${paper.title} ${paper.abstract} ${paper.authors}`.toLowerCase();
      return haystack.includes(normalized);
    })
    .slice(0, limit);
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
    return app.ListDeepStartSessions();
  }
  return [...mockDeepStartSessions];
}

export async function getDeepStartSession(sessionId: string): Promise<DeepStartSessionDetail> {
  const app = runtimeApp();
  if (app?.GetDeepStartSession) {
    return app.GetDeepStartSession(sessionId);
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
    return app.StartDeepStartSession(prompt, targetFolderId);
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
    return app.ReplyDeepStartSession(sessionId, message);
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
    return app.RerunDeepStartSearch(sessionId, query);
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
    return app.UpdateDeepStartSelections(sessionId, selectedPaperIds, targetFolderId);
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
    return app.GetFolders();
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
    return app.GetPapers(folderId);
  }
  return mockPapers.filter((paper) => paper.folderId === folderId);
}

export async function importPapers(folderId: string, papers: SearchPaper[]): Promise<Paper[]> {
  const app = runtimeApp();
  if (app?.ImportPapers) {
    return app.ImportPapers(folderId, papers);
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
    return app.GetTranslations(paperId);
  }
  return mockTranslations.get(paperId) ?? [];
}

// ============ Screening API ============

export async function createScreeningSession(title: string): Promise<any> {
  const app = runtimeApp();
  if (app?.CreateScreeningSession) {
    return app.CreateScreeningSession(title);
  }
  return { id: `mock-session-${Date.now()}`, title, status: 'upload' };
}

export async function uploadScreeningFiles(sessionId: string, filePaths: string[]): Promise<any> {
  const app = runtimeApp();
  if (app?.UploadScreeningFiles) {
    return app.UploadScreeningFiles(sessionId, filePaths);
  }
  return { session: { id: sessionId, status: 'extract' }, papers: [] };
}

export async function extractPaperContent(sessionId: string): Promise<any> {
  const app = runtimeApp();
  if (app?.ExtractPaperContent) {
    return app.ExtractPaperContent(sessionId);
  }

  // Mock extraction with progress
  return new Promise((resolve) => {
    setTimeout(() => {
      resolve({ status: 'completed', total: 5, completed: 5 });
    }, 2000);
  });
}

export async function analyzePapers(sessionId: string): Promise<any> {
  const app = runtimeApp();
  if (app?.AnalyzePapers) {
    return app.AnalyzePapers(sessionId);
  }

  return {
    id: 'node-1',
    message: 'What research areas are you most interested in?',
    dimension: 'Research Area',
    options: [
      { key: 'ml', label: 'Machine Learning', paperIds: [], count: 15 },
      { key: 'nlp', label: 'Natural Language Processing', paperIds: [], count: 12 },
      { key: 'cv', label: 'Computer Vision', paperIds: [], count: 8 },
      { key: 'rl', label: 'Reinforcement Learning', paperIds: [], count: 5 },
    ],
    allowMultiSelect: true,
  };
}

export async function applyScreeningChoice(sessionId: string, selectedOptions: string[]): Promise<any> {
  const app = runtimeApp();
  if (app?.ApplyScreeningChoice) {
    return app.ApplyScreeningChoice(sessionId, selectedOptions);
  }

  return {
    id: 'node-2',
    message: 'What methodology types do you prefer?',
    dimension: 'Methodology',
    options: [
      { key: 'empirical', label: 'Empirical Study', paperIds: [], count: 10 },
      { key: 'theoretical', label: 'Theoretical Analysis', paperIds: [], count: 8 },
      { key: 'review', label: 'Survey/Review', paperIds: [], count: 5 },
    ],
    allowMultiSelect: true,
  };
}

export async function completeScreening(sessionId: string, targetFolderId: string): Promise<any> {
  const app = runtimeApp();
  if (app?.CompleteScreening) {
    return app.CompleteScreening(sessionId, targetFolderId);
  }
  return { importedCount: 3 };
}

export async function listScreeningSessions(): Promise<any[]> {
  const app = runtimeApp();
  if (app?.ListScreeningSessions) {
    return app.ListScreeningSessions();
  }
  return [];
}

export async function getScreeningSession(sessionId: string): Promise<any> {
  const app = runtimeApp();
  if (app?.GetScreeningSession) {
    return app.GetScreeningSession(sessionId);
  }
  return { id: sessionId, title: 'Test Session', papers: [] };
}

export async function cancelScreening(sessionId: string): Promise<void> {
  const app = runtimeApp();
  if (app?.CancelScreening) {
    return app.CancelScreening(sessionId);
  }
}

// ============ Sync API ============

export async function getSyncStatus(): Promise<any> {
  const app = runtimeApp();
  if (app?.GetSyncStatus) {
    return app.GetSyncStatus();
  }
  return { enabled: false, provider: '', lastSync: null };
}

export async function triggerSync(): Promise<any> {
  const app = runtimeApp();
  if (app?.TriggerSync) {
    return app.TriggerSync();
  }
  return { status: 'completed', filesSynced: 0 };
}

export async function getSyncProgress(): Promise<any> {
  const app = runtimeApp();
  if (app?.GetSyncProgress) {
    return app.GetSyncProgress();
  }
  return { total: 0, completed: 0, currentFile: '' };
}

export async function getSyncConflicts(): Promise<any[]> {
  const app = runtimeApp();
  if (app?.GetSyncConflicts) {
    return app.GetSyncConflicts();
  }
  return [];
}

export async function resolveSyncConflict(conflictId: string, resolution: 'local' | 'remote'): Promise<void> {
  const app = runtimeApp();
  if (app?.ResolveSyncConflict) {
    return app.ResolveSyncConflict(conflictId, resolution);
  }
}
