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
  semanticScholarApiKey: string;
  hasSemanticScholarApiKey: boolean;
  clearSemanticScholarApiKey: boolean;
}

export interface BaiduCloudConfig {
  enabled: boolean;
  token: string;
  hasToken: boolean;
  quota: number;
  clearToken: boolean;
}

export interface AppConfig {
  llm: LLMConfig;
  weakLLM: LLMConfig;
  search: SearchAPIConfig;
  baiduCloud: BaiduCloudConfig;
  theme: 'light' | 'dark';
  leftPanelWidth: number;
  rightPanelWidth: number;
  dataPath: string;
}

export interface Folder {
  id: string;
  name: string;
  createdAt: string;
}

export interface Paper {
  id: string;
  title: string;
  authors: string;
  abstract: string;
  year: number;
  journal: string;
  url: string;
  pdfPath?: string;
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
  url: string;
  category: string;
  tags: string[];
  source: string; // 'semantic_scholar' | 'arxiv' | 'arxiv_sanity'
}

export interface SearchSourceStatus {
  name: string;
  success: boolean;
  error?: string;
  count: number;
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
