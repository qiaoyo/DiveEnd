export interface LLMConfig {
  providerId: string;
  providerName: string;
  providerType: 'openai_compatible' | 'anthropic';
  baseUrl: string;
  wireApi: 'responses' | 'chat_completions' | 'anthropic_messages';
  requiresOpenAIAuth: boolean;
  apiKey: string;
  hasApiKey: boolean;
  model: string;
  reasoningEffort: string;
  disableResponseStorage: boolean;
  clearApiKey: boolean;
}

export interface SearchAPIConfig {
  enableSemanticScholar: boolean;
  enableArxiv: boolean;
  semanticScholarKeyPath: string;
  perSourceResultLimit: number;
  deepStartResultLimit: number;
  retryDurationSeconds: number;
  retryIntervalSeconds: number;
  requestTimeoutSeconds: number;
  semanticScholarApiKey: string;
  hasSemanticScholarApiKey: boolean;
  clearSemanticScholarApiKey: boolean;
}

export interface BaiduCloudConfig {
  enabled: boolean;
  token: string;
  refreshToken?: string;
  clientId?: string;
  clientSecret?: string;
  hasToken: boolean;
  quota: number;
  clearToken: boolean;
}

export interface AppConfig {
  llm: LLMConfig;
  weakLLM: LLMConfig;
  search: SearchAPIConfig;
  baiduCloud: BaiduCloudConfig;
  sync: SyncSettings;
  theme: 'light' | 'dark';
  leftPanelWidth: number;
  rightPanelWidth: number;
  dataPath: string;
}

export interface Folder {
  id: string;
  name: string;
  parentId?: string;
  path: string;
  isSystem: boolean;
  createdAt: string;
}

export interface FolderNode {
  folder: Folder;
  children: FolderNode[];
}

export interface CreateFolderNodeRequest {
  parentId?: string;
  path?: string;
  name?: string;
}

export interface RenameFolderNodeRequest {
  folderId: string;
  name: string;
}

export interface MoveFolderNodeRequest {
  folderId: string;
  parentId?: string;
}

export interface Paper {
  id: string;
  sourcePaperId: string;
  title: string;
  authors: string;
  abstract: string;
  year: number;
  journal: string;
  url: string;
  pdfPath?: string;
  downloadStatus: 'queued' | 'downloading' | 'downloaded' | 'failed' | string;
  downloadError?: string;
  folderId: string;
  category: string;
  tags: string[];
  addedAt: string;
  updatedAt: string;
}

export interface SearchPaper {
  id: string;
  title: string;
  authors: string;
  abstract: string;
  year: number;
  journal: string;
  publicationVenue: string;
  publicationYear: number;
  citationCount: number;
  url: string;
  category: string;
  tags: string[];
  source: string; // 'semantic_scholar' | 'arxiv' | 'arxiv_sanity'
  externalIds?: Record<string, string>;
  pdfCandidates?: string[];
  institutions: string[];
  keywords: string[];
  sourceLabel: string;
  enrichmentNote?: string;
  preprocessStatus?: string;
  localPdfPath?: string;
  markdownPath?: string;
  parseStatus?: string;
  parseError?: string;
  extractStatus?: string;
  extractError?: string;
  problem?: string;
  method?: string;
  topicLabel?: string;
  methodLabel?: string;
  taskLabel?: string;
  domainLabel?: string;
  classificationConfidence?: number;
  processingStage?: string;
  processingError?: string;
}

export interface ImportSkippedPaper {
  sourcePaperId: string;
  title: string;
  reason: string;
}

export interface ImportPapersWithAssetsResult {
  imported: Paper[];
  skipped: ImportSkippedPaper[];
  queued: number;
  message?: string;
}

export interface LocalStorageFolderOverview {
  folderId: string;
  folderName: string;
  folderPath: string;
  totalPapers: number;
  queued: number;
  downloading: number;
  downloaded: number;
  failed: number;
  storedFileCount: number;
}

export interface LocalStorageOverview {
  rootPath: string;
  totalFolders: number;
  totalFiles: number;
  queued: number;
  downloading: number;
  downloaded: number;
  failed: number;
  folders: LocalStorageFolderOverview[];
  generatedAt: string;
}

