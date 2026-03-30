// Paper types
export interface Paper {
  id: string;
  title: string;
  authors: string;
  abstract: string;
  year: number;
  journal: string;
  pdfPath?: string;
  category: string;
  tags: string[];
  addedAt: string;
  updatedAt: string;
}

export interface TranslationCache {
  paperId: string;
  section: string;
  original: string;
  translated: string;
  model: string;
}

export interface Folder {
  id: string;
  name: string;
  parentId?: string;
  createdAt: string;
}

// Config types
export interface Config {
  openaiApiKey: string;
  anthropicApiKey: string;
  selectedModel: 'openai' | 'anthropic';
  semanticScholarApiKey: string;
  baiduCloud: {
    enabled: boolean;
    token: string;
    quota: number;
  };
  theme: 'light' | 'dark';
  leftPanelWidth: number;
  rightPanelWidth: number;
  dataPath: string;
}

// Search types
export interface SearchResult {
  papers: Paper[];
  total: number;
  query: string;
}

// LLM types
export interface LLMRequest {
  prompt: string;
  model?: string;
  stream?: boolean;
}

export interface LLMResponse {
  content: string;
  model: string;
  tokens: number;
}

// UI types
export type Panel = 'deepstart' | 'deepread';

export interface AppState {
  // Current view
  activePanel: Panel;
  selectedPaper: Paper | null;
  
  // Papers
  papers: Paper[];
  folders: Folder[];
  
  // Search
  searchQuery: string;
  searchResults: Paper[];
  isSearching: boolean;
  
  // Reading
  isTranslating: boolean;
  translationProgress: number;
  
  // Config
  config: Config;
  
  // UI state
  leftPanelCollapsed: boolean;
  rightPanelCollapsed: boolean;
  theme: 'light' | 'dark';
  
  // Loading states
  isLoading: boolean;
  error: string | null;
}