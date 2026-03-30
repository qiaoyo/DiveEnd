import { useState } from 'react';
import { Search, Loader2, FileText, Plus, X } from 'lucide-react';
import { useAppStore } from '../../stores/appStore';
import type { Paper } from '../../types';

// Wails runtime binding declarations
declare global {
  interface Window {
    go: {
      main: {
        App: {
          SearchPapers(query: string, limit: number): Promise<Paper[]>;
          TranslateText(text: string, targetLang: string): Promise<string>;
        };
      };
    };
  }
}

export function DeepStartPanel() {
  const { 
    searchQuery, 
    setSearchQuery, 
    searchResults, 
    setSearchResults,
    isSearching,
    setIsSearching,
    addPaper,
    setSelectedPaper,
    setActivePanel
  } = useAppStore();
  
  const [selectedResults, setSelectedResults] = useState<Set<string>>(new Set());

  const handleSearch = async () => {
    if (!searchQuery.trim()) return;
    
    setIsSearching(true);
    setSearchResults([]);
    
    try {
      // Call Wails backend
      if (window.go?.main?.App) {
        const results = await window.go.main.App.SearchPapers(searchQuery, 20);
        setSearchResults(results);
      } else {
        // Demo mode - simulate results
        await new Promise(resolve => setTimeout(resolve, 1000));
        setSearchResults([
          {
            id: '1',
            title: 'Transformer-based Language Models for Robotics',
            authors: 'John Smith, Jane Doe',
            abstract: 'This paper presents a novel approach to using large language models for robotic control...',
            year: 2024,
            journal: 'arXiv',
            category: 'robotics',
            tags: ['transformer', 'robotics', 'VLA'],
            addedAt: new Date().toISOString(),
            updatedAt: new Date().toISOString()
          },
          {
            id: '2',
            title: 'World Models for Embodied AI',
            authors: 'Alice Wang, Bob Chen',
            abstract: 'We propose a new method for learning world models in embodied AI agents...',
            year: 2024,
            journal: 'NeurIPS',
            category: 'AI',
            tags: ['world model', 'embodied AI'],
            addedAt: new Date().toISOString(),
            updatedAt: new Date().toISOString()
          }
        ]);
      }
    } catch (error) {
      console.error('Search failed:', error);
    } finally {
      setIsSearching(false);
    }
  };

  const toggleSelect = (id: string) => {
    const newSet = new Set(selectedResults);
    if (newSet.has(id)) {
      newSet.delete(id);
    } else {
      newSet.add(id);
    }
    setSelectedResults(newSet);
  };

  const handleImportSelected = () => {
    const selected = searchResults.filter(p => selectedResults.has(p.id));
    selected.forEach(paper => addPaper(paper));
    setSelectedResults(new Set());
    setSearchResults([]);
    setSearchQuery('');
  };

  const handlePaperClick = (paper: Paper) => {
    setSelectedPaper(paper);
    setActivePanel('deepread');
  };

  return (
    <div className="flex-1 flex flex-col min-h-0 border-b border-border">
      {/* Header */}
      <div className="p-4 border-b border-border">
        <h2 className="text-lg font-semibold flex items-center gap-2">
          <Search className="w-5 h-5 text-primary" />
          DeepStart - 论文发现
        </h2>
        <p className="text-sm text-text-muted mt-1">
          输入研究方向，AI 帮你发现和筛选论文
        </p>
      </div>

      {/* Search Input */}
      <div className="p-4 border-b border-border">
        <div className="flex gap-2">
          <input
            type="text"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && handleSearch()}
            placeholder="输入研究方向，如: Vision-Language-Action, World Model, 机器人控制..."
            className="flex-1 p-3 border border-border rounded-lg bg-background focus:outline-none focus:ring-2 focus:ring-primary"
          />
          <button
            onClick={handleSearch}
            disabled={isSearching || !searchQuery.trim()}
            className="px-4 py-2 bg-primary text-white rounded-lg hover:bg-primary-dark disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
          >
            {isSearching ? (
              <>
                <Loader2 className="w-4 h-4 animate-spin" />
                搜索中
              </>
            ) : (
              <>
                <Search className="w-4 h-4" />
                搜索
              </>
            )}
          </button>
        </div>
      </div>

      {/* Results */}
      <div className="flex-1 overflow-y-auto p-4">
        {searchResults.length > 0 && (
          <>
            <div className="flex items-center justify-between mb-3">
              <span className="text-sm text-text-muted">
                找到 {searchResults.length} 篇论文
              </span>
              {selectedResults.size > 0 && (
                <button
                  onClick={handleImportSelected}
                  className="px-3 py-1.5 bg-primary text-white text-sm rounded-lg hover:bg-primary-dark flex items-center gap-1"
                >
                  <Plus className="w-4 h-4" />
                  导入选中 ({selectedResults.size})
                </button>
              )}
            </div>
            
            <div className="space-y-3">
              {searchResults.map((paper) => (
                <div
                  key={paper.id}
                  className={`p-4 border rounded-lg cursor-pointer transition-all ${
                    selectedResults.has(paper.id)
                      ? 'border-primary bg-primary/5'
                      : 'border-border hover:border-primary/50'
                  }`}
                  onClick={() => toggleSelect(paper.id)}
                >
                  <div className="flex items-start justify-between">
                    <div className="flex-1">
                      <h3 className="font-medium line-clamp-2">{paper.title}</h3>
                      <p className="text-sm text-text-muted mt-1">{paper.authors}</p>
                      <p className="text-xs text-text-muted mt-1">{paper.journal} · {paper.year}</p>
                    </div>
                    <div className="flex gap-1 ml-2">
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          handlePaperClick(paper);
                        }}
                        className="p-1.5 text-text-muted hover:text-primary hover:bg-primary/10 rounded"
                        title="阅读"
                      >
                        <FileText className="w-4 h-4" />
                      </button>
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          toggleSelect(paper.id);
                        }}
                        className={`p-1.5 rounded ${
                          selectedResults.has(paper.id)
                            ? 'text-primary bg-primary/10'
                            : 'text-text-muted hover:text-primary hover:bg-primary/10'
                        }`}
                        title="选中"
                      >
                        {selectedResults.has(paper.id) ? (
                          <X className="w-4 h-4" />
                        ) : (
                          <Plus className="w-4 h-4" />
                        )}
                      </button>
                    </div>
                  </div>
                  <p className="text-sm text-text-muted mt-2 line-clamp-2">{paper.abstract}</p>
                </div>
              ))}
            </div>
          </>
        )}

        {searchResults.length === 0 && !isSearching && (
          <div className="flex flex-col items-center justify-center h-full text-text-muted">
            <Search className="w-12 h-12 mb-3 opacity-50" />
            <p>输入研究方向开始搜索论文</p>
          </div>
        )}
      </div>
    </div>
  );
}