export interface FolderStorageTreeNode {
  folderId: string;
  folderName: string;
  folderPath: string;
  paperCount: number;
  queued: number;
  downloading: number;
  downloaded: number;
  failed: number;
  children: FolderStorageTreeNode[];
}

export interface FolderStorageTreeOverview {
  rootPath: string;
  directories: FolderStorageTreeNode[];
  generatedAt: string;
}

export interface SearchSourceStatus {
  name: string;
  success: boolean;
  error?: string;
  count: number;
}

export interface SearchProgressSource {
  name: string;
  attempt: number;
  maxAttempts: number;
  status: 'pending' | 'retrying' | 'success' | 'failed' | string;
  success: boolean;
  done: boolean;
  resultCount: number;
  error?: string;
}

export interface SearchProgressEvent {
  query: string;
  elapsedSeconds: number;
  totalSeconds: number;
  completedSources: number;
  totalSources: number;
  sources: SearchProgressSource[];
  phase: 'searching' | 'completed';
  message?: string;
}

export interface EnhancedSearchResult {
  papers: SearchPaper[];
  total: number;
  hasMore: boolean;
  sources: SearchSourceStatus[];
  query: string;
  limit: number;
  offset: number;
  yearStart?: number;
  yearEnd?: number;
  sortBy?: string;
}

export interface SearchResult {
  ID: string;
  Title: string;
  Authors: string;
  Abstract: string;
  Year: number;
  Journal: string;
  URL: string;
  Source: string;
  Citations: number;
  PDFURL: string;
}

export interface CategoryNode {
  id: string;
  name: string;
  count: number;
  children?: CategoryNode[];
}

export interface SelectionStep {
  id: string;
  dimension: string;
  selected: string[];
  remainingCount: number;
}

export interface DeepStartSessionSummary {
  id: string;
  title: string;
  rootPrompt: string;
  currentQuery: string;
  targetFolderId: string;
  processingStatus?: 'initializing' | 'background_processing' | 'completed' | string;
  initialReadyCount?: number;
  totalPlannedCount?: number;
  backgroundRemaining?: number;
  createdAt: string;
  updatedAt: string;
}

export interface DeepStartMessage {
  id: string;
  sessionId: string;
  role: 'user' | 'assistant';
  content: string;
  createdAt: string;
}

export interface DeepStartDirection {
  id: string;
  name: string;
  summary: string;
  why: string;
  paperIds: string[];
}

export interface DeepStartPaperNote {
  paperId: string;
  tier: 'core' | 'important' | 'optional';
  reason: string;
  directionIds: string[];
}

export interface DeepStartAnalysis {
  overview: string;
  directions: DeepStartDirection[];
  paperNotes: DeepStartPaperNote[];
  followUpQuestions: string[];
  suggestedQueries: string[];
  recommendedPaperIds: string[];
  retainedPaperIds: string[];
  searchStats: {
    query: string;
    originalQuery?: string;
    rewrittenQueries?: string[];
    queryHits?: Record<string, number>;
    rawCount: number;
    dedupCount: number;
    finalCount: number;
  };
}

export interface DeepStartProgressEvent {
  sessionId?: string;
  phase:
    | 'searching'
    | 'enriching'
    | 'downloading'
    | 'parsing'
    | 'weak_extracting'
    | 'initial_batch_ready'
    | 'background_processing'
    | 'analyzing'
    | 'persisting'
    | 'cancelling'
    | 'cancelled'
    | 'failed'
    | 'completed';
  message?: string;
  elapsedSeconds: number;
  estimatedRemainingSeconds: number;
  total: number;
  completed: number;
  overallPercent: number;
  successCount?: number;
  failedCount?: number;
  noPdfUrlCount?: number;
  downloadedCount?: number;
  parsedCount?: number;
  extractedCount?: number;
  initialBatchTotal?: number;
  initialBatchCompleted?: number;
  backgroundCompleted?: number;
  stats?: {
    query: string;
    originalQuery?: string;
    rewrittenQueries?: string[];
    queryHits?: Record<string, number>;
    rawCount: number;
    dedupCount: number;
    finalCount: number;
  };
}

export interface DeepStartSessionDetail {
  summary: DeepStartSessionSummary;
  messages: DeepStartMessage[];
  currentResults: SearchPaper[];
  currentAnalysis: DeepStartAnalysis | null;
  selectedPaperIds: string[];
}

