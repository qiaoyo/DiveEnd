import React, { useState, useCallback } from 'react';
import { Allotment } from 'allotment';
import 'allotment/dist/style.css';
import { Maximize2, Minimize2, PanelLeft, PanelRight } from 'lucide-react';

interface SplitScreenLayoutProps {
  leftPanel: React.ReactNode;
  rightPanel: React.ReactNode;
  defaultLeftWidth?: number;
  minLeftWidth?: number;
  minRightWidth?: number;
  className?: string;
}

export const SplitScreenLayout: React.FC<SplitScreenLayoutProps> = ({
  leftPanel,
  rightPanel,
  defaultLeftWidth = 50,
  minLeftWidth = 200,
  minRightWidth = 200,
  className = '',
}) => {
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [visiblePanel, setVisiblePanel] = useState<'both' | 'left' | 'right'>('both');

  const toggleFullscreen = useCallback(() => {
    setIsFullscreen(!isFullscreen);
  }, [isFullscreen]);

  const handlePanelVisibility = useCallback((mode: 'both' | 'left' | 'right') => {
    setVisiblePanel(mode);
  }, []);

  const containerClasses = `
    ${isFullscreen ? 'fixed inset-0 z-50 bg-background' : 'relative h-full w-full'}
    ${className}
  `.trim();

  const toolbarClasses = 'flex items-center justify-between px-3 py-2 border-b bg-muted/50';

  return (
    <div className={containerClasses}>
      {/* Toolbar */}
      <div className={toolbarClasses}>
        <div className="flex items-center gap-1">
          <button
            onClick={() => handlePanelVisibility('left')}
            className={`p-1.5 rounded-md transition-colors ${
              visiblePanel === 'left' ? 'bg-primary text-primary-foreground' : 'hover:bg-muted'
            }`}
            title="Show PDF Only"
          >
            <PanelLeft className="w-4 h-4" />
          </button>
          <button
            onClick={() => handlePanelVisibility('both')}
            className={`p-1.5 rounded-md transition-colors ${
              visiblePanel === 'both' ? 'bg-primary text-primary-foreground' : 'hover:bg-muted'
            }`}
            title="Split View"
          >
            <Maximize2 className="w-4 h-4" />
          </button>
          <button
            onClick={() => handlePanelVisibility('right')}
            className={`p-1.5 rounded-md transition-colors ${
              visiblePanel === 'right' ? 'bg-primary text-primary-foreground' : 'hover:bg-muted'
            }`}
            title="Show Translation Only"
          >
            <PanelRight className="w-4 h-4" />
          </button>
        </div>

        <button
          onClick={toggleFullscreen}
          className="p-1.5 rounded-md hover:bg-muted transition-colors"
          title={isFullscreen ? 'Exit Fullscreen' : 'Fullscreen'}
        >
          {isFullscreen ? (
            <Minimize2 className="w-4 h-4" />
          ) : (
            <Maximize2 className="w-4 h-4" />
          )}
        </button>
      </div>

      {/* Content Area */}
      <div className="flex-1 overflow-hidden" style={{ height: 'calc(100% - 48px)' }}>
        {visiblePanel === 'left' && (
          <div className="h-full w-full">{leftPanel}</div>
        )}
        {visiblePanel === 'right' && (
          <div className="h-full w-full">{rightPanel}</div>
        )}
        {visiblePanel === 'both' && (
          <Allotment defaultSizes={[defaultLeftWidth, 100 - defaultLeftWidth]}>
            <Allotment.Pane minSize={minLeftWidth}>
              {leftPanel}
            </Allotment.Pane>
            <Allotment.Pane minSize={minRightWidth}>
              {rightPanel}
            </Allotment.Pane>
          </Allotment>
        )}
      </div>
    </div>
  );
};

export default SplitScreenLayout;
