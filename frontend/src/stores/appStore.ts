import { create } from 'zustand';
import type {
  AppConfig,
  AppState,
  DeepStartSessionDetail,
  DeepStartSessionSummary,
  Folder,
  InitialState,
  Panel,
  Paper,
  SearchPaper,
  TranslationRecord,
} from '../types';
import { defaultConfig } from '../types';
import { sanitizeUserVisibleError } from '../lib/errors';

interface AppStore extends AppState {
  hydrate: (state: InitialState) => void;
  setActivePanel: (panel: Panel) => void;
  setActiveFolderId: (folderId: string) => void;
  setSelectedPaper: (paper: Paper | null) => void;
  setPapers: (papers: Paper[]) => void;
  setFolders: (folders: Folder[]) => void;
  setDeepStartSessions: (sessions: DeepStartSessionSummary[]) => void;
  setActiveDeepStartSession: (session: DeepStartSessionDetail | null) => void;
  upsertDeepStartSession: (session: DeepStartSessionDetail) => void;
  addFolder: (folder: Folder) => void;
  setSearchQuery: (query: string) => void;
  setSearchResults: (papers: SearchPaper[]) => void;
  setTranslations: (translations: TranslationRecord[]) => void;
  prependTranslation: (translation: TranslationRecord) => void;
  setIsSearching: (isSearching: boolean) => void;
  setSavingConfig: (isSavingConfig: boolean) => void;
  setIsTranslating: (isTranslating: boolean) => void;
  setConfig: (config: AppConfig) => void;
  setHydrating: (isHydrating: boolean) => void;
  toggleLeftPanel: () => void;
  toggleRightPanel: () => void;
  setError: (error: string | null) => void;
}

export const useAppStore = create<AppStore>((set) => ({
  activePanel: 'deepstart',
  activeFolderId: '',
  selectedPaper: null,
  papers: [],
  folders: [],
  deepStartSessions: [],
  activeDeepStartSession: null,
  searchQuery: '',
  searchResults: [],
  translations: [],
  config: defaultConfig,
  leftPanelCollapsed: false,
  rightPanelCollapsed: false,
  theme: defaultConfig.theme,
  isHydrating: true,
  isSearching: false,
  isSavingConfig: false,
  isTranslating: false,
  error: null,

  hydrate: (initialState) =>
    set({
      config: initialState.config,
      folders: initialState.folders,
      papers: initialState.papers,
      activeFolderId: initialState.activeFolderId,
      deepStartSessions: initialState.deepStartSessions,
      activeDeepStartSession: initialState.activeDeepStartSession,
      theme: initialState.config.theme,
      isHydrating: false,
      error: null,
    }),

  setActivePanel: (activePanel) => set({ activePanel }),
  setActiveFolderId: (activeFolderId) => set({ activeFolderId }),
  setSelectedPaper: (selectedPaper) => set({ selectedPaper }),
  setPapers: (papers) => set({ papers }),
  setFolders: (folders) => set({ folders }),
  setDeepStartSessions: (deepStartSessions) => set({ deepStartSessions }),
  setActiveDeepStartSession: (activeDeepStartSession) => set({ activeDeepStartSession }),
  upsertDeepStartSession: (session) =>
    set((state) => {
      const nextSummaries = [
        session.summary,
        ...state.deepStartSessions.filter((item) => item.id !== session.summary.id),
      ].sort((a, b) => b.updatedAt.localeCompare(a.updatedAt));

      return {
        deepStartSessions: nextSummaries,
        activeDeepStartSession: session,
      };
    }),
  addFolder: (folder) =>
    set((state) => ({
      folders: state.folders.some((item) => item.id === folder.id)
        ? state.folders
        : [...state.folders, folder],
    })),
  setSearchQuery: (searchQuery) => set({ searchQuery }),
  setSearchResults: (searchResults) => set({ searchResults }),
  setTranslations: (translations) => set({ translations }),
  prependTranslation: (translation) =>
    set((state) => ({
      translations: [translation, ...state.translations],
    })),
  setIsSearching: (isSearching) => set({ isSearching }),
  setSavingConfig: (isSavingConfig) => set({ isSavingConfig }),
  setIsTranslating: (isTranslating) => set({ isTranslating }),
  setConfig: (config) =>
    set({
      config,
      theme: config.theme,
    }),
  setHydrating: (isHydrating) => set({ isHydrating }),
  toggleLeftPanel: () =>
    set((state) => ({
      leftPanelCollapsed: !state.leftPanelCollapsed,
    })),
  toggleRightPanel: () =>
    set((state) => ({
      rightPanelCollapsed: !state.rightPanelCollapsed,
    })),
  setError: (error) => set({ error: error ? sanitizeUserVisibleError(error) : null }),
}));