export interface TranslationRecord {
  id: string;
  paperId: string;
  section: string;
  originalText: string;
  translatedText: string;
  summary: string;
  createdAt: string;
  updatedAt: string;
}

export interface DeepReadSection {
  id: string;
  title: string;
  content: string;
  index: number;
}

export interface DeepReadNote {
  id: string;
  paperId: string;
  section: string;
  content: string;
  createdAt: string;
  updatedAt: string;
}

export interface DeepReadState {
  paperId: string;
  hasPdf: boolean;
  pdfPath: string;
  parseStatus: 'idle' | 'preparing' | 'ready' | 'failed' | 'missing_pdf' | string;
  parseError?: string;
  sections: DeepReadSection[];
  markdown?: string;
  translations: TranslationRecord[];
  notes: DeepReadNote[];
  lastPreparedAt?: string;
}

export interface ScreeningPaper {
  id: string;
  sessionId: string;
  fileName: string;
  filePath: string;
  fileSize: number;
  status: 'pending' | 'extracting' | 'extracted' | 'screening' | 'selected' | 'rejected';
  title?: string;
  authors?: string;
  abstract?: string;
  fullText?: string;
  sectionsJson?: string;
  selection?: string;
  reason?: string;
  targetFolderId?: string;
  createdAt: string;
  updatedAt: string;
}

export interface ScreeningSession {
  id: string;
  title: string;
  status: 'upload' | 'extract' | 'screen' | 'complete';
  totalPapers: number;
  currentNodeJson?: string;
  selectedOptionsJson?: string;
  pathHistoryJson?: string;
  createdAt: string;
  updatedAt: string;
}

export interface ScreeningDecisionOption {
  key: string;
  label: string;
  paperIds: string[];
  count: number;
}

export interface ScreeningDecisionNode {
  id: string;
  nodeType: 'branch' | 'complete';
  message: string;
  dimension: string;
  options: ScreeningDecisionOption[];
  allowMultiSelect: boolean;
  allowSkip: boolean;
  remainingPaperIds: string[];
}

export interface PathHistoryItem {
  dimension: string;
  choice: string;
}

export interface ScreeningSessionDetail {
  session: ScreeningSession;
  papers: ScreeningPaper[];
  currentNode: ScreeningDecisionNode | null;
  pathHistory: PathHistoryItem[];
}

export interface ExtractProgress {
  sessionId: string;
  total: number;
  completed: number;
  currentFile: string;
  status: 'processing' | 'completed' | 'error';
  errorMessage?: string;
}

export interface SyncStatus {
  enabled: boolean;
  provider: string;
  lastSync?: string | null;
  syncInProgress: boolean;
  pendingFiles: number;
  conflicts: number;
  totalSynced: number;
  totalFailed: number;
}

export interface SyncSettings {
  autoSync: boolean;
  syncOnStartup: boolean;
  syncBeforeExit: boolean;
  syncInterval: number;
  conflictResolution: 'timestamp' | 'local' | 'remote' | 'manual';
}

export interface PDFServiceStatus {
  enabled: boolean;
  url: string;
  healthy: boolean;
  ready: boolean;
  checks?: Record<string, string>;
  checkedAt: string;
  message?: string;
}

export interface SyncProgress {
  total: number;
  completed: number;
  currentFile: string;
  status: string;
  message?: string;
}

export interface BaiduTokenRefreshStatus {
  enabled: boolean;
  tokenFile: string;
  hasAccessToken: boolean;
  hasRefreshToken: boolean;
  hasClientId: boolean;
  hasClientSecret: boolean;
  refreshed: boolean;
  checkedAt: string;
  message?: string;
}

export interface SyncPreviewFile {
  key: string;
  fileName: string;
  kind: 'database' | 'paper_pdf' | 'other' | string;
  size: number;
  remotePath: string;
}

export interface SyncPreview {
  enabled: boolean;
  dataPath: string;
  remoteRoot: string;
  tokenFile: string;
  totalFiles: number;
  totalBytes: number;
  databaseBytes: number;
  paperPdfCount: number;
  paperPdfBytes: number;
  otherFiles: number;
  files: SyncPreviewFile[];
  checkedAt: string;
  warning?: string;
}

