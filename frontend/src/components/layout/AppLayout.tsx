import { Allotment } from 'allotment';
import 'allotment/dist/style.css';
import { useAppStore } from '../../stores/appStore';
import { DeepReadPanel } from '../deepread/DeepReadPanel';
import { DeepStartPanel } from '../deepstart/DeepStartPanel';
import { PaperListPanel } from '../paperlist/PaperListPanel';
import { SettingsPanel } from '../settings/SettingsPanel';

export function AppLayout() {
  const {
    activePanel,
    config,
    error,
    isHydrating,
    leftPanelCollapsed,
    rightPanelCollapsed,
  } = useAppStore();

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
              {activePanel === 'deepread' ? <DeepReadPanel /> : <DeepStartPanel />}
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
