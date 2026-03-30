import { useState } from 'react';
import { BookOpen, Loader2, Languages, FileText, Highlighter } from 'lucide-react';
import { useAppStore } from '../../stores/appStore';

export function DeepReadPanel() {
  const { selectedPaper, isTranslating, setIsTranslating, theme } = useAppStore();
  const [translatedContent, setTranslatedContent] = useState<Record<string, string>>({});
  const [showTranslation, setShowTranslation] = useState(false);

  if (!selectedPaper) {
    return (
      <div className="flex-1 flex flex-col items-center justify-center text-text-muted p-8">
        <BookOpen className="w-16 h-16 mb-4 opacity-30" />
        <h3 className="text-lg font-medium mb-2">DeepRead - 论文阅读</h3>
        <p className="text-sm text-center">
          从左侧选择一篇论文开始阅读<br />
          或在 DeepStart 中搜索并导入论文
        </p>
      </div>
    );
  }

  const handleTranslate = async () => {
    setIsTranslating(true);
    
    try {
      // Call Wails backend for translation
      if (window.go?.main?.App) {
        const sections = ['Abstract', 'Introduction', 'Method'];
        const translations: Record<string, string> = {};
        
        for (const section of sections) {
          const translated = await window.go.main.App.TranslateText(
            `${section}: ${selectedPaper.abstract}`,
            'zh-CN'
          );
          translations[section] = translated;
        }
        setTranslatedContent(translations);
      } else {
        // Demo mode
        await new Promise(resolve => setTimeout(resolve, 2000));
        setTranslatedContent({
          'Abstract': '本文提出了一种使用大型语言模型进行机器人控制的新方法。我们展示了一个基于Transformer的框架，能够理解自然语言指令并将其转换为机器人动作...',
          'Introduction': '近年来，大型语言模型（LLM）在各种任务中展现了卓越的能力。然而，将这些能力应用于机器人控制仍然是一个活跃的研究领域...',
          'Method': '我们提出的方法包括三个主要组件：指令解析器、动作规划器和执行控制器...'
        });
      }
      setShowTranslation(true);
    } catch (error) {
      console.error('Translation failed:', error);
    } finally {
      setIsTranslating(false);
    }
  };

  return (
    <div className="flex-1 flex flex-col min-h-0">
      {/* Header */}
      <div className="p-4 border-b border-border">
        <div className="flex items-center justify-between">
          <div>
            <h2 className="text-lg font-semibold flex items-center gap-2">
              <BookOpen className="w-5 h-5 text-primary" />
              DeepRead - 论文阅读
            </h2>
          </div>
          <div className="flex gap-2">
            <button
              onClick={handleTranslate}
              disabled={isTranslating}
              className="px-3 py-1.5 bg-primary text-white text-sm rounded-lg hover:bg-primary-dark disabled:opacity-50 flex items-center gap-1"
            >
              {isTranslating ? (
                <>
                  <Loader2 className="w-4 h-4 animate-spin" />
                  翻译中...
                </>
              ) : (
                <>
                  <Languages className="w-4 h-4" />
                  AI 翻译
                </>
              )}
            </button>
            <button
              className="px-3 py-1.5 border border-border text-sm rounded-lg hover:bg-surface flex items-center gap-1"
            >
              <Highlighter className="w-4 h-4" />
              标黄
            </button>
          </div>
        </div>
      </div>

      {/* Content */}
      <div className="flex-1 flex overflow-hidden">
        {/* Original Content */}
        <div className="flex-1 overflow-y-auto p-6">
          <h1 className="text-xl font-bold mb-4">{selectedPaper.title}</h1>
          <p className="text-sm text-text-muted mb-4">{selectedPaper.authors}</p>
          <p className="text-xs text-text-muted mb-6">{selectedPaper.journal} · {selectedPaper.year}</p>
          
          <div className="prose prose-sm max-w-none">
            <h3 className="text-lg font-semibold mb-2">Abstract</h3>
            <p className="text-sm leading-relaxed">{selectedPaper.abstract}</p>
          </div>

          {/* PDF Viewer placeholder */}
          <div className="mt-6 p-8 border-2 border-dashed border-border rounded-lg flex items-center justify-center text-text-muted">
            <div className="text-center">
              <FileText className="w-12 h-12 mx-auto mb-2 opacity-50" />
              <p>PDF 文件</p>
              <p className="text-xs mt-1">{selectedPaper.pdfPath || '点击加载 PDF'}</p>
            </div>
          </div>
        </div>

        {/* Translation Panel */}
        {showTranslation && (
          <div className="w-96 border-l border-border overflow-y-auto bg-surface p-6">
            <h3 className="text-lg font-semibold mb-4 flex items-center gap-2">
              <Languages className="w-5 h-5 text-primary" />
              中文翻译
            </h3>
            
            <div className="space-y-6">
              {Object.entries(translatedContent).map(([section, content]) => (
                <div key={section}>
                  <h4 className="font-medium text-primary mb-2">{section}</h4>
                  <p className="text-sm leading-relaxed text-text-muted">{content}</p>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}