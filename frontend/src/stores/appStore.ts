import { create } from 'zustand';
import type { AppState, Paper, Config, Panel } from '../types';

const defaultConfig: Config = {
  openaiApiKey: '',
  anthropicApiKey: '',
  selectedModel: 'anthropic',
  semanticScholarApiKey: '',
  baiduCloud: {
    enabled: false,
    token: '',
    quota: 0,
  },
  theme: 'light',
  leftPanelWidth: 240,
  rightPanelWidth: 320,
  dataPath: './data',
};

interface AppStore extends AppState {
  // Actions
  setActivePanel: (panel: Panel) => void;
  setSelectedPaper: (paper: Paper | null) => void;
  setPapers: (papers: Paper[]) => void;
  addPaper: (paper: Paper) => void;
  removePaper: (id: string) => void;
  
  setSearchQuery: (query: string) => void;
  setSearchResults: (papers: Paper[]) => void;
  setIsSearching: (isSearching: boolean) => void;
  
  setIsTranslating: (isTranslating: boolean) => void;
  setTranslationProgress: (progress: number) => void;
  
  setConfig: (config: Partial<Config>) => void;
  
  toggleLeftPanel: () => void;
  toggleRightPanel: () => void;
  toggleTheme: () => void;
  
  setLoading: (isLoading: boolean) => void;
  setError: (error: string | null) => void;
}

export const useAppStore = create<AppStore>((set) => ({
  // Initial state
  activePanel: 'deepstart',
  selectedPaper: null,
  papers: [],
  folders: [],
  searchQuery: '',
  searchResults: [],
  isSearching: false,
  isTranslating: false,
  translationProgress: 0,
  config: defaultConfig,
  leftPanelCollapsed: false,
  rightPanelCollapsed: false,
  theme: 'light',
  isLoading: false,
  error: null,
  
  // Actions
  setActivePanel: (panel) => set({ activePanel: panel }),
  setSelectedPaper: (paper) => set({ selectedPaper: paper }),
  
  setPapers: (papers) => set({ papers }),
  addPaper: (paper) => set((state) => ({ 
    papers: [paper, ...state.papers] 
  })),
  removePaper: (id) => set((state) => ({ 
    papers: state.papers.filter(p => p.id !== id) 
  })),
  
  setSearchQuery: (query) => set({ searchQuery: query }),
  setSearchResults: (papers) => set({ searchResults: papers }),
  setIsSearching: (isSearching) => set({ isSearching }),
  
  setIsTranslating: (isTranslating) => set({ isTranslating }),
  setTranslationProgress: (progress) => set({ translationProgress: progress }),
  
  setConfig: (config) => set((state) => ({ 
    config: { ...state.config, ...config } 
  })),
  
  toggleLeftPanel: () => set((state) => ({ 
    leftPanelCollapsed: !state.leftPanelCollapsed 
  })),
  toggleRightPanel: () => set((state) => ({ 
    rightPanelCollapsed: !state.rightPanelCollapsed 
  })),
  toggleTheme: () => set((state) => ({ 
    theme: state.theme === 'light' ? 'dark' : 'light',
    config: { ...state.config, theme: state.theme === 'light' ? 'dark' : 'light' }
  })),
  
  setLoading: (isLoading) => set({ isLoading }),
  setError: (error) => set({ error }),
}));