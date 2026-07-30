import React, { useEffect, useMemo, useState } from 'react';
import { CheckCircle2, FileUp, Filter, Loader2, Sparkles } from 'lucide-react';
import type { ExtractProgress, Paper, ScreeningDecisionNode, ScreeningSessionDetail } from '../types';
import * as backend from '../lib/backend';
import { errorToUserMessage, isCancellationError } from '../lib/errors';
import { useAppStore } from '../stores/appStore';

type ScreeningStage = 'upload' | 'extract' | 'screen' | 'results';

const stageOrder: ScreeningStage[] = ['upload', 'extract', 'screen', 'results'];
const stageLabel: Record<ScreeningStage, string> = {
  upload: '导入',
  extract: '内容提取',
  screen: '标准筛选',
  results: '结果入库',
};

export const Screening: React.FC = () => {
  const [currentStage, setCurrentStage] = useState<ScreeningStage>('upload');
  const [sessionId, setSessionId] = useState('');
  const [detail, setDetail] = useState<ScreeningSessionDetail | null>(null);
  const [currentNode, setCurrentNode] = useState<ScreeningDecisionNode | null>(null);
  const [selectedOptions, setSelectedOptions] = useState<string[]>([]);
  const [extractionProgress, setExtractionProgress] = useState<ExtractProgress | null>(null);
  const [isBusy, setIsBusy] = useState(false);
  const [isCancelling, setIsCancelling] = useState(false);
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
    setIsCancelling(false);
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
      if (isCancellationError(cause)) {
        setExtractionProgress((previous) => previous ? {
          ...previous,
          status: 'cancelled',
          currentFile: '',
          errorMessage: '',
        } : previous);
        setError(null);
      } else {
        setError(errorToUserMessage(cause, 'PDF 抽取失败'));
      }
      setCurrentStage('extract');
      const latest = await backend.getScreeningSession(nextSessionId).catch(() => null);
      if (latest) {
        setDetail(latest);
      }
    } finally {
      setIsBusy(false);
      setIsCancelling(false);
    }
  };

  const handleCancelTask = async () => {
    if (!sessionId || !isBusy) {
      return;
    }
    setIsCancelling(true);
    try {
      await backend.cancelScreeningTask(sessionId);
    } catch (cause) {
      setError(errorToUserMessage(cause, '停止批量分析失败'));
      setIsCancelling(false);
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
      if (isCancellationError(cause)) {
        setError(null);
      } else {
        setError(errorToUserMessage(cause, '提交筛选选择失败'));
      }
    } finally {
      setIsBusy(false);
      setIsCancelling(false);
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
    <ol className="flex flex-wrap items-center border-y border-[var(--de-rule)]">
      {stageOrder.map((stage, index) => {
        const active = currentStage === stage;
        const completed = stageOrder.indexOf(stage) < stageOrder.indexOf(currentStage);
        return (
          <li
            key={stage}
            className={`flex items-center gap-2 border-r border-[var(--de-rule)] px-4 py-2 text-xs font-medium transition-colors ${
              active
                ? 'bg-[var(--de-accent-soft)] text-[var(--de-accent)]'
                : completed
                  ? 'text-[var(--de-ink)]'
                  : 'text-[var(--de-ink-muted)]'
            }`}
          >
            <span>{completed ? <CheckCircle2 className="h-3.5 w-3.5" /> : index + 1}</span>
            {stageLabel[stage]}
          </li>
        );
      })}
    </ol>
  );

  const renderUploadCard = () => (
    <div className="de-glass p-5">
      <h3 className="text-base font-semibold text-[var(--de-ink)]">导入待分析论文</h3>
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
        className={`mt-4 border border-dashed px-5 py-10 text-center transition-colors ${
          dropActive
            ? 'border-[var(--de-accent)] bg-[var(--de-accent-soft)]'
            : 'border-[var(--de-rule-strong)] bg-[var(--de-surface)]'
        }`}
      >
        <FileUp className="mx-auto h-8 w-8 text-[var(--de-ink-muted)]" />
        <p className="mt-3 text-sm text-[var(--de-ink)]">上传待筛选论文</p>
        <p className="mt-1 text-xs text-[var(--de-ink-muted)]">
          拖拽 PDF 到这里，或者使用文件选择器。
        </p>
        <div className="mt-5 flex flex-wrap justify-center gap-2">
          {hasNativePicker ? (
            <button
              type="button"
              onClick={() => void handleNativePicker()}
              disabled={isBusy}
              className="de-button-primary px-4 py-2 text-sm font-medium disabled:opacity-60"
            >
              {isBusy ? '处理中...' : '选择 PDF 文件'}
            </button>
          ) : (
            <label className="de-button-primary cursor-pointer px-4 py-2 text-sm font-medium">
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
              className="de-button-secondary px-4 py-2 text-sm disabled:opacity-60"
            >
              使用演示样本
            </button>
          )}
        </div>
      </div>

      {papers.length > 0 && (
        <div className="mt-4 divide-y divide-[var(--de-rule)] border-y border-[var(--de-rule)]">
          {papers.map((paper) => (
            <div key={paper.id} className="flex items-center justify-between gap-3 px-1 py-2.5 text-xs">
              <span className="truncate">{paper.fileName}</span>
              <span className={`shrink-0 rounded-[var(--de-radius)] border px-2 py-0.5 ${
                paper.status === 'extracted' || paper.status === 'selected'
                  ? 'border-[var(--de-accent)] bg-[var(--de-accent-soft)] text-[var(--de-accent)]'
                  : paper.status === 'extracting'
                    ? 'border-[var(--de-rule-strong)] bg-[var(--de-surface-muted)] text-[var(--de-warning)]'
                    : 'border-[var(--de-rule)] bg-[var(--de-surface-muted)] text-[var(--de-ink-muted)]'
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
    <div className="de-glass p-5">
      <h3 className="text-base font-semibold text-[var(--de-ink)]">提取论文内容</h3>
      <p className="mt-2 text-sm text-[var(--de-ink-muted)]">
        通过 Python PDF 服务抽取 markdown，并调用强模型生成结构化字段。
      </p>
      <div className="mt-4 h-1.5 bg-[var(--de-surface-muted)]">
        <div className="h-1.5 bg-[var(--de-accent)] transition-[width] duration-150" style={{ width: `${extractionPercent}%` }} />
      </div>
      <p className="mt-2 text-xs text-[var(--de-ink-muted)]">
        已处理 {extractionProgress?.completed ?? 0} / {extractionProgress?.total ?? papers.length}
        {extractionProgress?.currentFile ? ` · 当前：${extractionProgress.currentFile}` : ''}
      </p>

      {error && (
        <div className="mt-4 border border-[var(--de-rule-strong)] bg-[var(--de-surface-muted)] px-3 py-2 text-sm text-[var(--de-danger)]">
          {error}
        </div>
      )}

      <div className="mt-4 flex flex-wrap gap-2">
        {isBusy ? (
          <button
            type="button"
            onClick={() => void handleCancelTask()}
            disabled={isCancelling}
            className="de-button-secondary px-4 py-2 text-sm text-[var(--de-danger)]"
          >
            {isCancelling ? '正在停止' : '停止任务'}
          </button>
        ) : null}
        <button
          onClick={() => sessionId && void runExtraction(sessionId)}
          disabled={!sessionId || isBusy}
          className="de-button-primary px-4 py-2 text-sm font-medium disabled:cursor-not-allowed disabled:opacity-60"
        >
          {isBusy ? '重试中...' : '重新提取'}
        </button>
        {papers.some((paper) => paper.status === 'extracted') && (
          <button
            onClick={() => sessionId && void proceedToAnalysis(sessionId)}
            className="de-button-secondary px-4 py-2 text-sm"
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
        <div className="de-panel p-5">
          <h3 className="text-sm font-semibold text-[var(--de-ink-muted)]">筛选标准</h3>
          <div className="mt-4 border-y border-[var(--de-rule)] bg-[var(--de-surface-muted)] p-4">
            <p className="text-sm text-[var(--de-ink-muted)]">解析完成后，AI 将在这里生成可交互的筛选节点。</p>
          </div>
        </div>
      );
    }

    if (currentStage === 'screen' && currentNode) {
      return (
        <div className="de-panel p-5">
          <h3 className="text-sm font-semibold text-[var(--de-ink-muted)]">筛选标准</h3>

          <div className="mt-4 border border-[var(--de-rule)] bg-[var(--de-surface-muted)] p-4">
            <div className="flex items-center gap-2 text-sm font-semibold text-[var(--de-accent)]">
              <Sparkles className="h-4 w-4" />
              当前判断
            </div>
            <p className="mt-2 text-sm leading-7">{currentNode.message}</p>
            <p className="mt-2 text-xs text-[var(--de-ink-muted)]">维度：{currentNode.dimension}</p>
          </div>

          {history.length > 0 && (
            <div className="mt-4 border-y border-[var(--de-rule)] px-1 py-3 text-xs text-[var(--de-ink-muted)]">
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
                  className={`rounded-[var(--de-radius)] border p-3 text-left text-sm transition-colors ${
                    isSelected
                      ? 'border-[var(--de-accent)] bg-[var(--de-accent-soft)]'
                      : 'border-[var(--de-rule)] bg-[var(--de-surface)] hover:border-[var(--de-rule-strong)]'
                  }`}
                >
                  <div className="flex items-center justify-between">
                    <span>{option.label}</span>
                    <span className="rounded-[var(--de-radius)] bg-[var(--de-surface-muted)] px-2 py-0.5 text-xs text-[var(--de-ink-muted)]">
                      {option.count}
                    </span>
                  </div>
                </button>
              );
            })}
          </div>

          <div className="mt-4 flex items-center justify-between gap-3">
            <p className="text-xs text-[var(--de-ink-muted)]">
              已选择 {selectedOptions.length} 个选项
            </p>
            <div className="flex items-center gap-2">
              {isBusy ? (
                <button
                  type="button"
                  onClick={() => void handleCancelTask()}
                  disabled={isCancelling}
                  className="de-button-secondary px-3 py-2 text-sm text-[var(--de-danger)]"
                >
                  {isCancelling ? '正在停止' : '停止任务'}
                </button>
              ) : null}
              <button
                onClick={() => void handleContinue()}
                disabled={selectedOptions.length === 0 || isBusy}
                className="de-button-primary px-4 py-2 text-sm font-medium disabled:cursor-not-allowed disabled:opacity-60"
              >
                {isBusy ? '处理中...' : '继续下一步'}
              </button>
            </div>
          </div>
        </div>
      );
    }

    if (currentStage === 'results') {
      return (
        <div className="de-panel p-5">
          <h3 className="text-sm font-semibold text-[var(--de-ink-muted)]">筛选结果</h3>
          <div className="mt-4 border-y border-[var(--de-rule)] bg-[var(--de-accent-soft)] p-4">
            <div className="flex items-center gap-2 text-sm font-semibold text-[var(--de-accent)]">
              <CheckCircle2 className="h-4 w-4" />
              筛选完成
            </div>
            <p className="mt-2 text-sm text-[var(--de-ink)]">
              总计 {detail?.session.totalPapers ?? papers.length} 篇，保留 {resultPapers.length} 篇。
            </p>
          </div>
          {importedPapers.length > 0 && (
            <p className="mt-4 border border-[var(--de-accent)] bg-[var(--de-accent-soft)] px-3 py-2 text-sm text-[var(--de-accent)]">
              已成功导入 {importedPapers.length} 篇论文到文库。
            </p>
          )}
        </div>
      );
    }

    return (
      <div className="de-panel p-5">
        <h3 className="text-sm font-semibold text-[var(--de-ink-muted)]">筛选标准</h3>
        <p className="mt-3 text-sm text-[var(--de-ink-muted)]">上传完成后会自动进入提取和筛选流程。</p>
      </div>
    );
  };

  const showActionBar = currentStage === 'screen' || currentStage === 'results';

  return (
    <div className="flex h-full min-h-0 flex-col overflow-hidden bg-[var(--de-paper)] text-[var(--de-ink)]">
      <div className="border-b border-[var(--de-rule)] bg-[var(--de-surface)] px-6 py-4">
        <div className="mx-auto max-w-6xl">
          <h2 className="de-display flex items-center gap-2 text-2xl font-semibold">
            <Filter className="h-5 w-5 text-[var(--de-accent)]" />
            批量论文分析
          </h2>
          <p className="mt-1 text-sm text-[var(--de-ink-muted)]">
            导入 PDF，提取研究内容，按你的标准筛选并保存到文库。
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
              <div className="border border-[var(--de-rule-strong)] bg-[var(--de-surface-muted)] px-4 py-3 text-sm text-[var(--de-danger)]">
                {error}
              </div>
            )}
            {renderDecisionTree()}

            {(currentStage === 'results' || currentStage === 'screen') && resultPapers.length > 0 && (
              <div className="de-panel p-4">
                <h3 className="text-sm font-semibold text-[var(--de-ink-muted)]">保留论文</h3>
                <div className="mt-3 divide-y divide-[var(--de-rule)] border-y border-[var(--de-rule)]">
                  {resultPapers.map((paper) => (
                    <div key={paper.id} className="px-1 py-3">
                      <div className="text-sm font-medium">{paper.title || paper.fileName}</div>
                      <div className="mt-1 text-xs text-[var(--de-ink-muted)]">{paper.authors || paper.fileName}</div>
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
          <div className="pointer-events-auto mx-auto flex max-w-xl items-center justify-between rounded-[var(--de-radius)] border border-[var(--de-rule-strong)] bg-[var(--de-surface)] px-4 py-3 shadow-md">
            <div className="text-sm text-[var(--de-ink-muted)]">
              保留 <span className="font-semibold text-[var(--de-accent)]">{resultPapers.length}</span> 篇论文
            </div>
            <div className="flex gap-2">
              {currentStage === 'results' && importedPapers.length === 0 && (
                <button
                  onClick={() => void handleImport()}
                  disabled={isBusy || resultPapers.length === 0}
                  className="de-button-primary px-4 py-2 text-sm font-medium disabled:cursor-not-allowed disabled:opacity-60"
                >
                  {isBusy ? <Loader2 className="h-4 w-4 animate-spin" /> : '导入到文库'}
                </button>
              )}
              <button
                onClick={resetFlow}
                className="de-button-secondary px-4 py-2 text-sm"
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
