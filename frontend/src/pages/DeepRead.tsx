import React, { useState, useEffect, useCallback } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { SplitScreenLayout } from '../components/deepread/SplitScreenLayout';
import PDFViewer from '../components/deepread/PDFViewer';
import TranslationPanel, { TranslationData, Concept } from '../components/deepread/TranslationPanel';
import { Button } from '../components/ui/button';
import {
  Sparkles,
  ArrowLeft,
  BookOpen,
  Loader2,
  AlertCircle,
  RefreshCw
} from 'lucide-react';
import { useToast } from '../hooks/use-toast';

interface Paper {
  id: string;
  title: string;
  authors: string[];
  pdfUrl: string;
  localPath?: string;
}

export const DeepRead: React.FC = () => {
  const { paperId } = useParams<{ paperId: string }>();
  const navigate = useNavigate();
  const { toast } = useToast();

  const [paper, setPaper] = useState<Paper | null>(null);
  const [translation, setTranslation] = useState<TranslationData | null>(null);
  const [isLoadingPaper, setIsLoadingPaper] = useState(true);
  const [isTranslating, setIsTranslating] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [currentPage, setCurrentPage] = useState(1);
  const [currentSectionId, setCurrentSectionId] = useState<string | undefined>();

  // Fetch paper details
  useEffect(() => {
    const fetchPaper = async () => {
      if (!paperId) return;

      try {
        setIsLoadingPaper(true);
        setError(null);

        // TODO: Replace with actual API call
        // const response = await fetch(`/api/papers/${paperId}`);
        // const data = await response.json();

        // Mock data for development
        await new Promise(resolve => setTimeout(resolve, 500));
        const mockPaper: Paper = {
          id: paperId,
          title: 'Sample Research Paper Title',
          authors: ['John Doe', 'Jane Smith', 'Bob Johnson'],
          pdfUrl: 'https://arxiv.org/pdf/2301.00001.pdf',
        };

        setPaper(mockPaper);

        // Check if translation exists
        // await fetchTranslation(paperId);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to load paper');
        toast({
          title: 'Error',
          description: 'Failed to load paper details',
          variant: 'destructive',
        });
      } finally {
        setIsLoadingPaper(false);
      }
    };

    fetchPaper();
  }, [paperId, toast]);

  // Handle DeepRead (translation)
  const handleDeepRead = useCallback(async () => {
    if (!paper) return;

    try {
      setIsTranslating(true);
      setError(null);

      // TODO: Replace with actual API call
      // const response = await fetch('/api/deepread', {
      //   method: 'POST',
      //   headers: { 'Content-Type': 'application/json' },
      //   body: JSON.stringify({ paperId: paper.id, pdfUrl: paper.pdfUrl }),
      // });
      // const data = await response.json();

      // Mock translation data for development
      await new Promise(resolve => setTimeout(resolve, 2000));
      const mockTranslation: TranslationData = {
        title: paper.title,
        titleCN: '示例研究论文标题',
        authors: paper.authors,
        abstract: 'This is a sample abstract of the research paper.',
        abstractCN: '这是研究论文的示例摘要。',
        sections: [
          {
            id: 'intro',
            title: 'Introduction',
            titleCN: '引言',
            content: '研究背景与动机介绍。本文提出了一种新的方法来解决现有问题。',
            highlights: ['研究背景', '问题定义'],
            keyConcepts: [
              { term: '深度学习', definition: '一种机器学习方法，使用多层神经网络' },
            ],
          },
          {
            id: 'method',
            title: 'Method',
            titleCN: '方法',
            content: '我们提出的方法包括三个主要步骤：数据预处理、特征提取和模型训练。',
            highlights: ['三步流程', '特征提取'],
            keyConcepts: [
              { term: '特征提取', definition: '从原始数据中提取有用特征的过程' },
            ],
          },
          {
            id: 'results',
            title: 'Results',
            titleCN: '结果',
            content: '实验结果表明，我们的方法在准确率和效率方面都优于现有方法。',
            highlights: ['准确率提升', '效率优化'],
          },
        ],
        keyFindings: [
          '新方法在准确率上提升了15%',
          '计算效率比现有方法快2倍',
          '在不同数据集上表现稳定',
        ],
        criticalAnalysis: '本研究提出了一种创新的方法，但仍有一些局限性需要在未来的工作中解决。',
        generatedAt: new Date().toISOString(),
        model: 'gpt-4',
      };

      setTranslation(mockTranslation);
      setExpandedSections(new Set(['intro']));

      toast({
        title: 'Translation Complete',
        description: 'AI has finished analyzing and translating the paper',
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Translation failed');
      toast({
        title: 'Translation Failed',
        description: err instanceof Error ? err.message : 'Failed to translate paper',
        variant: 'destructive',
      });
    } finally {
      setIsTranslating(false);
    }
  }, [paper, toast]);

  const [expandedSections, setExpandedSections] = useState<Set<string>>(new Set());

  const handleConceptClick = useCallback((concept: Concept) => {
    toast({
      title: concept.term,
      description: concept.definition,
    });
  }, [toast]);

  if (isLoadingPaper) {
    return (
      <div className="flex flex-col items-center justify-center h-full">
        <Loader2 className="w-8 h-8 animate-spin text-primary mb-4" />
        <p className="text-muted-foreground">Loading paper...</p>
      </div>
    );
  }

  if (error && !paper) {
    return (
      <div className="flex flex-col items-center justify-center h-full p-8">
        <AlertCircle className="w-12 h-12 text-red-500 mb-4" />
        <h3 className="text-lg font-semibold mb-2">Failed to Load Paper</h3>
        <p className="text-muted-foreground text-center mb-4">{error}</p>
        <Button onClick={() => navigate(-1)} variant="outline">
          <ArrowLeft className="w-4 h-4 mr-2" />
          Go Back
        </Button>
      </div>
    );
  }

  return (
    <div className="flex flex-col h-full bg-background">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b bg-muted/30">
        <div className="flex items-center gap-3">
          <Button variant="ghost" size="sm" onClick={() => navigate(-1)}>
            <ArrowLeft className="w-4 h-4 mr-1" />
            Back
          </Button>
          <div className="h-4 w-px bg-border" />
          {paper && (
            <div className="hidden sm:block">
              <h1 className="text-sm font-medium line-clamp-1">{paper.title}</h1>
              <p className="text-xs text-muted-foreground">
                {paper.authors.slice(0, 3).join(', ')}
                {paper.authors.length > 3 && ` +${paper.authors.length - 3}`}
              </p>
            </div>
          )}
        </div>

        {!translation && !isTranslating && (
          <Button onClick={handleDeepRead} className="gap-2">
            <Sparkles className="w-4 h-4" />
            DeepRead
          </Button>
        )}

        {isTranslating && (
          <Button disabled className="gap-2">
            <Loader2 className="w-4 h-4 animate-spin" />
            Analyzing...
          </Button>
        )}

        {translation && !isTranslating && (
          <div className="flex items-center gap-2">
            <Button variant="outline" size="sm" onClick={handleDeepRead} className="gap-1">
              <RefreshCw className="w-3 h-3" />
              Re-analyze
            </Button>
            <Button variant="ghost" size="sm" className="gap-1">
              <BookOpen className="w-4 h-4" />
              Read
            </Button>
          </div>
        )}
      </div>

      {/* Main Content */}
      {translation ? (
        <SplitScreenLayout
          leftPanel={
            <PDFViewer
              url={paper?.pdfUrl || ''}
              filename={paper?.title}
              initialPage={currentPage}
              onPageChange={setCurrentPage}
              onTextSelection={(text) => {
                console.log('Selected text:', text);
              }}
            />
          }
          rightPanel={
            <TranslationPanel
              translation={translation}
              currentSectionId={currentSectionId}
              onSectionClick={setCurrentSectionId}
              onConceptClick={handleConceptClick}
            />
          }
          defaultLeftWidth={55}
        />
      ) : (
        <div className="flex-1 flex flex-col">
          <PDFViewer
            url={paper?.pdfUrl || ''}
            filename={paper?.title}
            initialPage={currentPage}
            onPageChange={setCurrentPage}
            onTextSelection={(text) => {
              console.log('Selected text:', text);
            }}
          />
        </div>
      )}
    </div>
  );
};

export default DeepRead;