export interface SyncConflict {
  id: string;
  fileName: string;
  fileKind?: string;
  localPath: string;
  localSize: number;
  localTime: string;
  remotePath: string;
  remoteSize: number;
  remoteTime: string;
  newerSide?: 'local' | 'remote' | 'equal' | string;
  resolution?: string;
  resolvedAt?: string;
  createdAt: string;
}

export interface SyncRecord {
  id: string;
  type: 'upload' | 'download' | 'conflict';
  fileName: string;
  fileSize: number;
  remotePath: string;
  localPath: string;
  status: 'pending' | 'success' | 'failed';
  message?: string;
  errorMessage?: string;
  createdAt: string;
  completedAt?: string;
}

export interface DatabaseRestoreStatus {
  pending: boolean;
  applied: boolean;
  stagedPath?: string;
  backupPath?: string;
  remotePath?: string;
  scheduledAt?: string;
  appliedAt?: string;
  message?: string;
}

export interface InitialState {
  config: AppConfig;
  folders: Folder[];
  papers: Paper[];
  activeFolderId: string;
  deepStartSessions: DeepStartSessionSummary[];
  activeDeepStartSession: DeepStartSessionDetail | null;
}

export interface SaveConfigResult {
  config: AppConfig;
  restartRequired: boolean;
}

export interface ConfigSecretPrefill {
  strongLLMApiKey: string;
  hasStrongLLMApiKey: boolean;
  weakLLMApiKey: string;
  hasWeakLLMApiKey: boolean;
  baiduToken: string;
  hasBaiduToken: boolean;
}

export type Panel = 'deepstart' | 'deepread' | 'screening' | 'sync';

export interface AppState {
  activePanel: Panel;
  activeFolderId: string;
  selectedPaper: Paper | null;
  papers: Paper[];
  folders: Folder[];
  deepStartSessions: DeepStartSessionSummary[];
  activeDeepStartSession: DeepStartSessionDetail | null;
  searchQuery: string;
  searchResults: SearchPaper[];
  translations: TranslationRecord[];
  config: AppConfig;
  leftPanelCollapsed: boolean;
  rightPanelCollapsed: boolean;
  theme: 'light' | 'dark';
  isHydrating: boolean;
  isSearching: boolean;
  isSavingConfig: boolean;
  isTranslating: boolean;
  error: string | null;
}

export const defaultConfig: AppConfig = {
  llm: {
    providerId: 'anthropic',
    providerName: 'Anthropic',
    providerType: 'anthropic',
    baseUrl: 'https://api.anthropic.com/v1',
    wireApi: 'anthropic_messages',
    requiresOpenAIAuth: false,
    apiKey: '',
    hasApiKey: false,
    model: 'claude-3-5-sonnet-20241022',
    reasoningEffort: '',
    disableResponseStorage: true,
    clearApiKey: false,
  },
  weakLLM: {
    providerId: 'weak-llm',
    providerName: 'Weak LLM',
    providerType: 'openai_compatible',
    baseUrl: 'https://api.openai.com/v1',
    wireApi: 'responses',
    requiresOpenAIAuth: true,
    apiKey: '',
    hasApiKey: false,
    model: 'gpt-4o-mini',
    reasoningEffort: '',
    disableResponseStorage: true,
    clearApiKey: false,
  },
  search: {
    enableSemanticScholar: true,
    enableArxiv: true,
    semanticScholarKeyPath: 'config/semantic_scholar.json',
    perSourceResultLimit: 100,
    deepStartResultLimit: 200,
    retryDurationSeconds: 60,
    retryIntervalSeconds: 1,
    requestTimeoutSeconds: 5,
    semanticScholarApiKey: '',
    hasSemanticScholarApiKey: false,
    clearSemanticScholarApiKey: false,
  },
  baiduCloud: {
    enabled: false,
    token: '',
    hasToken: false,
    quota: 0,
    clearToken: false,
  },
  sync: {
    autoSync: false,
    syncOnStartup: false,
    syncBeforeExit: false,
    syncInterval: 30,
    conflictResolution: 'timestamp',
  },
  theme: 'light',
  leftPanelWidth: 280,
  rightPanelWidth: 340,
  dataPath: './DiveEndData',
};

export const defaultInitialState: InitialState = {
  config: defaultConfig,
  folders: [],
  papers: [],
  activeFolderId: '',
  deepStartSessions: [],
  activeDeepStartSession: null,
};
