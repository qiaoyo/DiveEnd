import { useState } from 'react';
import { FileText, FolderOpen, Search, ChevronRight, ChevronDown, MoreVertical, Trash2, FolderPlus } from 'lucide-react';
import { useAppStore } from '../../stores/appStore';

export function PaperListPanel() {
  const { papers, selectedPaper, setSelectedPaper, setActivePanel, removePaper, toggleRightPanel, rightPanelCollapsed } = useAppStore();
  const [searchTerm, setSearchTerm] = useState('');
  const [expandedFolders, setExpandedFolders] = useState<Set<string>>(new Set(['uncategorized']));

  const filteredPapers = papers.filter(paper => 
    paper.title.toLowerCase().includes(searchTerm.toLowerCase()) ||
    paper.authors.toLowerCase().includes(searchTerm.toLowerCase())
  );

  // Group papers by category
  const groupedPapers = filteredPapers.reduce((acc, paper) => {
    const category = paper.category || 'uncategorized';
    if (!acc[category]) {
      acc[category] = [];
    }
    acc[category].push(paper);
    return acc;
  }, {} as Record<string, typeof papers>);

  const toggleFolder = (folder: string) => {
    const newExpanded = new Set(expandedFolders);
    if (newExpanded.has(folder)) {
      newExpanded.delete(folder);
    } else {
      newExpanded.add(folder);
    }
    setExpandedFolders(newExpanded);
  };

  const handlePaperClick = (paper: typeof papers[0]) => {
    setSelectedPaper(paper);
    setActivePanel('deepread');
  };

  if (rightPanelCollapsed) {
    return (
      <div className="h-full flex items-center justify-center bg-surface border-l border-border">
        <button 
          onClick={toggleRightPanel}
          className="p-2 hover:bg-border rounded"
        >
          <ChevronRight className="w-4 h-4" />
        </button>
      </div>
    );
  }

  return (
    <div className="h-full flex flex-col bg-surface border-l border-border">
      {/* Header */}
      <div className="flex items-center justify-between p-4 border-b border-border">
        <div className="flex items-center gap-2">
          <FolderOpen className="w-5 h-5 text-primary" />
          <span className="font-semibold">论文库</span>
          <span className="text-sm text-text-muted">({papers.length})</span>
        </div>
        <div className="flex gap-1">
          <button 
            onClick={toggleRightPanel}
            className="p-1 hover:bg-border rounded"
          >
            <ChevronRight className="w-4 h-4" />
          </button>
        </div>
      </div>

      {/* Search */}
      <div className="p-3 border-b border-border">
        <div className="relative">
          <Search className="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-text-muted" />
          <input
            type="text"
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            placeholder="搜索论文..."
            className="w-full pl-9 pr-3 py-2 text-sm border border-border rounded bg-background focus:outline-none focus:ring-2 focus:ring-primary"
          />
        </div>
      </div>

      {/* Paper List */}
      <div className="flex-1 overflow-y-auto">
        {Object.keys(groupedPapers).length === 0 ? (
          <div className="flex flex-col items-center justify-center h-full text-text-muted p-4">
            <FileText className="w-10 h-10 mb-2 opacity-50" />
            <p className="text-sm">暂无论文</p>
            <p className="text-xs mt-1">在 DeepStart 中搜索并导入</p>
          </div>
        ) : (
          <div className="p-2">
            {Object.entries(groupedPapers).map(([category, categoryPapers]) => (
              <div key={category} className="mb-2">
                {/* Folder Header */}
                <button
                  onClick={() => toggleFolder(category)}
                  className="w-full flex items-center gap-2 px-2 py-1.5 text-sm font-medium hover:bg-border/50 rounded"
                >
                  {expandedFolders.has(category) ? (
                    <ChevronDown className="w-4 h-4" />
                  ) : (
                    <ChevronRight className="w-4 h-4" />
                  )}
                  <FolderOpen className="w-4 h-4 text-primary" />
                  <span className="capitalize">{category}</span>
                  <span className="text-xs text-text-muted ml-auto">{categoryPapers.length}</span>
                </button>

                {/* Papers in Folder */}
                {expandedFolders.has(category) && (
                  <div className="ml-4 space-y-1">
                    {categoryPapers.map((paper) => (
                      <div
                        key={paper.id}
                        onClick={() => handlePaperClick(paper)}
                        className={`group flex items-start gap-2 p-2 rounded cursor-pointer transition-colors ${
                          selectedPaper?.id === paper.id
                            ? 'bg-primary/10 border border-primary'
                            : 'hover:bg-border/50 border border-transparent'
                        }`}
                      >
                        <FileText className="w-4 h-4 mt-0.5 text-text-muted flex-shrink-0" />
                        <div className="flex-1 min-w-0">
                          <p className="text-sm font-medium line-clamp-2">{paper.title}</p>
                          <p className="text-xs text-text-muted line-clamp-1">{paper.authors}</p>
                        </div>
                        <button
                          onClick={(e) => {
                            e.stopPropagation();
                            removePaper(paper.id);
                          }}
                          className="opacity-0 group-hover:opacity-100 p-1 hover:text-red-500 transition-opacity"
                        >
                          <Trash2 className="w-3 h-3" />
                        </button>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Footer */}
      <div className="p-3 border-t border-border">
        <button className="w-full flex items-center justify-center gap-2 p-2 text-sm text-text-muted hover:text-text hover:bg-border/50 rounded transition-colors">
          <FolderPlus className="w-4 h-4" />
          新建文件夹
        </button>
      </div>
    </div>
  );
}