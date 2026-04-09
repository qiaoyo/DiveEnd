import React, { useEffect, useMemo, useState } from 'react';
import type { ExtractProgress, Paper, ScreeningDecisionNode, ScreeningSessionDetail } from '../types';
import * as backend from '../lib/backend';
import { useAppStore } from '../stores/appStore';

type ScreeningStage = 'upload' | 'extract' | 'screen' | 'results';

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
      const message = cause instanceof Error ? cause.message : 'PDF 抽取失败';
      setError(message);
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
      setError(canResolvePaths ? '没有拿到可用的 PDF 路径。' : '浏览器预览模式下只能用演示样本或本地 mock 路径。');
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
      setError(cause instanceof Error ? cause.message : '初始化 Screening 失败');
      setIsBusy(false);
    }
  };

  const handleInputChange = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(event.target.files ?? []);
    const paths = await resolveInputPaths(files);
    await startScreeningWithPaths(paths);
    event.target.value = '';
  };

  const handleDrop = async (event: React.DragEvent<HTMLDivElement>) => {
    event.preventDefault();
    event.stopPropagation();
    setDropActive(false);
    const paths = await resolveInputPaths(Array.from(event.dataTransfer.files ?? []));
    await startScreeningWithPaths(paths);
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
      setError(cause instanceof Error ? cause.message : '提交筛选选择失败');
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
      setError(cause instanceof Error ? cause.message : '导入论文库失败');
    } finally {
      setIsBusy(false);
    }
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
        {canResolvePaths
          ? '当前运行在 Wails 环境，会把你选择或拖入的 PDF 解析成真实本地路径，再交给 Go 和 PDF 服务处理。'
          : '当前是浏览器预览模式。你仍然可以体验页面交互，但真实路径解析与本地 PDF 提取需要在 Wails 桌面应用里运行。'}
      </p>

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
        className={`mx-auto mt-8 max-w-3xl rounded-[2rem] border border-dashed px-6 py-12 transition ${
          dropActive
            ? 'border-emerald-500 bg-emerald-50/80 dark:bg-emerald-950/20'
            : 'border-stone-300 bg-stone-50/70 dark:border-stone-700 dark:bg-stone-950/40'
        }`}
      >
        <p className="text-sm text-stone-500 dark:text-stone-400">拖拽 PDF 到这里，或者直接用文件选择器。</p>
        <div className="mt-6 flex flex-wrap justify-center gap-3">
          <label className="cursor-pointer rounded-2xl bg-emerald-600 px-6 py-3 text-sm font-medium text-white transition hover:bg-emerald-700">
            <input
              type="file"
              multiple
              accept=".pdf"
              className="hidden"
              onChange={(event) => void handleInputChange(event)}
            />
            {isBusy ? '处理中...' : '选择 PDF 文件'}
          </label>
          {!canResolvePaths && (
            <button
              onClick={() => void startScreeningWithPaths(['/mock/survey.pdf', '/mock/benchmark.pdf'])}
              className="rounded-2xl border border-stone-200 px-6 py-3 text-sm text-stone-600 transition hover:bg-stone-50 dark:border-stone-700 dark:text-stone-300 dark:hover:bg-stone-900"
            >
              使用演示样本
            </button>
          )}
        </div>
      </div>
    </div>
  );

  const renderExtractStage = () => (
    <div className="rounded-[2rem] border border-stone-200 bg-white/80 px-6 py-10 text-center shadow-sm dark:border-stone-800 dark:bg-stone-900/70">
      <div className="text-5xl">⚙️</div>
      <h3 className="mt-4 text-2xl font-semibold">提取论文内容</h3>
      <p className="mt-3 text-sm text-stone-500 dark:text-stone-400">
        通过 Python PDF 服务抽取 markdown，并调用配置好的强模型生成结构化信息。
      </p>
      <div className="mx-auto mt-8 max-w-xl">
        <div className="h-2 rounded-full bg-stone-200 dark:bg-stone-800">
          <div className="h-2 rounded-full bg-emerald-600 transition-all" style={{ width: `${extractionPercent}%` }} />
        </div>
        <p className="mt-3 text-sm text-stone-500 dark:text-stone-400">
          已处理 {extractionProgress?.completed ?? 0} / {extractionProgress?.total ?? papers.length} 篇论文
          {extractionProgress?.currentFile ? ` · 当前：${extractionProgress.currentFile}` : ''}
        </p>
      </div>
      {error && (
        <div className="mx-auto mt-6 max-w-2xl rounded-2xl border border-rose-200 bg-rose-50/80 px-4 py-3 text-left text-sm text-rose-700 dark:border-rose-900 dark:bg-rose-950/20 dark:text-rose-200">
          {error}
        </div>
      )}
      <div className="mt-8 flex flex-wrap justify-center gap-3">
        <button
          onClick={() => sessionId && void runExtraction(sessionId)}
          disabled={!sessionId || isBusy}
          className="rounded-2xl bg-emerald-600 px-4 py-2.5 text-sm font-medium text-white transition hover:bg-emerald-700 disabled:cursor-not-allowed disabled:opacity-60"
        >
          {isBusy ? '重试中...' : '重新提取'}
        </button>
        {papers.some((paper) => paper.status === 'extracted') && (
          <button
            onClick={() => sessionId && void proceedToAnalysis(sessionId)}
            className="rounded-2xl border border-stone-200 px-4 py-2.5 text-sm text-stone-600 transition hover:bg-stone-50 dark:border-stone-700 dark:text-stone-300 dark:hover:bg-stone-900"
          >
            基于已完成论文继续筛选
          </button>
        )}
      </div>
    </div>
  );

  const renderScreenStage = () => {
    if (!currentNode) {
      return null;
    }

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

        {history.length > 0 && (
          <div className="mt-5 rounded-2xl border border-stone-200 bg-stone-50/80 p-4 dark:border-stone-800 dark:bg-stone-950/60">
            <p className="text-xs uppercase tracking-[0.18em] text-stone-400 dark:text-stone-500">历史选择</p>
            <div className="mt-3 space-y-2 text-sm text-stone-600 dark:text-stone-300">
              {history.map((step, index) => (
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
                onClick={() => toggleOption(option.key)}
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
            disabled={selectedOptions.length === 0 || isBusy}
            className="rounded-2xl bg-emerald-600 px-4 py-2.5 font-medium text-white transition hover:bg-emerald-700 disabled:cursor-not-allowed disabled:opacity-60"
          >
            {isBusy ? '处理中...' : '继续下一步'}
          </button>
        </div>
      </div>
    );
  };

  const renderResultsStage = () => (
    <div className="rounded-[2rem] border border-stone-200 bg-white/80 px-6 py-10 shadow-sm dark:border-stone-800 dark:bg-stone-900/70">
      <div className="text-center">
        <div className="text-5xl">🎯</div>
        <h3 className="mt-4 text-2xl font-semibold">筛选完成</h3>
        <p className="mx-auto mt-3 max-w-2xl text-sm leading-7 text-stone-500 dark:text-stone-400">
          当前剩余的论文已经缩小到适合直接导入文库的规模。确认后会写入 SQLite 文库并保留原始 PDF 路径。
        </p>
      </div>

      <div className="mx-auto mt-8 grid max-w-md grid-cols-1 gap-3 sm:grid-cols-2">
        <div className="rounded-2xl border border-stone-200 bg-stone-50/80 p-4 dark:border-stone-800 dark:bg-stone-950/60">
          <div className="text-2xl font-bold text-emerald-600">{detail?.session.totalPapers ?? papers.length}</div>
          <div className="mt-1 text-sm text-stone-500 dark:text-stone-400">总论文数</div>
        </div>
        <div className="rounded-2xl border border-stone-200 bg-stone-50/80 p-4 dark:border-stone-800 dark:bg-stone-950/60">
          <div className="text-2xl font-bold text-emerald-600">{resultPapers.length}</div>
          <div className="mt-1 text-sm text-stone-500 dark:text-stone-400">保留数量</div>
        </div>
      </div>

      {resultPapers.length > 0 && (
        <div className="mt-8 space-y-3">
          {resultPapers.map((paper) => (
            <div key={paper.id} className="rounded-2xl border border-stone-200 bg-stone-50/80 p-4 dark:border-stone-800 dark:bg-stone-950/60">
              <div className="font-medium">{paper.title || paper.fileName}</div>
              <div className="mt-1 text-sm text-stone-500 dark:text-stone-400">{paper.authors || paper.fileName}</div>
              {paper.abstract && (
                <p className="mt-2 text-sm leading-6 text-stone-600 dark:text-stone-300">{paper.abstract}</p>
              )}
            </div>
          ))}
        </div>
      )}

      {importedPapers.length > 0 && (
        <div className="mt-8 rounded-2xl border border-emerald-200 bg-emerald-50/80 p-4 text-sm text-emerald-700 dark:border-emerald-900 dark:bg-emerald-950/20 dark:text-emerald-200">
          已成功导入 {importedPapers.length} 篇论文到文库。
        </div>
      )}

      <div className="mt-8 flex flex-wrap justify-center gap-3">
        {importedPapers.length === 0 && (
          <button
            onClick={() => void handleImport()}
            disabled={isBusy || resultPapers.length === 0}
            className="rounded-2xl bg-emerald-600 px-6 py-3 text-sm font-medium text-white transition hover:bg-emerald-700 disabled:cursor-not-allowed disabled:opacity-60"
          >
            {isBusy ? '导入中...' : '导入到文库'}
          </button>
        )}
        <button
          onClick={resetFlow}
          className="rounded-2xl border border-stone-200 px-6 py-3 text-sm text-stone-600 transition hover:bg-stone-50 dark:border-stone-700 dark:text-stone-300 dark:hover:bg-stone-900"
        >
          开始新的筛选
        </button>
      </div>
    </div>
  );

  return (
    <div className="flex h-full min-h-0 flex-col bg-[#f9f6f0] dark:bg-[#141414]">
      <div className="border-b border-stone-200 p-6 dark:border-stone-800">
        <h2 className="text-lg font-semibold">Screening - 批量筛选</h2>
        <p className="mt-1 text-sm text-stone-500 dark:text-stone-400">
          现在这条链路会真实调用 Go 后端、SQLite 和 PDF 服务，而不是继续停留在演示节点。
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
          {error && currentStage !== 'extract' && (
            <div className="rounded-2xl border border-rose-200 bg-rose-50/80 px-4 py-3 text-sm text-rose-700 dark:border-rose-900 dark:bg-rose-950/20 dark:text-rose-200">
              {error}
            </div>
          )}
          {currentStage === 'upload' && renderUploadStage()}
          {currentStage === 'extract' && renderExtractStage()}
          {currentStage === 'screen' && renderScreenStage()}
          {currentStage === 'results' && renderResultsStage()}
        </div>
      </div>
    </div>
  );
};
