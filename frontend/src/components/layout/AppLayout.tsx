import { Allotment } from 'allotment';
import 'allotment/dist/style.css';
import { SettingsPanel } from '../settings/SettingsPanel';
import { DeepStartPanel } from '../deepstart/DeepStartPanel';
import { DeepReadPanel } from '../deepread/DeepReadPanel';
import { PaperListPanel } from '../paperlist/PaperListPanel';
import { useAppStore } from '../../stores/appStore';

export function AppLayout() {
  const { theme, leftPanelCollapsed, rightPanelCollapsed } = useAppStore();
  
  return (
    <div className={`h-full ${theme === 'dark' ? 'dark' : ''}`}>
      <div className="h-full flex flex-col bg-background text-text">
        <Allotment>
          {/* Left Panel - Settings */}
          <Allotment.Pane 
            visible={!leftPanelCollapsed} 
            minSize={200}
            maxSize={360}
          >
            <SettingsPanel />
          </Allotment.Pane>
          
          {/* Center Panel - DeepStart / DeepRead */}
          <Allotment.Pane>
            <div className="h-full flex flex-col">
              <DeepStartPanel />
              <DeepReadPanel />
            </div>
          </Allotment.Pane>
          
          {/* Right Panel - Paper List */}
          <Allotment.Pane 
            visible={!rightPanelCollapsed}
            minSize={280}
            maxSize={480}
          >
            <PaperListPanel />
          </Allotment.Pane>
        </Allotment>
      </div>
    </div>
  );
}