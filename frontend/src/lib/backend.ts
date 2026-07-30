import type {
  AppConfig,
  ConfigSecretPrefill,
  DatabaseRestoreStatus,
  DeepStartAnalysis,
  DeepStartProgressEvent,
  DeepStartSessionDetail,
  DeepStartSessionSummary,
  DeepReadAIResponse,
  DeepReadNote,
  DeepReadState,
  EnhancedSearchResult,
  ExtractProgress,
  FolderNode,
  FolderStorageTreeOverview,
  Folder,
  CreateFolderNodeRequest,
  RenameFolderNodeRequest,
  MoveFolderNodeRequest,
  ImportPapersWithAssetsResult,
  InitialState,
  LocalStorageOverview,
  Paper,
  PDFServiceStatus,
  SaveConfigResult,
  ScreeningDecisionNode,
  ScreeningSession,
  ScreeningSessionDetail,
  SearchPaper,
  SearchProgressEvent,
  BaiduTokenRefreshStatus,
  SyncConflict,
  SyncPreview,
  SyncProgress,
  SyncRecord,
  SyncSettings,
  SyncStatus,
  TranslationRecord,
} from '../types';
import { defaultConfig, defaultInitialState } from '../types';
import { CanResolveFilePaths, EventsOn, ResolveFilePaths } from '../../wailsjs/runtime/runtime';
import { sanitizeUserVisibleError } from './errors';

