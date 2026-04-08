import { Allotment } from 'allotment';
import 'allotment/dist/style.css';
import { useLocation } from 'react-router-dom';
import { useAppStore } from '../../stores/appStore';
import { MainPanel } from '../main/MainPanel';
import { HistoryPanel } from '../history/HistoryPanel';
import { DeepReadPanel } from '../deepread/DeepReadPanel';
import { DeepStartPanel } from '../deepstart/DeepStartPanel';
import { SessionDetailPanel } from '../deepstart/SessionDetailPanel';
import { Screening as ScreeningPanel } from '../../pages/Screening';
import { Sync as SyncPanel } from '../../pages/Sync';
import { PaperListPanel } from '../paperlist/PaperListPanel';
import { SettingsPanel } from '../settings/SettingsPanel';

export function AppLayout() {
  const location = useLocation();
  const {
    config,
    error,
    isHydrating,
    leftPanelCollapsed,
    rightPanelCollapsed,
  } = useAppStore();

  // Determine active panel based on current route
  const getActivePanel = () => {
    const path = location.pathname;
    if (path === '/') return 'main';
    if (path === '/history') return 'history';
    if (path === '/deepstart') return 'deepstart';
    if (path.startsWith('/session/')) return 'session';
    if (path === '/deepread') return 'deepread';
    if (path === '/screening') return 'screening';
    if (path === '/sync') return 'sync';
    return 'main';
  };

  const activePanel = getActivePanel();

  // For main panel, show full-screen without sidebars
  if (activePanel === 'main') {
    return (
      <div className="flex h-full min-h-0 flex-col bg-[#f5f1e8] text-stone-900 dark:bg-[#151515] dark:text-stone-100">
        {error && (
          <div className="border-b border-amber-300 bg-amber-100 px-4 py-2 text-sm text-amber-900 dark:border-amber-800 dark:bg-amber-950/60 dark:text-amber-100">
            {error}
          </div>
        )}
        {isHydrating ? (
          <div className="flex flex-1 items-center justify-center">
            <div className="rounded-2xl border border-stone-200 bg-white/80 px-6 py-4 text-sm shadow-sm dark:border-stone-800 dark:bg-stone-900/80">
              正在加载 DiveEnd 工作区...
            </div>
          </div>
        ) : (
          <MainPanel />
        )}
      </div>
    );
  }

  // For history panel, show full-screen without sidebars
  if (activePanel === 'history') {
    return (
      <div className="flex h-full min-h-0 flex-col bg-[#f5f1e8] text-stone-900 dark:bg-[#151515] dark:text-stone-100">
        {error && (
          <div className="border-b border-amber-300 bg-amber-100 px-4 py-2 text-sm text-amber-900 dark:border-amber-800 dark:bg-amber-950/60 dark:text-amber-100">
            {error}
          </div>
        )}
        {isHydrating ? (
          <div className="flex flex-1 items-center justify-center">
            <div className="rounded-2xl border border-stone-200 bg-white/80 px-6 py-4 text-sm shadow-sm dark:border-stone-800 dark:bg-stone-900/80">
              正在加载 DiveEnd 工作区...
            </div>
          </div>
        ) : (
          <HistoryPanel />
        )}
      </div>
    );
  }

  // For session detail, show full-screen without sidebars
  if (activePanel === 'session') {
    return (
      <div className="flex h-full min-h-0 flex-col bg-[#f5f1e8] text-stone-900 dark:bg-[#151515] dark:text-stone-100">
        {error && (
          <div className="border-b border-amber-300 bg-amber-100 px-4 py-2 text-sm text-amber-900 dark:border-amber-800 dark:bg-amber-950/60 dark:text-amber-100">
            {error}
          </div>
        )}
        {isHydrating ? (
          <div className="flex flex-1 items-center justify-center">
            <div className="rounded-2xl border border-stone-200 bg-white/80 px-6 py-4 text-sm shadow-sm dark:border-stone-800 dark:bg-stone-900/80">
              正在加载 DiveEnd 工作区...
            </div>
          </div>
        ) : (
          <SessionDetailPanel />
        )}
      </div>
    );
  }

  // For deepstart panel, show full-screen without sidebars
  if (activePanel === 'deepstart') {
    return (
      <div className="flex h-full min-h-0 flex-col bg-[#f5f1e8] text-stone-900 dark:bg-[#151515] dark:text-stone-100">
        {error && (
          <div className="border-b border-amber-300 bg-amber-100 px-4 py-2 text-sm text-amber-900 dark:border-amber-800 dark:bg-amber-950/60 dark:text-amber-100">
            {error}
          </div>
        )}
        {isHydrating ? (
          <div className="flex flex-1 items-center justify-center">
            <div className="rounded-2xl border border-stone-200 bg-white/80 px-6 py-4 text-sm shadow-sm dark:border-stone-800 dark:bg-stone-900/80">
              正在加载 DiveEnd 工作区...
            </div>
          </div>
        ) : (
          <DeepStartPanel />
        )}
      </div>
    );
  }

  // For other panels, show with sidebars
  return (
    <div className="flex h-full min-h-0 flex-col bg-[#f5f1e8] text-stone-900 dark:bg-[#151515] dark:text-stone-100">
      <div className="flex h-full min-h-0 flex-col">
        {error && (
          <div className="border-b border-amber-300 bg-amber-100 px-4 py-2 text-sm text-amber-900 dark:border-amber-800 dark:bg-amber-950/60 dark:text-amber-100">
            {error}
          </div>
        )}

        {isHydrating ? (
          <div className="flex flex-1 items-center justify-center">
            <div className="rounded-2xl border border-stone-200 bg-white/80 px-6 py-4 text-sm shadow-sm dark:border-stone-800 dark:bg-stone-900/80">
              正在加载 DiveEnd 工作区...
            </div>
          </div>
        ) : (
          <div className="flex-1 min-h-0">
            <Allotment proportionalLayout={false}>
              <Allotment.Pane
                minSize={56}
                maxSize={380}
                preferredSize={leftPanelCollapsed ? 56 : Math.min(config.leftPanelWidth, 340)}
              >
                <SettingsPanel />
              </Allotment.Pane>

              <Allotment.Pane minSize={360}>
                {activePanel === 'deepread' ? <DeepReadPanel /> :
                 activePanel === 'screening' ? <ScreeningPanel /> :
                 activePanel === 'sync' ? <SyncPanel /> :
                 <DeepReadPanel />}
              </Allotment.Pane>

              <Allotment.Pane
                minSize={56}
                maxSize={460}
                preferredSize={rightPanelCollapsed ? 56 : Math.min(config.rightPanelWidth, 380)}
              >
                <PaperListPanel />
              </Allotment.Pane>
            </Allotment>
          </div>
        )}
      </div>
    </div>
  );
}
