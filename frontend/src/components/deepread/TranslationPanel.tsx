import React, { useState, useRef, useEffect } from 'react';
import {
  ChevronDown,
  ChevronUp,
  Copy,
  Check,
  Highlighter,
  MessageSquare,
  Languages,
  BookOpen,
  ScrollText,
  Sparkles
} from 'lucide-react';

export interface SectionTranslation {
  id: string;
  title: string;
  titleCN: string;
  content: string;
  highlights?: string[];
  keyConcepts?: Concept[];
}

export interface Concept {
  term: string;
  definition: string;
  context?: string;
}

export interface TranslationData {
  title: string;
  titleCN: string;
  authors: string[];
  abstract: string;
  abstractCN: string;
  sections: SectionTranslation[];
  keyFindings: string[];
  criticalAnalysis?: string;
  generatedAt: string;
  model: string;
}

interface TranslationPanelProps {
  translation: TranslationData | null;
  isLoading?: boolean;
  error?: string | null;
  currentSectionId?: string;
  onSectionClick?: (sectionId: string) => void;
  onConceptClick?: (concept: Concept) => void;
  className?: string;
}

export const TranslationPanel: React.FC<TranslationPanelProps> = ({
  translation,
  isLoading = false,
  error = null,
  currentSectionId,
  onSectionClick,
  onConceptClick,
  className = '',
}) => {
  const [expandedSections, setExpandedSections] = useState<Set<string>>(new Set());
  const [copiedId, setCopiedId] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<'translation' | 'concepts' | 'analysis'>('translation');
  const sectionRefs = useRef<Map<string, HTMLDivElement>>(new Map());

  // Auto-expand current section
  useEffect(() => {
    if (currentSectionId) {
      setExpandedSections(prev => new Set([...prev, currentSectionId]));
      // Scroll to section
      const element = sectionRefs.current.get(currentSectionId);
      if (element) {
        element.scrollIntoView({ behavior: 'smooth', block: 'start' });
      }
    }
  }, [currentSectionId]);

  const toggleSection = (sectionId: string) => {
    setExpandedSections(prev => {
      const newSet = new Set(prev);
      if (newSet.has(sectionId)) {
        newSet.delete(sectionId);
      } else {
        newSet.add(sectionId);
        onSectionClick?.(sectionId);
      }
      return newSet;
    });
  };

  const copyToClipboard = async (text: string, id: string) => {
    try {
      await navigator.clipboard.writeText(text);
      setCopiedId(id);
      setTimeout(() => setCopiedId(null), 2000);
    } catch (err) {
      console.error('Failed to copy:', err);
    }
  };

  if (isLoading) {
    return (
      <div className={`flex flex-col items-center justify-center h-full p-8 ${className}`}>
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-primary mb-4" />
        <p className="text-muted-foreground">AI is analyzing and translating...</p>
        <p className="text-sm text-muted-foreground mt-2">This may take a minute for long papers</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className={`flex flex-col items-center justify-center h-full p-8 ${className}`}>
        <div className="text-red-500 mb-4">
          <svg className="w-16 h-16" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        </div>
        <h3 className="text-lg font-semibold text-gray-900 mb-2">Translation Failed</h3>
        <p className="text-gray-600 text-center max-w-md">{error}</p>
      </div>
    );
  }

  if (!translation) {
    return (
      <div className={`flex flex-col items-center justify-center h-full p-8 ${className}`}>
        <div className="w-16 h-16 bg-muted rounded-full flex items-center justify-center mb-4">
          <Languages className="w-8 h-8 text-muted-foreground" />
        </div>
        <h3 className="text-lg font-semibold mb-2">Ready to Translate</h3>
        <p className="text-muted-foreground text-center max-w-md">
          Click "DeepRead" to start AI-powered translation and analysis of this paper.
        </p>
      </div>
    );
  }

  const allConcepts = translation.sections.flatMap(s => s.keyConcepts || []);

  return (
    <div className={`flex flex-col h-full ${className}`}>
      {/* Header with Tabs */}
      <div className="border-b bg-muted/30">
        <div className="px-4 py-3">
          <h2 className="text-lg font-semibold mb-1">{translation.titleCN}</h2>
          <p className="text-sm text-muted-foreground line-clamp-1">{translation.title}</p>
        </div>

        <div className="flex border-t">
          {[
            { id: 'translation', label: '翻译', icon: ScrollText },
            { id: 'concepts', label: '概念', icon: BookOpen },
            { id: 'analysis', label: '分析', icon: Sparkles },
          ].map(({ id, label, icon: Icon }) => (
            <button
              key={id}
              onClick={() => setActiveTab(id as any)}
              className={`flex items-center gap-2 px-4 py-2 text-sm font-medium transition-colors border-b-2 -mb-px ${
                activeTab === id
                  ? 'border-primary text-primary'
                  : 'border-transparent text-muted-foreground hover:text-foreground'
              }`}
            >
              <Icon className="w-4 h-4" />
              {label}
            </button>
          ))}
        </div>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-auto">
        {activeTab === 'translation' && (
          <div className="p-4 space-y-4">
            {/* Abstract */}
            <div className="bg-muted/30 rounded-lg p-4">
              <h3 className="font-semibold mb-2 flex items-center gap-2">
                <Bookmark className="w-4 h-4" />
                摘要
              </h3>
              <p className="text-sm text-muted-foreground leading-relaxed">
                {translation.abstractCN}
              </p>
            </div>

            {/* Sections */}
            {translation.sections.map((section) => (
              <div
                key={section.id}
                ref={(el) => {
                  if (el) sectionRefs.current.set(section.id, el);
                }}
                className={`border rounded-lg overflow-hidden transition-colors ${
                  currentSectionId === section.id ? 'border-primary bg-primary/5' : 'border-border'
                }`}
              >
                <button
                  onClick={() => toggleSection(section.id)}
                  className="w-full flex items-center justify-between p-3 hover:bg-muted/50 transition-colors"
                >
                  <div className="flex items-center gap-2">
                    <Highlighter className="w-4 h-4 text-muted-foreground" />
                    <span className="font-medium">{section.titleCN}</span>
                    <span className="text-sm text-muted-foreground">({section.title})</span>
                  </div>
                  {expandedSections.has(section.id) ? (
                    <ChevronUp className="w-4 h-4 text-muted-foreground" />
                  ) : (
                    <ChevronDown className="w-4 h-4 text-muted-foreground" />
                  )}
                </button>

                {expandedSections.has(section.id) && (
                  <div className="p-3 border-t bg-muted/20">
                    <p className="text-sm leading-relaxed whitespace-pre-wrap">
                      {section.content}
                    </p>

                    {/* Key Concepts */}
                    {section.keyConcepts && section.keyConcepts.length > 0 && (
                      <div className="mt-3 pt-3 border-t">
                        <p className="text-xs font-medium text-muted-foreground mb-2">关键概念:</p>
                        <div className="flex flex-wrap gap-1">
                          {section.keyConcepts.map((concept, idx) => (
                            <button
                              key={idx}
                              onClick={() => onConceptClick?.(concept)}
                              className="text-xs px-2 py-1 bg-primary/10 text-primary rounded-full hover:bg-primary/20 transition-colors"
                            >
                              {concept.term}
                            </button>
                          ))}
                        </div>
                      </div>
                    )}

                    {/* Copy Button */}
                    <div className="mt-3 flex justify-end">
                      <button
                        onClick={() => copyToClipboard(section.content, section.id)}
                        className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors"
                      >
                        {copiedId === section.id ? (
                          <>
                            <Check className="w-3 h-3" />
                            已复制
                          </>
                        ) : (
                          <>
                            <Copy className="w-3 h-3" />
                            复制
                          </>
                        )}
                      </button>
                    </div>
                  </div>
                )}
              </div>
            ))}
          </div>
        )}

        {activeTab === 'concepts' && (
          <div className="p-4">
            <div className="flex items-center justify-between mb-4">
              <h3 className="font-semibold">关键概念 ({allConcepts.length})</h3>
            </div>

            {allConcepts.length === 0 ? (
              <div className="text-center py-8 text-muted-foreground">
                <BookOpen className="w-12 h-12 mx-auto mb-3 opacity-50" />
                <p>暂无提取的概念</p>
              </div>
            ) : (
              <div className="space-y-3">
                {allConcepts.map((concept, idx) => (
                  <div
                    key={idx}
                    className="p-4 bg-muted/30 rounded-lg hover:bg-muted/50 transition-colors cursor-pointer"
                    onClick={() => onConceptClick?.(concept)}
                  >
                    <h4 className="font-medium text-primary mb-1">{concept.term}</h4>
                    <p className="text-sm text-muted-foreground">{concept.definition}</p>
                    {concept.context && (
                      <p className="text-xs text-muted-foreground mt-2 italic">
                        上下文: {concept.context}
                      </p>
                    )}
                  </div>
                ))}
              </div>
            )}
          </div>
        )}

        {activeTab === 'analysis' && (
          <div className="p-4 space-y-4">
            {/* Key Findings */}
            <div className="bg-muted/30 rounded-lg p-4">
              <h3 className="font-semibold mb-3 flex items-center gap-2">
                <Sparkles className="w-4 h-4" />
                关键发现
              </h3>
              <ul className="space-y-2">
                {translation.keyFindings?.map((finding, idx) => (
                  <li key={idx} className="text-sm text-muted-foreground flex items-start gap-2">
                    <span className="text-primary mt-0.5">•</span>
                    {finding}
                  </li>
                )) || <li className="text-sm text-muted-foreground">暂无分析数据</li>}
              </ul>
            </div>

            {/* Critical Analysis */}
            {translation.criticalAnalysis && (
              <div className="bg-muted/30 rounded-lg p-4">
                <h3 className="font-semibold mb-3 flex items-center gap-2">
                  <MessageSquare className="w-4 h-4" />
                  批判性分析
                </h3>
                <div className="text-sm text-muted-foreground leading-relaxed">
                  {translation.criticalAnalysis}
                </div>
              </div>
            )}

            {/* Metadata */}
            <div className="text-xs text-muted-foreground border-t pt-4">
              <p>生成时间: {new Date(translation.generatedAt).toLocaleString('zh-CN')}</p>
              <p>模型: {translation.model}</p>
            </div>
          </div>
        )}
      </div>
    </div>
  );
};

export default TranslationPanel;
