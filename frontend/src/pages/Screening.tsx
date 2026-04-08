import React, { useCallback, useEffect, useState } from 'react';
import * as backend from '../lib/backend';

interface Paper {
  id: string;
  title: string;
  authors: string;
  abstract: string;
  year: number;
  pdfPath?: string;
  status: 'pending' | 'extracting' | 'extracted' | 'screening' | 'selected' | 'rejected';
}

interface DecisionNode {
  id: string;
  message: string;
  dimension: string;
  options: Array<{
    key: string;
    label: string;
    paperIds: string[];
    count: number;
  }>;
  allowMultiSelect: boolean;
}

type ScreeningStage = 'upload' | 'extract' | 'screen' | 'results';

export const Screening: React.FC = () => {
  const [currentStage, setCurrentStage] = useState<ScreeningStage>('upload');
  const [papers, setPapers] = useState<Paper[]>([]);
  const [selectedPapers, setSelectedPapers] = useState<Set<string>>(new Set());
  const [selectedOptions, setSelectedOptions] = useState<string[]>([]);
  const [screeningHistory, setScreeningHistory] = useState<Array<{ dimension: string; choice: string }>>([]);
  const [currentNode, setCurrentNode] = useState<DecisionNode | null>(null);
  const [isProcessing, setIsProcessing] = useState(false);
  const [screeningSessionId, setScreeningSessionId] = useState<string | null>(null);
  const [extractionProgress, setExtractionProgress] = useState<number>(0);

  useEffect(() => {
    const handleExtractProgress = (event: Event) => {
      const custom = event as CustomEvent<{ progress: number }>;
      setExtractionProgress(custom.detail?.progress ?? 0);
    };

    window.addEventListener('extract-progress', handleExtractProgress as EventListener);
    return () => window.removeEventListener('extract-progress', handleExtractProgress as EventListener);
  }, []);

  const createScreeningSession = useCallback(async () => {
    try {
      const session = await backend.createScreeningSession('New Screening Session');
      setScreeningSessionId(session.id);
      return session.id;
    } catch (error) {
      console.error('创建筛选会话失败:', error);
      return null;
    }
  }, []);

  const initializeScreening = useCallback(async (sessionId: string) => {
    try {
      const decisionTree = await backend.analyzePapers(sessionId);
      setCurrentNode(decisionTree);
    } catch (error) {
      console.error('初始化筛选过程失败:', error);
      setCurrentNode({
        id: 'node-1',
        message: '先告诉我你想保留哪一类论文。',
        dimension: '研究主题',
        options: [
          { key: 'ml', label: 'Machine Learning', paperIds: [], count: 15 },
          { key: 'nlp', label: 'Natural Language Processing', paperIds: [], count: 12 },
          { key: 'cv', label: 'Computer Vision', paperIds: [], count: 8 },
          { key: 'rl', label: 'Reinforcement Learning', paperIds: [], count: 5 },
        ],
        allowMultiSelect: true,
      });
    }
  }, []);

  const triggerExtraction = useCallback(
    async (sessionId: string) => {
      try {
        setPapers((prev) => prev.map((paper) => ({ ...paper, status: 'extracting' })));
        await backend.extractPaperContent(sessionId);
        setCurrentStage('screen');
        void initializeScreening(sessionId);
      } catch (error) {
        console.error('提取文件内容失败:', error);
      }
    },
    [initializeScreening],
  );

  const handleFileUpload = useCallback(
    async (files: FileList) => {
      setIsProcessing(true);
      const sessionId = await createScreeningSession();
      if (!sessionId) {
        setIsProcessing(false);
        return;
      }

      const filePaths = Array.from(files).map((file) => file.webkitRelativePath || file.name);
      const result = await backend.uploadScreeningFiles(sessionId, filePaths);
      setPapers(result.papers);
      setIsProcessing(false);

      if (result.papers.length > 0) {
        setCurrentStage('extract');
        void triggerExtraction(sessionId);
      }
    },
    [createScreeningSession, triggerExtraction],
  );

  const handleOptionToggle = useCallback((optionKey: string) => {
    setSelectedOptions((prev) => (prev.includes(optionKey) ? prev.filter((key) => key !== optionKey) : [...prev, optionKey]));
  }, []);

  const handleContinue = useCallback(async () => {
    if (!currentNode || selectedOptions.length === 0 || !screeningSessionId) return;

    setScreeningHistory((prev) => [
      ...prev,
      {
        dimension: currentNode.dimension,
        choice: selectedOptions.join(', '),
      },
    ]);

    try {
      const nextNode = await backend.applyScreeningChoice(screeningSessionId, selectedOptions);
      if (nextNode.message.includes('筛选完成')) {
        const results = await backend.completeScreening(screeningSessionId, '');
        setSelectedPapers(new Set(results.map((paper: { id: string }) => paper.id)));
        setCurrentStage('results');
      } else {
        setCurrentNode(nextNode);
        setSelectedOptions([]);
      }
    } catch (error) {
      console.error('提交筛选选择失败:', error);
      if (screeningHistory.length >= 2) {
        setCurrentStage('results');
      } else {
        setCurrentNode({
          id: `node-${screeningHistory.length + 2}`,
          message: '继续缩窄，你更偏好哪类方法论？',
          dimension: '方法类型',
          options: [
            { key: 'empirical', label: 'Empirical Study', paperIds: [], count: 10 },
            { key: 'theoretical', label: 'Theoretical Analysis', paperIds: [], count: 8 },
            { key: 'review', label: 'Survey / Review', paperIds: [], count: 5 },
          ],
          allowMultiSelect: true,
        });
        setSelectedOptions([]);
      }
    }
  }, [currentNode, screeningHistory, screeningSessionId, selectedOptions]);

  const resetFlow = () => {
    setCurrentStage('upload');
    setPapers([]);
    setSelectedPapers(new Set());
    setSelectedOptions([]);
    setScreeningHistory([]);
    setCurrentNode(null);
    setExtractionProgress(0);
    setScreeningSessionId(null);
  };

  const stageChip = (stage: ScreeningStage, label: string) => (
    <div
      className={`rounded-full px-3 py-1.5 text-xs font-medium ${
        currentStage === stage
          ? 'bg-emerald-600 text-white'
          : 'bg-stone-200 text-stone-600 dark:bg-stone-800 dark:text-stone-300'
      }`}
    >
      {label}
    </div>
  );

  const renderUploadStage = () => (
    <div className="rounded-[2rem] border border-stone-200 bg-white/80 px-6 py-10 text-center shadow-sm dark:border-stone-800 dark:bg-stone-900/70">
      <div className="text-5xl">📄</div>
      <h3 className="mt-4 text-2xl font-semibold">上传待筛选论文</h3>
      <p className="mx-auto mt-3 max-w-2xl text-sm leading-7 text-stone-500 dark:text-stone-400">
        当前流程仍处于开发过渡期：上传后会尝试触发提取与决策树筛选；如果后端链路没接通，页面会自动退回到演示节点，方便先验证交互和布局。
      </p>
      <div className="mt-8 flex flex-wrap justify-center gap-3">
        <label className="cursor-pointer rounded-2xl bg-emerald-600 px-6 py-3 text-sm font-medium text-white transition hover:bg-emerald-700">
          <input
            type="file"
            multiple
            accept=".pdf"
            className="hidden"
            onChange={(event) => event.target.files && void handleFileUpload(event.target.files)}
          />
          {isProcessing ? '准备中...' : '选择 PDF 文件'}
        </label>
        <button
          onClick={() => {
            setPapers([
              { id: '1', title: 'Sample Survey', authors: 'Demo Author', abstract: '', year: 2024, status: 'extracted' },
              { id: '2', title: 'Benchmark Paper', authors: 'Demo Author', abstract: '', year: 2025, status: 'extracted' },
            ]);
            setCurrentStage('screen');
            setCurrentNode({
              id: 'demo-node',
              message: '你希望优先保留哪一类样本？',
              dimension: '演示分组',
              options: [
                { key: 'survey', label: 'Survey / Review', paperIds: [], count: 3 },
                { key: 'benchmark', label: 'Benchmark', paperIds: [], count: 5 },
                { key: 'recent', label: 'Recent Progress', paperIds: [], count: 4 },
              ],
              allowMultiSelect: true,
            });
          }}
          className="rounded-2xl border border-stone-200 px-6 py-3 text-sm text-stone-600 transition hover:bg-stone-50 dark:border-stone-700 dark:text-stone-300 dark:hover:bg-stone-900"
        >
          使用演示样本
        </button>
      </div>
    </div>
  );

  const renderExtractStage = () => (
    <div className="rounded-[2rem] border border-stone-200 bg-white/80 px-6 py-10 text-center shadow-sm dark:border-stone-800 dark:bg-stone-900/70">
      <div className="text-5xl">⚙️</div>
      <h3 className="mt-4 text-2xl font-semibold">提取论文内容</h3>
      <p className="mt-3 text-sm text-stone-500 dark:text-stone-400">正在抽取 PDF 文本并准备决策树输入...</p>
      <div className="mx-auto mt-8 max-w-xl">
        <div className="h-2 rounded-full bg-stone-200 dark:bg-stone-800">
          <div className="h-2 rounded-full bg-emerald-600 transition-all" style={{ width: `${Math.round(extractionProgress * 100)}%` }} />
        </div>
        <p className="mt-3 text-sm text-stone-500 dark:text-stone-400">
          已处理 {Math.round(extractionProgress * papers.length)} / {papers.length} 篇论文
        </p>
      </div>
      <button
        onClick={() => setCurrentStage('screen')}
        className="mt-8 rounded-2xl border border-stone-200 px-4 py-2 text-sm text-stone-600 transition hover:bg-stone-50 dark:border-stone-700 dark:text-stone-300 dark:hover:bg-stone-900"
      >
        提取链路未完成时，先进入演示筛选
      </button>
    </div>
  );

  const renderScreenStage = () => {
    if (!currentNode) return null;

    return (
      <div className="rounded-[2rem] border border-stone-200 bg-white/80 p-6 shadow-sm dark:border-stone-800 dark:bg-stone-900/70">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div>
            <h3 className="text-xl font-semibold">{currentNode.message}</h3>
            <p className="mt-2 text-sm text-stone-500 dark:text-stone-400">
              维度：<span className="font-medium text-stone-700 dark:text-stone-200">{currentNode.dimension}</span>
              {currentNode.allowMultiSelect && <span className="ml-2">· 支持多选</span>}
            </p>
          </div>
          <button
            onClick={resetFlow}
            className="rounded-2xl border border-stone-200 px-4 py-2 text-sm text-stone-600 transition hover:bg-stone-50 dark:border-stone-700 dark:text-stone-300 dark:hover:bg-stone-900"
          >
            重新开始
          </button>
        </div>

        {screeningHistory.length > 0 && (
          <div className="mt-5 rounded-2xl border border-stone-200 bg-stone-50/80 p-4 dark:border-stone-800 dark:bg-stone-950/60">
            <p className="text-xs uppercase tracking-[0.18em] text-stone-400 dark:text-stone-500">历史选择</p>
            <div className="mt-3 space-y-2 text-sm text-stone-600 dark:text-stone-300">
              {screeningHistory.map((step, index) => (
                <div key={`${step.dimension}-${index}`}>
                  <span className="font-medium">{step.dimension}：</span>
                  {step.choice}
                </div>
              ))}
            </div>
          </div>
        )}

        <div className="mt-5 grid grid-cols-1 gap-3">
          {currentNode.options.map((option) => {
            const isSelected = selectedOptions.includes(option.key);
            return (
              <button
                key={option.key}
                onClick={() => handleOptionToggle(option.key)}
                className={`rounded-2xl border p-4 text-left transition ${
                  isSelected
                    ? 'border-emerald-500 bg-emerald-50 dark:bg-emerald-950/20'
                    : 'border-stone-200 bg-stone-50/70 hover:border-stone-300 dark:border-stone-800 dark:bg-stone-950/60'
                }`}
              >
                <div className="flex items-center justify-between gap-3">
                  <span className="font-medium">{option.label}</span>
                  <span className="rounded-full bg-white px-2.5 py-1 text-xs text-stone-500 dark:bg-stone-900 dark:text-stone-400">
                    {option.count}
                  </span>
                </div>
                {isSelected && <div className="mt-3 text-sm text-emerald-700 dark:text-emerald-300">已选中</div>}
              </button>
            );
          })}
        </div>

        <div className="mt-6 flex flex-wrap items-center justify-between gap-3 border-t border-stone-200 pt-4 text-sm dark:border-stone-800">
          <div className="text-stone-500 dark:text-stone-400">
            已选择 <span className="font-medium text-stone-900 dark:text-stone-100">{selectedOptions.length}</span> 个选项
          </div>
          <button
            onClick={() => void handleContinue()}
            disabled={selectedOptions.length === 0}
            className="rounded-2xl bg-emerald-600 px-4 py-2.5 font-medium text-white transition hover:bg-emerald-700 disabled:cursor-not-allowed disabled:opacity-60"
          >
            继续下一步
          </button>
        </div>
      </div>
    );
  };

  const renderResultsStage = () => (
    <div className="rounded-[2rem] border border-stone-200 bg-white/80 px-6 py-10 text-center shadow-sm dark:border-stone-800 dark:bg-stone-900/70">
      <div className="text-5xl">🎉</div>
      <h3 className="mt-4 text-2xl font-semibold">筛选完成</h3>
      <p className="mx-auto mt-3 max-w-2xl text-sm leading-7 text-stone-500 dark:text-stone-400">
        这一步目前仍以交互流验证为主。真正的 PDF 抽取、决策树推进与导入论文库链路还需要继续补后端绑定。
      </p>
      <div className="mx-auto mt-8 grid max-w-md grid-cols-1 gap-3 sm:grid-cols-2">
        <div className="rounded-2xl border border-stone-200 bg-stone-50/80 p-4 dark:border-stone-800 dark:bg-stone-950/60">
          <div className="text-2xl font-bold text-emerald-600">{papers.length}</div>
          <div className="mt-1 text-sm text-stone-500 dark:text-stone-400">总论文数</div>
        </div>
        <div className="rounded-2xl border border-stone-200 bg-stone-50/80 p-4 dark:border-stone-800 dark:bg-stone-950/60">
          <div className="text-2xl font-bold text-emerald-600">{selectedPapers.size}</div>
          <div className="mt-1 text-sm text-stone-500 dark:text-stone-400">保留数量</div>
        </div>
      </div>
      <button
        onClick={resetFlow}
        className="mt-8 rounded-2xl bg-stone-900 px-6 py-3 text-sm font-medium text-white transition hover:bg-stone-700 dark:bg-stone-100 dark:text-stone-900 dark:hover:bg-white"
      >
        开始新的筛选
      </button>
    </div>
  );

  return (
    <div className="flex h-full min-h-0 flex-col bg-[#f9f6f0] dark:bg-[#141414]">
      <div className="border-b border-stone-200 p-6 dark:border-stone-800">
        <h2 className="text-lg font-semibold">Screening - 批量筛选</h2>
        <p className="mt-1 text-sm text-stone-500 dark:text-stone-400">
          这块仍是半成品：界面已重整为可用布局，但真实上传、提取与筛选后端尚未完全接通。
        </p>
        <div className="mt-4 flex flex-wrap gap-2">
          {stageChip('upload', '上传')}
          {stageChip('extract', '提取')}
          {stageChip('screen', '决策树筛选')}
          {stageChip('results', '结果')}
        </div>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto p-6">
        <div className="mx-auto max-w-5xl space-y-6">
          {currentStage === 'upload' && renderUploadStage()}
          {currentStage === 'extract' && renderExtractStage()}
          {currentStage === 'screen' && renderScreenStage()}
          {currentStage === 'results' && renderResultsStage()}
        </div>
      </div>
    </div>
  );
};