declare global {
  interface Window {
    go?: {
      main?: {
        App?: {
          GetInitialState(): Promise<InitialState>;
          GetSecretPrefill(): Promise<ConfigSecretPrefill>;
          GetPDFServiceStatus(): Promise<PDFServiceStatus>;
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
          CancelDeepStartTask(sessionId: string): Promise<void>;
          ReplyDeepStartSession(sessionId: string, message: string): Promise<DeepStartSessionDetail>;
          SupplementDeepStartSearch(
            sessionId: string,
            query: string,
            perSourceLimit: number
          ): Promise<DeepStartSessionDetail>;
          UndoDeepStartNarrow(sessionId: string): Promise<DeepStartSessionDetail>;
          RerunDeepStartSearch(sessionId: string, query: string): Promise<DeepStartSessionDetail>;
          UpdateDeepStartSelections(
            sessionId: string,
            selectedPaperIds: string[],
            targetFolderId: string
          ): Promise<DeepStartSessionDetail>;
          GetFolders(): Promise<Folder[]>;
          CreateFolder(name: string): Promise<Folder>;
          GetFolderTree(): Promise<FolderNode[]>;
          CreateFolderNode(request: CreateFolderNodeRequest): Promise<Folder>;
          RenameFolderNode(request: RenameFolderNodeRequest): Promise<Folder>;
          MoveFolderNode(request: MoveFolderNodeRequest): Promise<Folder>;
          DeleteFolderNode(folderId: string): Promise<void>;
          GetPapers(folderId: string): Promise<Paper[]>;
          ImportPapers(folderId: string, papers: SearchPaper[]): Promise<Paper[]>;
          ImportPapersWithAssets(folderId: string, papers: SearchPaper[]): Promise<ImportPapersWithAssetsResult>;
          RetryPaperDownload(paperId: string): Promise<void>;
          RetryPaperDownloadWithURL(paperId: string, manualURL: string): Promise<void>;
          RetryFolderPendingDownloads(folderId: string): Promise<number>;
          SelectAndAttachPaperPDF(paperId: string): Promise<Paper>;
          AttachLocalPDFToPaper(paperId: string, sourcePath: string): Promise<Paper>;
          GetLocalStorageOverview(): Promise<LocalStorageOverview>;
          GetFolderStorageTreeOverview(): Promise<FolderStorageTreeOverview>;
          MovePaperToFolder(paperId: string, targetFolderId: string): Promise<Paper>;
          MovePapersToFolder(paperIds: string[], targetFolderId: string): Promise<Paper[]>;
          DeletePaper(id: string): Promise<void>;
          TranslatePaperSection(
            paperId: string,
            section: string,
            text: string
          ): Promise<TranslationRecord>;
          GetTranslations(paperId: string): Promise<TranslationRecord[]>;
          GetDeepReadState(paperId: string): Promise<DeepReadState>;
          PrepareDeepReadPaper(paperId: string): Promise<DeepReadState>;
          SaveDeepReadNote(paperId: string, section: string, content: string): Promise<DeepReadNote>;
          AskDeepReadPaper(
            paperId: string,
            sectionId: string,
            question: string,
            mode: 'question' | 'summary'
          ): Promise<DeepReadAIResponse>;
          GetDeepReadPDFURL(paperId: string): Promise<string>;
          GetDeepReadPDFBytes(paperId: string): Promise<string>;

          // Screening API
          SelectScreeningPDFs(): Promise<string[]>;
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
          GetSyncPreview(): Promise<SyncPreview>;
          RefreshBaiduToken(): Promise<BaiduTokenRefreshStatus>;
          GetSyncConflicts(): Promise<SyncConflict[]>;
          GetSyncRecords(limit: number): Promise<SyncRecord[]>;
          ResolveSyncConflict(conflictId: string, resolution: 'local' | 'remote' | 'skipped' | 'timestamp'): Promise<void>;
          GetSyncSettings(): Promise<SyncSettings>;
          SaveSyncSettings(settings: SyncSettings): Promise<SyncSettings>;
          GetPendingDatabaseRestore(): Promise<DatabaseRestoreStatus>;
          ApplyPendingDatabaseRestore(): Promise<DatabaseRestoreStatus>;
          CancelPendingDatabaseRestore(): Promise<void>;
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

function assertMockFallbackAllowed(app: ReturnType<typeof runtimeApp>, methodName: string): void {
  if (app) {
    throw new Error(`Wails backend method ${methodName} is unavailable. Please rebuild DiveEnd so frontend bindings match backend.`);
  }
}

const emptySecretPrefill: ConfigSecretPrefill = {
  strongLLMApiKey: '',
  hasStrongLLMApiKey: false,
  weakLLMApiKey: '',
  hasWeakLLMApiKey: false,
  baiduToken: '',
  hasBaiduToken: false,
};

function normalizeArray<T>(value: T[] | null | undefined): T[] {
  return Array.isArray(value) ? value : [];
}

function normalizeStringMap(value: Record<string, string> | null | undefined): Record<string, string> {
  if (!value || typeof value !== 'object') {
    return {};
  }
  const entries = Object.entries(value)
    .map(([key, item]) => [String(key).trim(), String(item ?? '').trim()] as const)
    .filter(([key, item]) => key.length > 0 && item.length > 0);
  return Object.fromEntries(entries);
}

function normalizeFolder(folder: Partial<Folder> | null | undefined): Folder {
  return {
    id: folder?.id ?? '',
    name: folder?.name ?? '',
    parentId: folder?.parentId ?? '',
    path: folder?.path ?? folder?.name ?? '',
    isSystem: Boolean(folder?.isSystem),
    createdAt: folder?.createdAt ?? new Date().toISOString(),
  };
}

function normalizeFolderNode(node: Partial<FolderNode> | null | undefined): FolderNode {
  return {
    folder: normalizeFolder(node?.folder),
    children: normalizeArray(node?.children).map((child) => normalizeFolderNode(child)),
  };
}

function normalizeFolderStorageTreeOverview(
  overview: Partial<FolderStorageTreeOverview> | null | undefined,
): FolderStorageTreeOverview {
  type TreeNode = FolderStorageTreeOverview['directories'][number];
  const normalizeNode = (node: Partial<TreeNode> | null | undefined): TreeNode => ({
    folderId: node?.folderId ?? '',
    folderName: node?.folderName ?? '',
    folderPath: node?.folderPath ?? '',
    paperCount: Number(node?.paperCount ?? 0) || 0,
    queued: Number(node?.queued ?? 0) || 0,
    downloading: Number(node?.downloading ?? 0) || 0,
    downloaded: Number(node?.downloaded ?? 0) || 0,
    failed: Number(node?.failed ?? 0) || 0,
    children: normalizeArray(node?.children).map((child) => normalizeNode(child as Partial<TreeNode>)),
  });

  return {
    rootPath: overview?.rootPath ?? '',
    directories: normalizeArray(overview?.directories).map((node) => normalizeNode(node)),
    generatedAt: overview?.generatedAt ?? new Date().toISOString(),
  };
}

function normalizePaper(paper: Partial<Paper> | null | undefined): Paper {
  return {
    id: paper?.id ?? '',
    sourcePaperId: paper?.sourcePaperId ?? '',
    title: paper?.title ?? '',
    authors: paper?.authors ?? '',
    abstract: paper?.abstract ?? '',
    year: paper?.year ?? 0,
    journal: paper?.journal ?? '',
    url: paper?.url ?? '',
    pdfPath: paper?.pdfPath ?? '',
    downloadStatus: paper?.downloadStatus ?? 'queued',
    downloadError: sanitizeUserVisibleError(paper?.downloadError ?? ''),
    folderId: paper?.folderId ?? '',
    category: paper?.category ?? '',
    tags: normalizeArray(paper?.tags),
    addedAt: paper?.addedAt ?? new Date().toISOString(),
    updatedAt: paper?.updatedAt ?? new Date().toISOString(),
  };
}

function normalizeImportWithAssetsResult(
  result: Partial<ImportPapersWithAssetsResult> | null | undefined,
): ImportPapersWithAssetsResult {
  return {
    imported: normalizeArray(result?.imported).map((paper) => normalizePaper(paper)),
    skipped: normalizeArray(result?.skipped).map((item) => ({
      sourcePaperId: item?.sourcePaperId ?? '',
      title: item?.title ?? '',
      reason: item?.reason ?? '',
    })),
    queued: Number(result?.queued ?? 0) || 0,
    message: result?.message ?? '',
  };
}

function normalizeLocalStorageOverview(
  overview: Partial<LocalStorageOverview> | null | undefined,
): LocalStorageOverview {
  return {
    rootPath: overview?.rootPath ?? '',
    totalFolders: Number(overview?.totalFolders ?? 0) || 0,
    totalFiles: Number(overview?.totalFiles ?? 0) || 0,
    queued: Number(overview?.queued ?? 0) || 0,
    downloading: Number(overview?.downloading ?? 0) || 0,
    downloaded: Number(overview?.downloaded ?? 0) || 0,
    failed: Number(overview?.failed ?? 0) || 0,
    folders: normalizeArray(overview?.folders).map((item) => ({
      folderId: item?.folderId ?? '',
      folderName: item?.folderName ?? '',
      folderPath: item?.folderPath ?? '',
      totalPapers: Number(item?.totalPapers ?? 0) || 0,
      queued: Number(item?.queued ?? 0) || 0,
      downloading: Number(item?.downloading ?? 0) || 0,
      downloaded: Number(item?.downloaded ?? 0) || 0,
      failed: Number(item?.failed ?? 0) || 0,
      storedFileCount: Number(item?.storedFileCount ?? 0) || 0,
    })),
    generatedAt: overview?.generatedAt ?? new Date().toISOString(),
  };
}

function normalizeSearchPaper(paper: Partial<SearchPaper> | null | undefined): SearchPaper {
  return {
    id: paper?.id ?? '',
    title: paper?.title ?? '',
    authors: paper?.authors ?? '',
    abstract: paper?.abstract ?? '',
    year: paper?.year ?? 0,
    journal: paper?.journal ?? '',
    publicationVenue: paper?.publicationVenue ?? '',
    publicationYear: paper?.publicationYear ?? 0,
    citationCount: paper?.citationCount ?? 0,
    url: paper?.url ?? '',
    category: paper?.category ?? '',
    tags: normalizeArray(paper?.tags),
    source: paper?.source ?? '',
    externalIds: normalizeStringMap(paper?.externalIds as Record<string, string> | undefined),
    pdfCandidates: normalizeArray(paper?.pdfCandidates),
    institutions: normalizeArray(paper?.institutions),
    keywords: normalizeArray(paper?.keywords),
    sourceLabel: paper?.sourceLabel ?? '',
    enrichmentNote: paper?.enrichmentNote ?? '',
    preprocessStatus: paper?.preprocessStatus ?? '',
    localPdfPath: paper?.localPdfPath ?? '',
    markdownPath: paper?.markdownPath ?? '',
    parseStatus: paper?.parseStatus ?? '',
    parseError: sanitizeUserVisibleError(paper?.parseError ?? ''),
    extractStatus: paper?.extractStatus ?? '',
    extractError: sanitizeUserVisibleError(paper?.extractError ?? ''),
    problem: paper?.problem ?? '',
    method: paper?.method ?? '',
    topicLabel: paper?.topicLabel ?? '',
    methodLabel: paper?.methodLabel ?? '',
    taskLabel: paper?.taskLabel ?? '',
    domainLabel: paper?.domainLabel ?? '',
    classificationConfidence: Number(paper?.classificationConfidence ?? 0) || 0,
    processingStage: paper?.processingStage ?? '',
    processingError: sanitizeUserVisibleError(paper?.processingError ?? ''),
  };
}

function normalizeDeepStartSummary(
  summary: Partial<DeepStartSessionDetail['summary']> | null | undefined,
): DeepStartSessionDetail['summary'] {
  return {
    id: summary?.id ?? '',
    title: summary?.title ?? '',
    rootPrompt: summary?.rootPrompt ?? '',
    currentQuery: summary?.currentQuery ?? '',
    targetFolderId: summary?.targetFolderId ?? '',
    processingStatus: summary?.processingStatus ?? 'completed',
    initialReadyCount: Number(summary?.initialReadyCount ?? 0) || 0,
    totalPlannedCount: Number(summary?.totalPlannedCount ?? 0) || 0,
    backgroundRemaining: Number(summary?.backgroundRemaining ?? 0) || 0,
    createdAt: summary?.createdAt ?? new Date().toISOString(),
    updatedAt: summary?.updatedAt ?? new Date().toISOString(),
  };
}

function normalizeDeepReadNote(note: Partial<DeepReadNote> | null | undefined): DeepReadNote {
  return {
    id: note?.id ?? '',
    paperId: note?.paperId ?? '',
    section: note?.section ?? '',
    content: note?.content ?? '',
    createdAt: note?.createdAt ?? new Date().toISOString(),
    updatedAt: note?.updatedAt ?? new Date().toISOString(),
  };
}

function normalizeDeepReadAIResponse(
  response: Partial<DeepReadAIResponse> | null | undefined,
): DeepReadAIResponse {
  return {
    mode: response?.mode ?? 'question',
    answer: response?.answer ?? '',
    takeaway: response?.takeaway ?? '',
    evidence: normalizeArray(response?.evidence).map((item) => ({
      sectionId: item?.sectionId ?? '',
      sectionTitle: item?.sectionTitle ?? '',
      excerpt: item?.excerpt ?? '',
    })),
    limitations: normalizeArray(response?.limitations),
  };
}

function normalizeDeepReadState(state: Partial<DeepReadState> | null | undefined): DeepReadState {
  return {
    paperId: state?.paperId ?? '',
    hasPdf: Boolean(state?.hasPdf),
    pdfPath: state?.pdfPath ?? '',
    parseStatus: state?.parseStatus ?? 'idle',
    parseError: sanitizeUserVisibleError(state?.parseError ?? ''),
    sections: normalizeArray(state?.sections).map((section, index) => ({
      id: section?.id ?? `section-${index + 1}`,
      title: section?.title ?? `Section ${index + 1}`,
      content: section?.content ?? '',
      index: section?.index ?? index,
    })),
    markdown: state?.markdown ?? '',
    translations: normalizeArray(state?.translations).map((record) => ({
      id: record?.id ?? '',
      paperId: record?.paperId ?? '',
      section: record?.section ?? '',
      originalText: record?.originalText ?? '',
      translatedText: record?.translatedText ?? '',
      summary: record?.summary ?? '',
      createdAt: record?.createdAt ?? new Date().toISOString(),
      updatedAt: record?.updatedAt ?? new Date().toISOString(),
    })),
    notes: normalizeArray(state?.notes).map((note) => normalizeDeepReadNote(note)),
    lastPreparedAt: state?.lastPreparedAt ?? '',
  };
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
    retainedPaperIds: normalizeArray(analysis.retainedPaperIds),
    searchStats: {
      query: analysis.searchStats?.query ?? '',
      originalQuery: analysis.searchStats?.originalQuery ?? '',
      rewrittenQueries: normalizeArray(analysis.searchStats?.rewrittenQueries),
      queryHits: (analysis.searchStats?.queryHits as Record<string, number> | undefined) ?? {},
      rawCount: analysis.searchStats?.rawCount ?? 0,
      dedupCount: analysis.searchStats?.dedupCount ?? 0,
      finalCount: analysis.searchStats?.finalCount ?? 0,
    },
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
    summary: normalizeDeepStartSummary(detail.summary),
    messages: normalizeArray(detail.messages),
    currentResults: normalizeArray(detail.currentResults).map((paper) => normalizeSearchPaper(paper)),
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
    sync: normalizeSyncSettings(config?.sync),
  };
}

function normalizeSyncSettings(settings: Partial<SyncSettings> | null | undefined): SyncSettings {
  const conflictResolution = settings?.conflictResolution || defaultConfig.sync.conflictResolution;
  return {
    ...defaultConfig.sync,
    ...settings,
    syncInterval: Math.max(1, Number(settings?.syncInterval || defaultConfig.sync.syncInterval)),
    conflictResolution: ['timestamp', 'local', 'remote', 'manual'].includes(conflictResolution)
      ? conflictResolution
      : defaultConfig.sync.conflictResolution,
  };
}

function normalizeOptionalTimestamp(value: unknown): string | undefined {
  if (!value) {
    return undefined;
  }
  if (typeof value === 'string') {
    return value;
  }
  const date = new Date(value as string | number | Date);
  if (Number.isNaN(date.getTime())) {
    return undefined;
  }
  return date.toISOString();
}

function normalizeDatabaseRestoreStatus(
  status: Partial<DatabaseRestoreStatus> | null | undefined,
): DatabaseRestoreStatus {
  return {
    pending: Boolean(status?.pending),
    applied: Boolean(status?.applied),
    stagedPath: status?.stagedPath ?? '',
    backupPath: status?.backupPath ?? '',
    remotePath: status?.remotePath ?? '',
    scheduledAt: normalizeOptionalTimestamp(status?.scheduledAt),
    appliedAt: normalizeOptionalTimestamp(status?.appliedAt),
    message: sanitizeUserVisibleError(status?.message ?? ''),
  };
}

function normalizeSecretPrefill(
  prefill: Partial<ConfigSecretPrefill> | null | undefined,
): ConfigSecretPrefill {
  return {
    strongLLMApiKey: '',
    hasStrongLLMApiKey: Boolean(prefill?.hasStrongLLMApiKey || prefill?.strongLLMApiKey?.trim()),
    weakLLMApiKey: '',
    hasWeakLLMApiKey: Boolean(prefill?.hasWeakLLMApiKey || prefill?.weakLLMApiKey?.trim()),
    baiduToken: '',
    hasBaiduToken: Boolean(prefill?.hasBaiduToken || prefill?.baiduToken?.trim()),
  };
}

function normalizeInitialState(state: Partial<InitialState> | null | undefined): InitialState {
  return {
    ...defaultInitialState,
    ...state,
    config: normalizeConfig(state?.config),
    folders: normalizeArray(state?.folders).map((folder) => normalizeFolder(folder)),
    papers: normalizeArray(state?.papers).map((paper) => normalizePaper(paper)),
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
    errorMessage: sanitizeUserVisibleError(progress?.errorMessage ?? ''),
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
  const status = progress?.status === 'completed' ? 'complete' : progress?.status;
  return {
    total: progress?.total ?? 0,
    completed: progress?.completed ?? 0,
    currentFile: progress?.currentFile ?? '',
    status: status ?? 'idle',
    message: sanitizeUserVisibleError(progress?.message ?? ''),
  };
}

function normalizeSyncRecord(record: Partial<SyncRecord> | null | undefined): SyncRecord {
  return {
    id: record?.id ?? '',
    type: record?.type === 'download' || record?.type === 'conflict' ? record.type : 'upload',
    fileName: record?.fileName ?? '',
    fileSize: Number(record?.fileSize ?? 0) || 0,
    remotePath: record?.remotePath ?? '',
    localPath: record?.localPath ?? '',
    status: record?.status === 'success' || record?.status === 'failed' ? record.status : 'pending',
    message: sanitizeUserVisibleError(record?.message ?? ''),
    errorMessage: sanitizeUserVisibleError(record?.errorMessage ?? ''),
    createdAt: record?.createdAt ?? new Date().toISOString(),
    completedAt: record?.completedAt,
  };
}

const mockFolders: Folder[] = [
  {
    id: 'mock-inbox',
    name: 'Cache',
    parentId: '',
    path: 'Cache',
    isSystem: true,
    createdAt: new Date().toISOString(),
  },
];

let mockConfig: AppConfig = defaultConfig;
let mockPapers: Paper[] = [];
const mockTranslations = new Map<string, TranslationRecord[]>();
let mockDeepStartSessions: DeepStartSessionSummary[] = [];
const mockDeepStartDetails = new Map<string, DeepStartSessionDetail>();

function normalizeFolderPathForMock(value: string): string {
  return value
    .replace(/\\/g, '/')
    .split('/')
    .map((part) => part.trim())
    .filter(Boolean)
    .join('/');
}

function buildMockFolderTree(): FolderNode[] {
  const map = new Map<string, FolderNode>();
  for (const folder of mockFolders) {
    map.set(folder.id, {
      folder: normalizeFolder(folder),
      children: [],
    });
  }

  const roots: FolderNode[] = [];
  for (const folder of mockFolders) {
    const node = map.get(folder.id);
    if (!node) {
      continue;
    }
    const parentId = folder.parentId?.trim();
    if (!parentId) {
      roots.push(node);
      continue;
    }
    const parentNode = map.get(parentId);
    if (parentNode) {
      parentNode.children.push(node);
    } else {
      roots.push(node);
    }
  }
  return roots;
}

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
      refreshToken: '',
      clientId: '',
      clientSecret: '',
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
    publicationVenue: 'arXiv',
    publicationYear: 2024,
    citationCount: 12,
    url: 'https://arxiv.org/abs/2403.10001',
    category: 'code agent',
    tags: ['agent', 'software engineering'],
    source: 'mock',
    institutions: ['OpenAI'],
    keywords: ['agent', 'software engineering', 'tool use'],
    sourceLabel: 'Mock Source',
    enrichmentNote: '',
  },
  {
    id: '2402.20002',
    title: 'Survey of LLM-Based Scientific Reading Assistants',
    authors: 'C. Author, D. Scholar',
    abstract:
      'This survey reviews AI-assisted paper reading systems, translation support, note synthesis and retrieval workflows.',
    year: 2024,
    journal: 'arXiv',
    publicationVenue: 'arXiv',
    publicationYear: 2024,
    citationCount: 18,
    url: 'https://arxiv.org/abs/2402.20002',
    category: 'paper reading',
    tags: ['survey', 'reading assistant'],
    source: 'mock',
    institutions: ['CMU'],
    keywords: ['survey', 'reading assistant'],
    sourceLabel: 'Mock Source',
    enrichmentNote: '',
  },
];

function mockInitialState(): InitialState {
  return {
    ...defaultInitialState,
    config: mockConfig,
    folders: [...mockFolders],
    papers: mockPapers.filter((paper) => paper.folderId === mockFolders[0].id).map((paper) => normalizePaper(paper)),
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
    return normalizeInitialState(await app.GetInitialState());
  }

  return mockInitialState();
}

export async function getSecretPrefill(): Promise<ConfigSecretPrefill> {
  const app = runtimeApp();
  if (app?.GetSecretPrefill) {
    return normalizeSecretPrefill(await app.GetSecretPrefill());
  }

  return emptySecretPrefill;
}

export async function getPDFServiceStatus(): Promise<PDFServiceStatus> {
  const app = runtimeApp();
  if (app?.GetPDFServiceStatus) {
    return app.GetPDFServiceStatus();
  }
  assertMockFallbackAllowed(app, 'GetPDFServiceStatus');
  return { enabled: false, url: '', healthy: false, ready: false, checkedAt: new Date().toISOString(), message: 'PDF service status is only available in the desktop app.' };
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
    return normalizeArray(await app.SearchPapers(query, limit)).map((paper) => normalizeSearchPaper(paper));
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
      papers: normalizeArray(result.papers).map((paper) => normalizeSearchPaper(paper)),
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
    retainedPaperIds: recommendedPaperIds,
    searchStats: {
      query,
      rawCount: results.length,
      dedupCount: results.length,
      finalCount: results.length,
    },
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

export async function cancelDeepStartTask(sessionId: string): Promise<void> {
  const app = runtimeApp();
  if (app?.CancelDeepStartTask) {
    return app.CancelDeepStartTask(sessionId);
  }
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

export async function supplementDeepStartSearch(
  sessionId: string,
  query: string,
  perSourceLimit: number
): Promise<DeepStartSessionDetail> {
  const app = runtimeApp();
  if (app?.SupplementDeepStartSearch) {
    const detail = await app.SupplementDeepStartSearch(sessionId, query, perSourceLimit);
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
  const added = await searchPapers(query, Math.max(20, (Number(perSourceLimit) || 20) * 2));
  const now = new Date().toISOString();
  const mergedById = new Map(existing.currentResults.map((paper) => [paper.id, paper]));
  for (const paper of added) {
    mergedById.set(paper.id, paper);
  }
  const merged = [...mergedById.values()];

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
        content: `补充检索：${query}`,
        createdAt: now,
      },
      {
        id: `mock-msg-assistant-${Date.now()}`,
        sessionId,
        role: 'assistant',
        content: `已补充检索并合并到当前论文池，当前共 ${merged.length} 篇。`,
        createdAt: now,
      },
    ],
    currentResults: merged,
    currentAnalysis: buildMockDeepStartAnalysis(query, merged),
  };
  upsertMockDeepStartSession(detail);
  return detail;
}

export async function undoDeepStartNarrow(sessionId: string): Promise<DeepStartSessionDetail> {
  const app = runtimeApp();
  if (app?.UndoDeepStartNarrow) {
    const detail = await app.UndoDeepStartNarrow(sessionId);
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
  return existing;
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
    return normalizeArray(await app.GetFolders()).map((folder) => normalizeFolder(folder));
  }
  return [...mockFolders].map((folder) => normalizeFolder(folder));
}

export async function createFolder(name: string): Promise<Folder> {
  const app = runtimeApp();
  if (app?.CreateFolder) {
    return normalizeFolder(await app.CreateFolder(name));
  }

  const existing = mockFolders.find((folder) => folder.name === name);
  if (existing) {
    return normalizeFolder(existing);
  }

  const folder: Folder = {
    id: `mock-folder-${Date.now()}`,
    name,
    parentId: '',
    path: normalizeFolderPathForMock(name),
    isSystem: false,
    createdAt: new Date().toISOString(),
  };
  mockFolders.push(folder);
  return normalizeFolder(folder);
}

export async function getFolderTree(): Promise<FolderNode[]> {
  const app = runtimeApp();
  if (app?.GetFolderTree) {
    return normalizeArray(await app.GetFolderTree()).map((node) => normalizeFolderNode(node));
  }
  return buildMockFolderTree();
}

export async function createFolderNode(request: CreateFolderNodeRequest): Promise<Folder> {
  const app = runtimeApp();
  if (app?.CreateFolderNode) {
    return normalizeFolder(await app.CreateFolderNode(request));
  }

  const parentId = (request.parentId ?? '').trim();
  const pathInput = (request.path ?? '').trim();
  const nameInput = (request.name ?? '').trim();

  const parent = parentId ? mockFolders.find((folder) => folder.id === parentId) : undefined;
  const parentPath = parent?.path?.trim() ?? '';
  const relativePath = pathInput || nameInput;
  const normalizedPath = normalizeFolderPathForMock(relativePath);
  if (!normalizedPath) {
    throw new Error('文件夹名称不能为空');
  }

  const finalPath = parentPath ? normalizeFolderPathForMock(`${parentPath}/${normalizedPath}`) : normalizedPath;
  const finalName = finalPath.split('/').pop() || normalizedPath;
  const existing = mockFolders.find((folder) => folder.path === finalPath);
  if (existing) {
    return normalizeFolder(existing);
  }

  const folder: Folder = {
    id: `mock-folder-${Date.now()}`,
    name: finalName,
    parentId: parentId || '',
    path: finalPath,
    isSystem: false,
    createdAt: new Date().toISOString(),
  };
  mockFolders.push(folder);
  return normalizeFolder(folder);
}

export async function renameFolderNode(request: RenameFolderNodeRequest): Promise<Folder> {
  const app = runtimeApp();
  if (app?.RenameFolderNode) {
    return normalizeFolder(await app.RenameFolderNode(request));
  }

  const folderId = (request.folderId ?? '').trim();
  const name = (request.name ?? '').trim();
  const target = mockFolders.find((folder) => folder.id === folderId);
  if (!target) {
    throw new Error('文件夹不存在');
  }
  if (target.isSystem) {
    throw new Error('系统目录不可重命名');
  }

  const normalizedName = normalizeFolderPathForMock(name);
  if (!normalizedName || normalizedName.includes('/')) {
    throw new Error('文件夹名称不能为空');
  }
  const oldPath = normalizeFolderPathForMock(target.path || target.name);
  const parent = target.parentId ? mockFolders.find((folder) => folder.id === target.parentId) : undefined;
  const parentPath = parent ? normalizeFolderPathForMock(parent.path || parent.name) : '';
  const newPath = parentPath ? normalizeFolderPathForMock(`${parentPath}/${normalizedName}`) : normalizedName;
  const duplicate = mockFolders.find((folder) => folder.id !== folderId && normalizeFolderPathForMock(folder.path) === newPath);
  if (duplicate) {
    throw new Error('文件夹路径已存在');
  }

  for (const folder of mockFolders) {
    const currentPath = normalizeFolderPathForMock(folder.path || folder.name);
    if (folder.id === folderId) {
      folder.name = normalizedName;
      folder.path = newPath;
    } else if (currentPath.startsWith(`${oldPath}/`)) {
      folder.path = normalizeFolderPathForMock(`${newPath}/${currentPath.slice(oldPath.length + 1)}`);
    }
  }

  return normalizeFolder(target);
}

export async function moveFolderNode(request: MoveFolderNodeRequest): Promise<Folder> {
  const app = runtimeApp();
  if (app?.MoveFolderNode) {
    return normalizeFolder(await app.MoveFolderNode(request));
  }

  const folderId = (request.folderId ?? '').trim();
  const parentId = (request.parentId ?? '').trim();
  const target = mockFolders.find((folder) => folder.id === folderId);
  if (!target) {
    throw new Error('文件夹不存在');
  }
  if (target.isSystem) {
    throw new Error('系统目录不可移动');
  }
  if (folderId === parentId) {
    throw new Error('文件夹不能移动到自身');
  }
  const oldPath = normalizeFolderPathForMock(target.path || target.name);
  const parent = parentId ? mockFolders.find((folder) => folder.id === parentId) : undefined;
  if (parentId && !parent) {
    throw new Error('目标父文件夹不存在');
  }
  const parentPath = parent ? normalizeFolderPathForMock(parent.path || parent.name) : '';
  if (parentPath === oldPath || parentPath.startsWith(`${oldPath}/`)) {
    throw new Error('文件夹不能移动到其子目录');
  }

  const newPath = parentPath ? normalizeFolderPathForMock(`${parentPath}/${target.name}`) : normalizeFolderPathForMock(target.name);
  const duplicate = mockFolders.find((folder) => folder.id !== folderId && normalizeFolderPathForMock(folder.path) === newPath);
  if (duplicate) {
    throw new Error('文件夹路径已存在');
  }

  for (const folder of mockFolders) {
    const currentPath = normalizeFolderPathForMock(folder.path || folder.name);
    if (folder.id === folderId) {
      folder.parentId = parentId;
      folder.path = newPath;
    } else if (currentPath.startsWith(`${oldPath}/`)) {
      folder.path = normalizeFolderPathForMock(`${newPath}/${currentPath.slice(oldPath.length + 1)}`);
    }
  }

  return normalizeFolder(target);
}

export async function deleteFolderNode(folderId: string): Promise<void> {
  const app = runtimeApp();
  if (app?.DeleteFolderNode) {
    return app.DeleteFolderNode(folderId);
  }

  const target = mockFolders.find((folder) => folder.id === folderId);
  if (!target) {
    return;
  }
  if (target.isSystem) {
    throw new Error('系统目录不可删除');
  }

  const idsToDelete = new Set<string>();
  const walk = (id: string) => {
    idsToDelete.add(id);
    for (const folder of mockFolders) {
      if ((folder.parentId ?? '') === id) {
        walk(folder.id);
      }
    }
  };
  walk(folderId);

  mockPapers = mockPapers.filter((paper) => !idsToDelete.has(paper.folderId));
  for (const id of [...idsToDelete]) {
    mockTranslations.delete(id);
  }
  for (let i = mockFolders.length - 1; i >= 0; i -= 1) {
    if (idsToDelete.has(mockFolders[i].id)) {
      mockFolders.splice(i, 1);
    }
  }
}

export async function getPapers(folderId: string): Promise<Paper[]> {
  const app = runtimeApp();
  if (app?.GetPapers) {
    return normalizeArray(await app.GetPapers(folderId)).map((paper) => normalizePaper(paper));
  }
  return mockPapers.filter((paper) => paper.folderId === folderId).map((paper) => normalizePaper(paper));
}

export async function importPapers(folderId: string, papers: SearchPaper[]): Promise<Paper[]> {
  const app = runtimeApp();
  if (app?.ImportPapers) {
    return normalizeArray(await app.ImportPapers(folderId, papers)).map((paper) => normalizePaper(paper));
  }

  const imported = papers.map<Paper>((paper) => ({
    id: paper.id,
    sourcePaperId: paper.id,
    title: paper.title,
    authors: paper.authors,
    abstract: paper.abstract,
    year: paper.year,
    journal: paper.journal,
    url: paper.url,
    downloadStatus: 'queued',
    downloadError: '',
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

export async function importPapersWithAssets(
  folderId: string,
  papers: SearchPaper[],
): Promise<ImportPapersWithAssetsResult> {
  const app = runtimeApp();
  if (app?.ImportPapersWithAssets) {
    return normalizeImportWithAssetsResult(await app.ImportPapersWithAssets(folderId, papers));
  }

  const imported = await importPapers(folderId, papers);
  return normalizeImportWithAssetsResult({
    imported,
    queued: imported.length,
    skipped: [],
    message: imported.length > 0 ? `已导入 ${imported.length} 篇论文（演示模式）` : '',
  });
}

export async function retryPaperDownload(paperId: string): Promise<void> {
  const app = runtimeApp();
  if (app?.RetryPaperDownload) {
    return app.RetryPaperDownload(paperId);
  }
}

export async function retryPaperDownloadWithURL(paperId: string, manualURL: string): Promise<void> {
  const normalized = manualURL.trim();
  if (!/^https?:\/\//i.test(normalized)) {
    throw new Error('请填写有效的 http/https PDF 链接');
  }

  const app = runtimeApp();
  if (app?.RetryPaperDownloadWithURL) {
    return app.RetryPaperDownloadWithURL(paperId, normalized);
  }

  const now = new Date().toISOString();
  mockPapers = mockPapers.map((paper) =>
    paper.id === paperId
      ? {
          ...paper,
          url: normalized,
          downloadStatus: 'queued',
          downloadError: '',
          updatedAt: now,
        }
      : paper,
  );
}

export async function retryFolderPendingDownloads(folderId: string): Promise<number> {
  const app = runtimeApp();
  if (app?.RetryFolderPendingDownloads) {
    return app.RetryFolderPendingDownloads(folderId);
  }
  const now = new Date().toISOString();
  let queued = 0;
  mockPapers = mockPapers.map((paper) => {
    if (paper.folderId !== folderId || paper.downloadStatus === 'downloaded') {
      return paper;
    }
    queued += 1;
    return { ...paper, downloadStatus: 'queued', downloadError: '', updatedAt: now };
  });
  return queued;
}

export async function selectAndAttachPaperPDF(paperId: string): Promise<Paper> {
  const app = runtimeApp();
  if (app?.SelectAndAttachPaperPDF) {
    return normalizePaper(await app.SelectAndAttachPaperPDF(paperId));
  }
  throw new Error('当前运行环境不支持选择本地 PDF');
}

export async function getLocalStorageOverview(): Promise<LocalStorageOverview> {
  const app = runtimeApp();
  if (app?.GetLocalStorageOverview) {
    return normalizeLocalStorageOverview(await app.GetLocalStorageOverview());
  }

  return normalizeLocalStorageOverview({
    rootPath: '',
    totalFolders: mockFolders.length,
    totalFiles: mockPapers.filter((paper) => paper.pdfPath).length,
    queued: mockPapers.filter((paper) => paper.downloadStatus === 'queued').length,
    downloading: mockPapers.filter((paper) => paper.downloadStatus === 'downloading').length,
    downloaded: mockPapers.filter((paper) => paper.downloadStatus === 'downloaded').length,
    failed: mockPapers.filter((paper) => paper.downloadStatus === 'failed').length,
    folders: mockFolders.map((folder) => {
      const folderPapers = mockPapers.filter((paper) => paper.folderId === folder.id);
      return {
        folderId: folder.id,
        folderName: folder.name,
        folderPath: '',
        totalPapers: folderPapers.length,
        queued: folderPapers.filter((paper) => paper.downloadStatus === 'queued').length,
        downloading: folderPapers.filter((paper) => paper.downloadStatus === 'downloading').length,
        downloaded: folderPapers.filter((paper) => paper.downloadStatus === 'downloaded').length,
        failed: folderPapers.filter((paper) => paper.downloadStatus === 'failed').length,
        storedFileCount: folderPapers.filter((paper) => paper.pdfPath).length,
      };
    }),
  });
}

export async function getFolderStorageTreeOverview(): Promise<FolderStorageTreeOverview> {
  const app = runtimeApp();
  if (app?.GetFolderStorageTreeOverview) {
    return normalizeFolderStorageTreeOverview(await app.GetFolderStorageTreeOverview());
  }

  const nodeById = new Map<string, FolderStorageTreeOverview['directories'][number]>();
  const makeNode = (folder: Folder): FolderStorageTreeOverview['directories'][number] => {
    const papers = mockPapers.filter((paper) => paper.folderId === folder.id);
    return {
      folderId: folder.id,
      folderName: folder.name,
      folderPath: folder.path,
      paperCount: papers.length,
      queued: papers.filter((paper) => paper.downloadStatus === 'queued').length,
      downloading: papers.filter((paper) => paper.downloadStatus === 'downloading').length,
      downloaded: papers.filter((paper) => paper.downloadStatus === 'downloaded').length,
      failed: papers.filter((paper) => paper.downloadStatus === 'failed').length,
      children: [],
    };
  };
  for (const folder of mockFolders) {
    nodeById.set(folder.id, makeNode(folder));
  }

  const roots: FolderStorageTreeOverview['directories'] = [];
  for (const folder of mockFolders) {
    const node = nodeById.get(folder.id);
    if (!node) {
      continue;
    }
    if (!folder.parentId) {
      roots.push(node);
      continue;
    }
    const parentNode = nodeById.get(folder.parentId);
    if (parentNode) {
      parentNode.children.push(node);
    } else {
      roots.push(node);
    }
  }

  const aggregate = (node: FolderStorageTreeOverview['directories'][number]) => {
    for (const child of node.children) {
      aggregate(child);
      node.paperCount += child.paperCount;
      node.queued += child.queued;
      node.downloading += child.downloading;
      node.downloaded += child.downloaded;
      node.failed += child.failed;
    }
  };
  for (const root of roots) {
    aggregate(root);
  }

  return normalizeFolderStorageTreeOverview({
    rootPath: '',
    directories: roots,
    generatedAt: new Date().toISOString(),
  });
}

export async function deletePaper(id: string): Promise<void> {
  const app = runtimeApp();
  if (app?.DeletePaper) {
    return app.DeletePaper(id);
  }

  mockPapers = mockPapers.filter((paper) => paper.id !== id);
  mockTranslations.delete(id);
}

export async function movePaperToFolder(paperId: string, targetFolderId: string): Promise<Paper> {
  const app = runtimeApp();
  if (app?.MovePaperToFolder) {
    return normalizePaper(await app.MovePaperToFolder(paperId, targetFolderId));
  }
  assertMockFallbackAllowed(app, 'MovePaperToFolder');

  const paper = mockPapers.find((item) => item.id === paperId);
  if (!paper) {
    throw new Error('论文不存在');
  }
  const targetFolder = mockFolders.find((folder) => folder.id === targetFolderId);
  if (!targetFolder) {
    throw new Error('文件夹不存在');
  }
  if (paper.folderId === targetFolderId) {
    return normalizePaper(paper);
  }
  const sourcePaperId = (paper.sourcePaperId ?? '').trim();
  if (sourcePaperId) {
    const duplicate = mockPapers.find(
      (item) => item.id !== paperId && item.folderId === targetFolderId && item.sourcePaperId === sourcePaperId,
    );
    if (duplicate) {
      throw new Error('目标文件夹已存在这篇论文');
    }
  }

  const updated: Paper = {
    ...paper,
    folderId: targetFolderId,
    updatedAt: new Date().toISOString(),
  };
  mockPapers = mockPapers.map((item) => (item.id === paperId ? updated : item));
  return normalizePaper(updated);
}

export async function movePapersToFolder(paperIds: string[], targetFolderId: string): Promise<Paper[]> {
  const app = runtimeApp();
  if (app?.MovePapersToFolder) {
    return normalizeArray(await app.MovePapersToFolder(paperIds, targetFolderId)).map((paper) => normalizePaper(paper));
  }
  assertMockFallbackAllowed(app, 'MovePapersToFolder');

  const ids = [...new Set(paperIds.map((id) => id.trim()).filter(Boolean))];
  if (ids.length === 0) {
    return [];
  }
  const targetFolder = mockFolders.find((folder) => folder.id === targetFolderId);
  if (!targetFolder) {
    throw new Error('文件夹不存在');
  }

  const selectedIds = new Set(ids);
  const papersToMove = ids.map((id) => {
    const paper = mockPapers.find((item) => item.id === id);
    if (!paper) {
      throw new Error('论文不存在');
    }
    return paper;
  });
  const sourceOwners = new Map<string, string>();
  for (const paper of papersToMove) {
    const sourcePaperId = (paper.sourcePaperId ?? '').trim();
    if (!sourcePaperId) {
      continue;
    }
    const existingSelection = sourceOwners.get(sourcePaperId);
    if (existingSelection && existingSelection !== paper.id) {
      throw new Error('选中的论文包含重复来源');
    }
    sourceOwners.set(sourcePaperId, paper.id);
    const duplicate = mockPapers.find(
      (item) => !selectedIds.has(item.id) && item.folderId === targetFolderId && item.sourcePaperId === sourcePaperId,
    );
    if (duplicate) {
      throw new Error('目标文件夹已存在这篇论文');
    }
  }

  const now = new Date().toISOString();
  const movedById = new Map<string, Paper>();
  for (const paper of papersToMove) {
    movedById.set(paper.id, {
      ...paper,
      folderId: targetFolderId,
      updatedAt: now,
    });
  }
  mockPapers = mockPapers.map((paper) => movedById.get(paper.id) ?? paper);
  return ids.map((id) => normalizePaper(movedById.get(id)));
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

export async function getDeepReadState(paperId: string): Promise<DeepReadState> {
  const app = runtimeApp();
  if (app?.GetDeepReadState) {
    return normalizeDeepReadState(await app.GetDeepReadState(paperId));
  }

  const selected = mockPapers.find((paper) => paper.id === paperId);
  return normalizeDeepReadState({
    paperId,
    hasPdf: Boolean(selected?.pdfPath),
    pdfPath: selected?.pdfPath ?? '',
    parseStatus: selected?.pdfPath ? 'ready' : 'missing_pdf',
    parseError: selected?.pdfPath ? '' : '演示模式：当前论文没有本地 PDF。',
    sections: [
      {
        id: 'section-1',
        title: 'Abstract',
        content: selected?.abstract ?? '',
        index: 0,
      },
    ],
    translations: mockTranslations.get(paperId) ?? [],
    notes: [],
    lastPreparedAt: new Date().toISOString(),
  });
}

export async function prepareDeepReadPaper(paperId: string): Promise<DeepReadState> {
  const app = runtimeApp();
  if (app?.PrepareDeepReadPaper) {
    return normalizeDeepReadState(await app.PrepareDeepReadPaper(paperId));
  }
  return getDeepReadState(paperId);
}

export async function saveDeepReadNote(
  paperId: string,
  section: string,
  content: string,
): Promise<DeepReadNote> {
  const app = runtimeApp();
  if (app?.SaveDeepReadNote) {
    return normalizeDeepReadNote(await app.SaveDeepReadNote(paperId, section, content));
  }

  return normalizeDeepReadNote({
    id: `mock-note-${Date.now()}`,
    paperId,
    section,
    content,
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
  });
}

export async function askDeepReadPaper(
  paperId: string,
  sectionId: string,
  question: string,
  mode: 'question' | 'summary',
): Promise<DeepReadAIResponse> {
  const app = runtimeApp();
  if (app?.AskDeepReadPaper) {
    return normalizeDeepReadAIResponse(
      await app.AskDeepReadPaper(paperId, sectionId, question, mode),
    );
  }
  assertMockFallbackAllowed(app, 'AskDeepReadPaper');

  const state = await getDeepReadState(paperId);
  const section = state.sections.find((item) => item.id === sectionId) ?? state.sections[0];
  return normalizeDeepReadAIResponse({
    mode,
    answer:
      mode === 'summary'
        ? `演示模式总结：${section?.content || '当前没有可总结的正文。'}`
        : `演示模式回答：问题“${question}”需要在桌面应用中连接强模型后根据论文全文回答。`,
    takeaway: mode === 'summary' ? '这是浏览器预览数据，不代表真实论文分析。' : '请在桌面应用中验证回答。',
    evidence: section
      ? [{ sectionId: section.id, sectionTitle: section.title, excerpt: section.content.slice(0, 180) }]
      : [],
    limitations: ['浏览器预览没有连接真实 LLM。'],
  });
}

export async function getDeepReadPDFBytes(paperId: string): Promise<string> {
  const app = runtimeApp();
  if (app?.GetDeepReadPDFBytes) {
    return app.GetDeepReadPDFBytes(paperId);
  }
  throw new Error('当前运行环境不支持直接读取本地 PDF 字节流');
}

export async function getDeepReadPDFURL(paperId: string): Promise<string> {
  const app = runtimeApp();
  if (app?.GetDeepReadPDFURL) {
    return app.GetDeepReadPDFURL(paperId);
  }
  return '';
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

export function hasNativeFilePicker(): boolean {
  return Boolean(runtimeApp()?.SelectScreeningPDFs);
}

export async function selectScreeningPDFs(): Promise<string[]> {
  const app = runtimeApp();
  if (app?.SelectScreeningPDFs) {
    return normalizeArray(await app.SelectScreeningPDFs());
  }
  assertMockFallbackAllowed(app, 'SelectScreeningPDFs');
  return [];
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
  };
}

export function onSyncProgress(callback: (progress: SyncProgress) => void): () => void {
  if (!hasWailsRuntime()) {
    return () => undefined;
  }

  const unsubscribe = EventsOn('sync-progress', (progress: SyncProgress) => {
    callback(normalizeSyncProgress(progress));
  });

  return () => {
    unsubscribe?.();
  };
}

function normalizeSearchProgressEvent(progress: Partial<SearchProgressEvent> | null | undefined): SearchProgressEvent {
  const totalSeconds = Number(progress?.totalSeconds ?? 60) || 60;
  const elapsedSeconds = Math.max(0, Number(progress?.elapsedSeconds ?? 0) || 0);
  const sources = normalizeArray(progress?.sources).map((source) => ({
    name: source.name ?? '',
    attempt: Number(source.attempt ?? 0) || 0,
    maxAttempts: Number(source.maxAttempts ?? 60) || 60,
    status: source.status ?? 'pending',
    success: Boolean(source.success),
    done: Boolean(source.done),
    resultCount: Number(source.resultCount ?? 0) || 0,
    error: sanitizeUserVisibleError(source.error ?? ''),
  }));

  return {
    query: progress?.query ?? '',
    elapsedSeconds: Math.min(elapsedSeconds, totalSeconds),
    totalSeconds,
    completedSources: Number(progress?.completedSources ?? 0) || 0,
    totalSources: Number(progress?.totalSources ?? sources.length) || sources.length,
    sources,
    phase: progress?.phase === 'completed' ? 'completed' : 'searching',
    message: sanitizeUserVisibleError(progress?.message ?? ''),
  };
}

function normalizeDeepStartProgressEvent(
  progress: Partial<DeepStartProgressEvent> | null | undefined,
): DeepStartProgressEvent {
  const total = Number(progress?.total ?? 0) || 0;
  const completed = Math.min(total, Math.max(0, Number(progress?.completed ?? 0) || 0));

  return {
    sessionId: progress?.sessionId ?? '',
    phase:
      progress?.phase === 'enriching' ||
      progress?.phase === 'downloading' ||
      progress?.phase === 'parsing' ||
      progress?.phase === 'weak_extracting' ||
      progress?.phase === 'initial_batch_ready' ||
      progress?.phase === 'background_processing' ||
      progress?.phase === 'analyzing' ||
      progress?.phase === 'persisting' ||
      progress?.phase === 'cancelling' ||
      progress?.phase === 'cancelled' ||
      progress?.phase === 'failed' ||
      progress?.phase === 'completed'
        ? progress.phase
        : 'searching',
    message: sanitizeUserVisibleError(progress?.message ?? ''),
    elapsedSeconds: Math.max(0, Number(progress?.elapsedSeconds ?? 0) || 0),
    estimatedRemainingSeconds: Math.max(0, Number(progress?.estimatedRemainingSeconds ?? 0) || 0),
    total,
    completed,
    overallPercent: Math.max(0, Math.min(100, Number(progress?.overallPercent ?? 0) || 0)),
    successCount: Math.max(0, Number(progress?.successCount ?? 0) || 0),
    failedCount: Math.max(0, Number(progress?.failedCount ?? 0) || 0),
    noPdfUrlCount: Math.max(0, Number(progress?.noPdfUrlCount ?? 0) || 0),
    downloadedCount: Math.max(0, Number(progress?.downloadedCount ?? 0) || 0),
    parsedCount: Math.max(0, Number(progress?.parsedCount ?? 0) || 0),
    extractedCount: Math.max(0, Number(progress?.extractedCount ?? 0) || 0),
    initialBatchTotal: Math.max(0, Number(progress?.initialBatchTotal ?? 0) || 0),
    initialBatchCompleted: Math.max(0, Number(progress?.initialBatchCompleted ?? 0) || 0),
    backgroundCompleted: Math.max(0, Number(progress?.backgroundCompleted ?? 0) || 0),
    stats: progress?.stats
      ? {
          query: progress.stats.query ?? '',
          originalQuery: progress.stats.originalQuery ?? '',
          rewrittenQueries: normalizeArray(progress.stats.rewrittenQueries),
          queryHits: (progress.stats.queryHits as Record<string, number> | undefined) ?? {},
          rawCount: Number(progress.stats.rawCount ?? 0) || 0,
          dedupCount: Number(progress.stats.dedupCount ?? 0) || 0,
          finalCount: Number(progress.stats.finalCount ?? 0) || 0,
        }
      : undefined,
  };
}

export function onSearchProgress(callback: (progress: SearchProgressEvent) => void): () => void {
  if (!hasWailsRuntime()) {
    return () => undefined;
  }

  const unsubscribe = EventsOn('search-progress', (progress: SearchProgressEvent) => {
    callback(normalizeSearchProgressEvent(progress));
  });

  return () => {
    unsubscribe?.();
  };
}

export function onDeepStartProgress(callback: (progress: DeepStartProgressEvent) => void): () => void {
  if (!hasWailsRuntime()) {
    return () => undefined;
  }

  const unsubscribe = EventsOn('deepstart-progress', (progress: DeepStartProgressEvent) => {
    callback(normalizeDeepStartProgressEvent(progress));
  });

  return () => {
    unsubscribe?.();
  };
}

export async function createScreeningSession(title: string): Promise<ScreeningSession> {
  const app = runtimeApp();
  if (app?.CreateScreeningSession) {
    return app.CreateScreeningSession(title);
  }
  assertMockFallbackAllowed(app, 'CreateScreeningSession');
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
  assertMockFallbackAllowed(app, 'UploadScreeningFiles');

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
  assertMockFallbackAllowed(app, 'ExtractPaperContent');

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
  assertMockFallbackAllowed(app, 'GetExtractProgress');
  return normalizeExtractProgress({ sessionId, total: 0, completed: 0, currentFile: '', status: 'processing' });
}

export async function analyzePapers(sessionId: string): Promise<ScreeningDecisionNode> {
  const app = runtimeApp();
  if (app?.AnalyzePapers) {
    return normalizeScreeningDecisionNode(await app.AnalyzePapers(sessionId)) as ScreeningDecisionNode;
  }
  assertMockFallbackAllowed(app, 'AnalyzePapers');

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
  assertMockFallbackAllowed(app, 'ApplyScreeningChoice');

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
    return normalizeArray(await app.CompleteScreening(sessionId, targetFolderId)).map((paper) => normalizePaper(paper));
  }
  assertMockFallbackAllowed(app, 'CompleteScreening');
  return [];
}

export async function listScreeningSessions(): Promise<ScreeningSession[]> {
  const app = runtimeApp();
  if (app?.ListScreeningSessions) {
    return normalizeArray(await app.ListScreeningSessions());
  }
  assertMockFallbackAllowed(app, 'ListScreeningSessions');
  return [];
}

export async function getScreeningSession(sessionId: string): Promise<ScreeningSessionDetail> {
  const app = runtimeApp();
  if (app?.GetScreeningSession) {
    return normalizeScreeningSessionDetail(await app.GetScreeningSession(sessionId));
  }
  assertMockFallbackAllowed(app, 'GetScreeningSession');
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
  assertMockFallbackAllowed(app, 'CancelScreening');
}

// ============ Sync API ============

export async function getSyncStatus(): Promise<SyncStatus> {
  const app = runtimeApp();
  if (app?.GetSyncStatus) {
    return normalizeSyncStatus(await app.GetSyncStatus());
  }
  assertMockFallbackAllowed(app, 'GetSyncStatus');
  return normalizeSyncStatus({ enabled: false, provider: 'baidu_cloud', lastSync: null });
}

export async function triggerSync(): Promise<SyncProgress> {
  const app = runtimeApp();
  if (app?.TriggerSync) {
    return normalizeSyncProgress(await app.TriggerSync());
  }
  assertMockFallbackAllowed(app, 'TriggerSync');
  return normalizeSyncProgress({ status: 'complete', total: 0, completed: 0, currentFile: '' });
}

export async function getSyncProgress(): Promise<SyncProgress> {
  const app = runtimeApp();
  if (app?.GetSyncProgress) {
    return normalizeSyncProgress(await app.GetSyncProgress());
  }
  assertMockFallbackAllowed(app, 'GetSyncProgress');
  return normalizeSyncProgress({ total: 0, completed: 0, currentFile: '', status: 'idle' });
}

export async function refreshBaiduToken(): Promise<BaiduTokenRefreshStatus> {
  const app = runtimeApp();
  if (app?.RefreshBaiduToken) {
    return app.RefreshBaiduToken();
  }
  assertMockFallbackAllowed(app, 'RefreshBaiduToken');
  return { enabled: false, tokenFile: 'baiduyun_token.json', hasAccessToken: false, hasRefreshToken: false, hasClientId: false, hasClientSecret: false, refreshed: false, checkedAt: new Date().toISOString(), message: 'Baidu token refresh is only available in the desktop app.' };
}

export async function getSyncPreview(): Promise<SyncPreview> {
  const app = runtimeApp();
  if (app?.GetSyncPreview) {
    return app.GetSyncPreview();
  }
  assertMockFallbackAllowed(app, 'GetSyncPreview');
  return {
    enabled: false,
    dataPath: '',
    remoteRoot: '',
    tokenFile: 'baiduyun_token.json',
    totalFiles: 0,
    totalBytes: 0,
    databaseBytes: 0,
    paperPdfCount: 0,
    paperPdfBytes: 0,
    otherFiles: 0,
    files: [],
    checkedAt: new Date().toISOString(),
    warning: 'Sync preview is only available in the desktop app.',
  };
}

export async function getSyncConflicts(): Promise<SyncConflict[]> {
  const app = runtimeApp();
  if (app?.GetSyncConflicts) {
    return normalizeArray(await app.GetSyncConflicts());
  }
  assertMockFallbackAllowed(app, 'GetSyncConflicts');
  return [];
}

export async function getSyncRecords(limit = 20): Promise<SyncRecord[]> {
  const app = runtimeApp();
  if (app?.GetSyncRecords) {
    return normalizeArray(await app.GetSyncRecords(limit)).map((record) => normalizeSyncRecord(record));
  }
  assertMockFallbackAllowed(app, 'GetSyncRecords');
  return [];
}

export async function resolveSyncConflict(conflictId: string, resolution: 'local' | 'remote' | 'skipped' | 'timestamp'): Promise<void> {
  const app = runtimeApp();
  if (app?.ResolveSyncConflict) {
    return app.ResolveSyncConflict(conflictId, resolution);
  }
  assertMockFallbackAllowed(app, 'ResolveSyncConflict');
}

export async function getSyncSettings(): Promise<SyncSettings> {
  const app = runtimeApp();
  if (app?.GetSyncSettings) {
    return normalizeSyncSettings(await app.GetSyncSettings());
  }
  assertMockFallbackAllowed(app, 'GetSyncSettings');
  return defaultConfig.sync;
}

export async function saveSyncSettings(settings: SyncSettings): Promise<SyncSettings> {
  const normalized = normalizeSyncSettings(settings);
  const app = runtimeApp();
  if (app?.SaveSyncSettings) {
    return normalizeSyncSettings(await app.SaveSyncSettings(normalized));
  }
  assertMockFallbackAllowed(app, 'SaveSyncSettings');
  return normalized;
}

export async function getPendingDatabaseRestore(): Promise<DatabaseRestoreStatus> {
  const app = runtimeApp();
  if (app?.GetPendingDatabaseRestore) {
    return normalizeDatabaseRestoreStatus(await app.GetPendingDatabaseRestore());
  }
  assertMockFallbackAllowed(app, 'GetPendingDatabaseRestore');
  return normalizeDatabaseRestoreStatus(null);
}

export async function applyPendingDatabaseRestore(): Promise<DatabaseRestoreStatus> {
  const app = runtimeApp();
  if (app?.ApplyPendingDatabaseRestore) {
    return normalizeDatabaseRestoreStatus(await app.ApplyPendingDatabaseRestore());
  }
  assertMockFallbackAllowed(app, 'ApplyPendingDatabaseRestore');
  return normalizeDatabaseRestoreStatus({ pending: false, applied: false });
}

export async function cancelPendingDatabaseRestore(): Promise<void> {
  const app = runtimeApp();
  if (app?.CancelPendingDatabaseRestore) {
    return app.CancelPendingDatabaseRestore();
  }
  assertMockFallbackAllowed(app, 'CancelPendingDatabaseRestore');
}
