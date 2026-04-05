import { Allotment } from 'allotment';
import 'allotment/dist/style.css';
import { useLocation } from 'react-router-dom';
import { useAppStore } from '../../stores/appStore';
import { DeepReadPanel } from '../deepread/DeepReadPanel';
import { DeepStartPanel } from '../deepstart/DeepStartPanel';
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
  const activePanel = location.pathname === '/deepread' ? 'deepread' :
                       location.pathname === '/screening' ? 'screening' :
                       location.pathname === '/sync' ? 'sync' :
                       'deepstart';

  return (
    <div className="h-full bg-[#f5f1e8] text-stone-900 dark:bg-[#151515] dark:text-stone-100">
      <div className="flex h-full flex-col">
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
          <Allotment proportionalLayout={false}>
            <Allotment.Pane
              minSize={64}
              maxSize={420}
              preferredSize={leftPanelCollapsed ? 64 : config.leftPanelWidth}
            >
              <SettingsPanel />
            </Allotment.Pane>

            <Allotment.Pane minSize={520}>
              {activePanel === 'deepread' ? <DeepReadPanel /> :
               activePanel === 'screening' ? <ScreeningPanel /> :
               activePanel === 'sync' ? <SyncPanel /> :
               <DeepStartPanel />}
            </Allotment.Pane>

            <Allotment.Pane
              minSize={68}
              maxSize={520}
              preferredSize={rightPanelCollapsed ? 68 : config.rightPanelWidth}
            >
              <PaperListPanel />
            </Allotment.Pane>
          </Allotment>
        )}
      </div>
    </div>
  );
}
