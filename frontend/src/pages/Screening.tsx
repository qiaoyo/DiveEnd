import React, { useEffect, useMemo, useState } from 'react';
import { CheckCircle2, FileUp, Filter, Loader2, Sparkles } from 'lucide-react';
import type { ExtractProgress, Paper, ScreeningDecisionNode, ScreeningSessionDetail } from '../types';
import * as backend from '../lib/backend';
import { errorToUserMessage } from '../lib/errors';
import { useAppStore } from '../stores/appStore';

type ScreeningStage = 'upload' | 'extract' | 'screen' | 'results';

const stageOrder: ScreeningStage[] = ['upload', 'extract', 'screen', 'results'];
const stageLabel: Record<ScreeningStage, string> = {
  upload: '1. Queue',
  extract: '2. Extracting',
  screen: '3. AI Screening',
  results: '4. Results',
};

export const Screening: React.FC = () => {
  const [currentStage, setCurrentStage] = useState<ScreeningStage>('upload');
  const [sessionId, setSessionId] = useState('');
  const [detail, setDetail] = useState<ScreeningSessionDetail | null>(null);
  const [currentNode, setCurrentNode] = useState<ScreeningDecisionNode | null>(null);
  const [selectedOptions, setSelectedOptions] = useState<string[]>([]);
  const [extractionProgress, setExtractionProgress] = useState<ExtractProgress | null>(null);
  const [isBusy, setIsBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [dropActive, setDropActive] = useState(false);
  const [importedPapers, setImportedPapers] = useState<Paper[]>([]);
  const { setPapers, setActiveFolderId } = useAppStore();

  useEffect(() => {
    return backend.onExtractProgress((progress) => {
      if (progress.sessionId && progress.sessionId === sessionId) {
        setExtractionProgress(progress);
      }
    });
  }, [sessionId]);

  const canResolvePaths = backend.canResolveFilePaths();
  const hasNativePicker = backend.hasNativeFilePicker();
  const showDemoSamples = import.meta.env.DEV && !hasNativePicker && !canResolvePaths;
  const papers = detail?.papers ?? [];
  const history = detail?.pathHistory ?? [];

  const resultPapers = useMemo(() => {
    if (!currentNode?.remainingPaperIds?.length) {
      return [];
    }
    const remaining = new Set(currentNode.remainingPaperIds);
    return papers.filter((paper) => remaining.has(paper.id));
  }, [currentNode, papers]);

  const extractionPercent = extractionProgress && extractionProgress.total > 0
    ? Math.round((extractionProgress.completed / extractionProgress.total) * 100)
    : 0;

  const resetFlow = () => {
    setCurrentStage('upload');
    setSessionId('');
    setDetail(null);
    setCurrentNode(null);
    setSelectedOptions([]);
    setExtractionProgress(null);
    setError(null);
    setImportedPapers([]);
    setIsBusy(false);
  };

  const resolveInputPaths = async (files: File[]): Promise<string[]> => {
    if (files.length === 0) {
      return [];
    }

    if (canResolvePaths) {
      return backend.resolveFilePaths(files);
    }

    return files.map((file) => file.webkitRelativePath || file.name);
  };

  const proceedToAnalysis = async (nextSessionId: string) => {
    const sessionDetail = await backend.getScreeningSession(nextSessionId);
    setDetail(sessionDetail);

    const node = await backend.analyzePapers(nextSessionId);
    setCurrentNode(node);
    setSelectedOptions([]);
    setCurrentStage(node.nodeType === 'complete' ? 'results' : 'screen');
  };

  const runExtraction = async (nextSessionId: string) => {
    setCurrentStage('extract');
    setError(null);
    setIsBusy(true);

    try {
      const progress = await backend.extractPaperContent(nextSessionId);
      setExtractionProgress(progress);

      if (progress.status !== 'completed') {
        throw new Error(progress.errorMessage || 'PDF 抽取未能完成，请检查 PDF 服务和模型配置。');
      }

      await proceedToAnalysis(nextSessionId);
    } catch (cause) {
      setError(errorToUserMessage(cause, 'PDF 抽取失败'));
      setCurrentStage('extract');
      const latest = await backend.getScreeningSession(nextSessionId).catch(() => null);
      if (latest) {
        setDetail(latest);
      }
    } finally {
      setIsBusy(false);
    }
  };

  const startScreeningWithPaths = async (filePaths: string[]) => {
    if (filePaths.length === 0) {
      setError(canResolvePaths ? '没有拿到可用的 PDF 路径。' : '当前环境无法读取真实本地路径，请使用桌面文件选择器或拖拽真实 PDF。');
      return;
    }

    setIsBusy(true);
    setError(null);
    setImportedPapers([]);

    try {
      const session = await backend.createScreeningSession(`Screening ${new Date().toLocaleString()}`);
      setSessionId(session.id);

      const nextDetail = await backend.uploadScreeningFiles(session.id, filePaths);
      setDetail(nextDetail);
      setExtractionProgress({
        sessionId: session.id,
        total: nextDetail.papers.length,
        completed: 0,
        currentFile: '',
        status: 'processing',
        errorMessage: '',
      });

      await runExtraction(session.id);
    } catch (cause) {
      setError(errorToUserMessage(cause, '初始化 Screening 失败'));
      setIsBusy(false);
    }
  };

  const handleNativePicker = async () => {
    setError(null);
    try {
      const paths = await backend.selectScreeningPDFs();
      await startScreeningWithPaths(paths);
    } catch (cause) {
      setError(errorToUserMessage(cause, '选择 PDF 文件失败'));
    }
  };

  const handleInputChange = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(event.target.files ?? []);
    try {
      const paths = await resolveInputPaths(files);
      await startScreeningWithPaths(paths);
    } catch (cause) {
      setError(errorToUserMessage(cause, '解析 PDF 文件路径失败'));
    } finally {
      event.target.value = '';
    }
  };

  const handleDrop = async (event: React.DragEvent<HTMLDivElement>) => {
    event.preventDefault();
    event.stopPropagation();
    setDropActive(false);
    try {
      const paths = await resolveInputPaths(Array.from(event.dataTransfer.files ?? []));
      await startScreeningWithPaths(paths);
    } catch (cause) {
      setError(errorToUserMessage(cause, '解析拖拽 PDF 文件失败'));
    }
  };

  const toggleOption = (optionKey: string) => {
    setSelectedOptions((previous) => (
      previous.includes(optionKey)
        ? previous.filter((key) => key !== optionKey)
        : [...previous, optionKey]
    ));
  };

  const handleContinue = async () => {
    if (!sessionId || !currentNode || selectedOptions.length === 0) {
      return;
    }

    setIsBusy(true);
    setError(null);

    try {
      const nextNode = await backend.applyScreeningChoice(sessionId, selectedOptions);
      setCurrentNode(nextNode);
      setSelectedOptions([]);

      const nextDetail = await backend.getScreeningSession(sessionId);
      setDetail(nextDetail);
      setCurrentStage(nextNode.nodeType === 'complete' ? 'results' : 'screen');
    } catch (cause) {
      setError(errorToUserMessage(cause, '提交筛选选择失败'));
    } finally {
      setIsBusy(false);
    }
  };

  const handleImport = async () => {
    if (!sessionId) {
      return;
    }

    setIsBusy(true);
    setError(null);

    try {
      const imported = await backend.completeScreening(sessionId, '');
      setImportedPapers(imported);

      if (imported[0]?.folderId) {
        setActiveFolderId(imported[0].folderId);
        setPapers(await backend.getPapers(imported[0].folderId));
      }

      const nextDetail = await backend.getScreeningSession(sessionId);
      setDetail(nextDetail);
      setCurrentStage('results');
    } catch (cause) {
      setError(errorToUserMessage(cause, '导入论文库失败'));
    } finally {
      setIsBusy(false);
    }
  };

  const renderSteps = () => (
    <div className="flex flex-wrap gap-2">
      {stageOrder.map((stage) => {
        const active = currentStage === stage;
        const completed = stageOrder.indexOf(stage) < stageOrder.indexOf(currentStage);
        return (
          <div
            key={stage}
            className={`rounded-full border px-3 py-1.5 text-xs font-medium transition ${
              active
                ? 'border-violet-400 bg-violet-600 text-white'
                : completed
                  ? 'border-indigo-300 bg-indigo-100 text-indigo-700 dark:border-indigo-500/40 dark:bg-indigo-500/20 dark:text-indigo-200'
                  : 'border-slate-300 bg-slate-100 text-slate-500 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-300'
            }`}
          >
            {stageLabel[stage]}
          </div>
        );
      })}
    </div>
  );

  const renderUploadCard = () => (
    <div className="de-glass rounded-2xl p-5">
      <h3 className="text-sm font-semibold uppercase tracking-[0.18em] text-slate-500 dark:text-slate-300">Upload & Progress</h3>
      <div
        onDragEnter={(event) => {
          event.preventDefault();
          setDropActive(true);
        }}
        onDragOver={(event) => {
          event.preventDefault();
          setDropActive(true);
        }}
        onDragLeave={(event) => {
          event.preventDefault();
          setDropActive(false);
        }}
        onDrop={(event) => void handleDrop(event)}
        className={`mt-4 rounded-2xl border border-dashed px-5 py-10 text-center transition ${
          dropActive
            ? 'border-violet-400 bg-violet-100/70 dark:bg-violet-500/15'
            : 'border-slate-300 bg-white/60 dark:border-slate-600 dark:bg-slate-900/60'
        }`}
      >
        <FileUp className="mx-auto h-8 w-8 text-slate-400" />
        <p className="mt-3 text-sm text-slate-600 dark:text-slate-300">上传待筛选论文</p>
        <p className="mt-1 text-xs text-slate-500 dark:text-slate-400">
          拖拽 PDF 到这里，或者使用文件选择器。
        </p>
        <div className="mt-5 flex flex-wrap justify-center gap-2">
          {hasNativePicker ? (
            <button
              type="button"
              onClick={() => void handleNativePicker()}
              disabled={isBusy}
              className="rounded-xl bg-indigo-600 px-4 py-2 text-sm font-medium text-white transition hover:bg-indigo-500 disabled:opacity-60"
            >
              {isBusy ? '处理中...' : '选择 PDF 文件'}
            </button>
          ) : (
            <label className="cursor-pointer rounded-xl bg-indigo-600 px-4 py-2 text-sm font-medium text-white transition hover:bg-indigo-500">
            <input
              type="file"
              multiple
              accept=".pdf"
              className="hidden"
              onChange={(event) => void handleInputChange(event)}
            />
            {isBusy ? '处理中...' : '选择 PDF 文件'}
            </label>
          )}
          {showDemoSamples && (
            <button
              type="button"
              onClick={() => void startScreeningWithPaths(['/mock/survey.pdf', '/mock/benchmark.pdf'])}
              disabled={isBusy}
              className="rounded-xl border border-slate-300 bg-white/70 px-4 py-2 text-sm text-slate-600 transition hover:bg-slate-50 disabled:opacity-60 dark:border-slate-600 dark:bg-slate-900/70 dark:text-slate-200 dark:hover:bg-slate-800"
            >
              使用演示样本
            </button>
          )}
        </div>
      </div>

      {papers.length > 0 && (
        <div className="mt-4 space-y-2 rounded-2xl border border-slate-200 bg-white/75 p-4 dark:border-slate-700 dark:bg-slate-900/75">
          {papers.map((paper) => (
            <div key={paper.id} className="flex items-center justify-between rounded-lg bg-slate-100/70 px-3 py-2 text-xs dark:bg-slate-800/70">
              <span className="truncate">{paper.fileName}</span>
              <span className={`rounded-full px-2 py-0.5 ${
                paper.status === 'extracted' || paper.status === 'selected'
                  ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/20 dark:text-emerald-200'
                  : paper.status === 'extracting'
                    ? 'bg-amber-100 text-amber-700 dark:bg-amber-500/20 dark:text-amber-200'
                    : 'bg-slate-200 text-slate-600 dark:bg-slate-700 dark:text-slate-200'
              }`}
              >
                {paper.status}
              </span>
            </div>
          ))}
        </div>
      )}
    </div>
  );

  const renderExtractCard = () => (
    <div className="de-glass rounded-2xl p-5">
      <h3 className="text-sm font-semibold uppercase tracking-[0.18em] text-slate-500 dark:text-slate-300">提取论文内容</h3>
      <p className="mt-2 text-sm text-slate-600 dark:text-slate-300">
        通过 Python PDF 服务抽取 markdown，并调用强模型生成结构化字段。
      </p>
      <div className="mt-4 h-2 rounded-full bg-slate-200 dark:bg-slate-800">
        <div className="h-2 rounded-full bg-violet-600 transition-all" style={{ width: `${extractionPercent}%` }} />
      </div>
      <p className="mt-2 text-xs text-slate-500 dark:text-slate-400">
        已处理 {extractionProgress?.completed ?? 0} / {extractionProgress?.total ?? papers.length}
        {extractionProgress?.currentFile ? ` · 当前：${extractionProgress.currentFile}` : ''}
      </p>

      {error && (
        <div className="mt-4 rounded-xl border border-rose-200 bg-rose-50/80 px-3 py-2 text-sm text-rose-700 dark:border-rose-900 dark:bg-rose-500/15 dark:text-rose-200">
          {error}
        </div>
      )}

      <div className="mt-4 flex flex-wrap gap-2">
        <button
          onClick={() => sessionId && void runExtraction(sessionId)}
          disabled={!sessionId || isBusy}
          className="rounded-xl bg-indigo-600 px-4 py-2 text-sm font-medium text-white transition hover:bg-indigo-500 disabled:cursor-not-allowed disabled:opacity-60"
        >
          {isBusy ? '重试中...' : '重新提取'}
        </button>
        {papers.some((paper) => paper.status === 'extracted') && (
          <button
            onClick={() => sessionId && void proceedToAnalysis(sessionId)}
            className="rounded-xl border border-slate-300 bg-white/70 px-4 py-2 text-sm text-slate-600 transition hover:bg-slate-50 dark:border-slate-600 dark:bg-slate-900/70 dark:text-slate-200 dark:hover:bg-slate-800"
          >
            基于已完成论文继续筛选
          </button>
        )}
      </div>
    </div>
  );

  const renderDecisionTree = () => {
    if (currentStage === 'extract') {
      return (
        <div className="de-glass rounded-2xl p-5">
          <h3 className="text-sm font-semibold uppercase tracking-[0.18em] text-slate-500 dark:text-slate-300">AI Decision Tree</h3>
          <div className="mt-4 rounded-2xl border border-slate-200 bg-white/75 p-4 dark:border-slate-700 dark:bg-slate-900/75">
            <p className="text-sm text-slate-600 dark:text-slate-300">解析完成后，AI 将在这里生成可交互的筛选节点。</p>
          </div>
        </div>
      );
    }

    if (currentStage === 'screen' && currentNode) {
      return (
        <div className="de-glass rounded-2xl p-5">
          <h3 className="text-sm font-semibold uppercase tracking-[0.18em] text-slate-500 dark:text-slate-300">AI Decision Tree</h3>

          <div className="mt-4 rounded-2xl border border-violet-300 bg-violet-50/80 p-4 dark:border-violet-500/40 dark:bg-violet-500/15">
            <div className="flex items-center gap-2 text-sm font-semibold text-violet-700 dark:text-violet-200">
              <Sparkles className="h-4 w-4" />
              AI Question
            </div>
            <p className="mt-2 text-sm leading-7">{currentNode.message}</p>
            <p className="mt-2 text-xs text-slate-500 dark:text-slate-300">维度：{currentNode.dimension}</p>
          </div>

          {history.length > 0 && (
            <div className="mt-4 rounded-xl border border-slate-200 bg-white/75 p-3 text-xs text-slate-600 dark:border-slate-700 dark:bg-slate-900/75 dark:text-slate-200">
              {history.map((step, index) => (
                <div key={`${step.dimension}-${index}`} className="mb-1 last:mb-0">
                  <span className="font-semibold">{step.dimension}：</span>{step.choice}
                </div>
              ))}
            </div>
          )}

          <div className="mt-4 grid gap-2">
            {currentNode.options.map((option) => {
              const isSelected = selectedOptions.includes(option.key);
              return (
                <button
                  key={option.key}
                  onClick={() => toggleOption(option.key)}
                  className={`rounded-xl border p-3 text-left text-sm transition ${
                    isSelected
                      ? 'border-indigo-400 bg-indigo-50 dark:border-indigo-500/50 dark:bg-indigo-500/15'
                      : 'border-slate-200 bg-white/80 hover:border-indigo-300 dark:border-slate-700 dark:bg-slate-900/80 dark:hover:border-indigo-500/50'
                  }`}
                >
                  <div className="flex items-center justify-between">
                    <span>{option.label}</span>
                    <span className="rounded-full bg-slate-100 px-2 py-0.5 text-xs text-slate-600 dark:bg-slate-700 dark:text-slate-200">
                      {option.count}
                    </span>
                  </div>
                </button>
              );
            })}
          </div>

          <div className="mt-4 flex items-center justify-between gap-3">
            <p className="text-xs text-slate-500 dark:text-slate-300">
              已选择 {selectedOptions.length} 个选项
            </p>
            <button
              onClick={() => void handleContinue()}
              disabled={selectedOptions.length === 0 || isBusy}
              className="rounded-xl bg-indigo-600 px-4 py-2 text-sm font-medium text-white transition hover:bg-indigo-500 disabled:cursor-not-allowed disabled:opacity-60"
            >
              {isBusy ? '处理中...' : '继续下一步'}
            </button>
          </div>
        </div>
      );
    }

    if (currentStage === 'results') {
      return (
        <div className="de-glass rounded-2xl p-5">
          <h3 className="text-sm font-semibold uppercase tracking-[0.18em] text-slate-500 dark:text-slate-300">Results</h3>
          <div className="mt-4 rounded-xl border border-emerald-200 bg-emerald-50/80 p-4 dark:border-emerald-500/40 dark:bg-emerald-500/15">
            <div className="flex items-center gap-2 text-sm font-semibold text-emerald-700 dark:text-emerald-200">
              <CheckCircle2 className="h-4 w-4" />
              筛选完成
            </div>
            <p className="mt-2 text-sm text-slate-600 dark:text-slate-200">
              总计 {detail?.session.totalPapers ?? papers.length} 篇，保留 {resultPapers.length} 篇。
            </p>
          </div>
          {importedPapers.length > 0 && (
            <p className="mt-4 rounded-xl border border-emerald-200 bg-emerald-50/80 px-3 py-2 text-sm text-emerald-700 dark:border-emerald-500/40 dark:bg-emerald-500/15 dark:text-emerald-200">
              已成功导入 {importedPapers.length} 篇论文到文库。
            </p>
          )}
        </div>
      );
    }

    return (
      <div className="de-glass rounded-2xl p-5">
        <h3 className="text-sm font-semibold uppercase tracking-[0.18em] text-slate-500 dark:text-slate-300">AI Decision Tree</h3>
        <p className="mt-3 text-sm text-slate-600 dark:text-slate-300">上传完成后会自动进入提取和筛选流程。</p>
      </div>
    );
  };

  const showActionBar = currentStage === 'screen' || currentStage === 'results';

  return (
    <div className="flex h-full min-h-0 flex-col overflow-hidden bg-slate-50 text-slate-900 dark:bg-slate-950 dark:text-slate-100">
      <div className="border-b border-slate-200/80 bg-white/70 px-6 py-4 backdrop-blur-md dark:border-slate-700/50 dark:bg-slate-900/55">
        <div className="mx-auto max-w-6xl">
          <h2 className="flex items-center gap-2 text-lg font-semibold">
            <Filter className="h-5 w-5 text-violet-600" />
            Screening Pipeline
          </h2>
          <p className="mt-1 text-sm text-slate-500 dark:text-slate-300">
            真实链路：上传 PDF ➜ 结构化提取 ➜ AI 决策树筛选 ➜ 导入文库
          </p>
          <div className="mt-4">{renderSteps()}</div>
        </div>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto px-6 pb-28 pt-6">
        <div className="mx-auto grid max-w-6xl gap-5 lg:grid-cols-[0.92fr_1.08fr]">
          <div className="space-y-5">
            {currentStage === 'extract' ? renderExtractCard() : renderUploadCard()}
          </div>
          <div className="space-y-5">
            {error && currentStage !== 'extract' && (
              <div className="rounded-xl border border-rose-200 bg-rose-50/80 px-4 py-3 text-sm text-rose-700 dark:border-rose-900 dark:bg-rose-500/15 dark:text-rose-200">
                {error}
              </div>
            )}
            {renderDecisionTree()}

            {(currentStage === 'results' || currentStage === 'screen') && resultPapers.length > 0 && (
              <div className="de-glass rounded-2xl p-4">
                <h3 className="text-sm font-semibold uppercase tracking-[0.18em] text-slate-500 dark:text-slate-300">Selected Papers</h3>
                <div className="mt-3 space-y-2">
                  {resultPapers.map((paper) => (
                    <div key={paper.id} className="rounded-xl border border-slate-200 bg-white/80 px-3 py-2 dark:border-slate-700 dark:bg-slate-900/80">
                      <div className="text-sm font-medium">{paper.title || paper.fileName}</div>
                      <div className="mt-1 text-xs text-slate-500 dark:text-slate-300">{paper.authors || paper.fileName}</div>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>
        </div>
      </div>

      {showActionBar && (
        <div className="pointer-events-none fixed bottom-6 left-1/2 z-30 w-full max-w-6xl -translate-x-1/2 px-6">
          <div className="pointer-events-auto mx-auto flex max-w-xl items-center justify-between rounded-2xl border border-slate-200 bg-white/90 px-4 py-3 shadow-xl backdrop-blur-md dark:border-slate-700 dark:bg-slate-900/90">
            <div className="text-sm text-slate-600 dark:text-slate-200">
              保留 <span className="font-semibold text-indigo-600 dark:text-indigo-300">{resultPapers.length}</span> 篇论文
            </div>
            <div className="flex gap-2">
              {currentStage === 'results' && importedPapers.length === 0 && (
                <button
                  onClick={() => void handleImport()}
                  disabled={isBusy || resultPapers.length === 0}
                  className="rounded-xl bg-indigo-600 px-4 py-2 text-sm font-medium text-white transition hover:bg-indigo-500 disabled:cursor-not-allowed disabled:opacity-60"
                >
                  {isBusy ? <Loader2 className="h-4 w-4 animate-spin" /> : '导入到文库'}
                </button>
              )}
              <button
                onClick={resetFlow}
                className="rounded-xl border border-slate-300 bg-white/70 px-4 py-2 text-sm text-slate-600 transition hover:bg-slate-50 dark:border-slate-600 dark:bg-slate-900/70 dark:text-slate-200 dark:hover:bg-slate-800"
              >
                开始新的筛选
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
