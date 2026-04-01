import { useEffect, useMemo, useState } from 'react';
import {
  ArrowRight,
  Bot,
  CheckCheck,
  FolderPlus,
  Loader2,
  MessageSquarePlus,
  Plus,
  RefreshCw,
  Search,
  Sparkles,
} from 'lucide-react';
import {
  createFolder,
  getDeepStartSession,
  getPapers,
  importPapers,
  replyDeepStartSession,
  rerunDeepStartSearch,
  startDeepStartSession,
  updateDeepStartSelections,
} from '../../lib/backend';
import { useAppStore } from '../../stores/appStore';
import type { DeepStartDirection, SearchPaper } from '../../types';

type BusyAction = 'loading' | 'starting' | 'replying' | 'rerunning' | 'selecting' | 'importing' | null;

function tierStyle(tier: string) {
  switch (tier) {
    case 'core':
      return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300';
    case 'important':
      return 'bg-amber-100 text-amber-700 dark:bg-amber-950 dark:text-amber-300';
    default:
      return 'bg-stone-100 text-stone-600 dark:bg-stone-800 dark:text-stone-300';
  }
}

export function DeepStartPanel() {
  const {
    activeDeepStartSession,
    activeFolderId,
    deepStartSessions,
    folders,
    setActiveDeepStartSession,
    setActiveFolderId,
    setActivePanel,
    setError,
    setFolders,
    setPapers,
    setSelectedPaper,
    upsertDeepStartSession,
  } = useAppStore();

  const [busyAction, setBusyAction] = useState<BusyAction>(null);
  const [isCreatingFolder, setIsCreatingFolder] = useState(false);
  const [isComposerOpen, setIsComposerOpen] = useState(() => !activeDeepStartSession);
  const [newPrompt, setNewPrompt] = useState('');
  const [replyInput, setReplyInput] = useState('');
  const [rerunQuery, setRerunQuery] = useState(activeDeepStartSession?.summary.currentQuery ?? '');

  useEffect(() => {
    if (activeDeepStartSession) {
      setRerunQuery(activeDeepStartSession.summary.currentQuery);
      setIsComposerOpen(false);
    } else {
      setIsComposerOpen(true);
    }
  }, [activeDeepStartSession?.summary.currentQuery, activeDeepStartSession]);

  useEffect(() => {
    const targetFolderId = activeDeepStartSession?.summary.targetFolderId;
    if (!targetFolderId) {
      return;
    }

    let cancelled = false;
    void getPapers(targetFolderId)
      .then((papers) => {
        if (cancelled) {
          return;
        }
        setActiveFolderId(targetFolderId);
        setPapers(papers);
      })
      .catch((error) => {
        if (!cancelled) {
          setError(error instanceof Error ? error.message : '同步目标文件夹失败');
        }
      });

    return () => {
      cancelled = true;
    };
  }, [activeDeepStartSession?.summary.targetFolderId, setActiveFolderId, setError, setPapers]);

  const currentResults = activeDeepStartSession?.currentResults ?? [];
  const currentAnalysis = activeDeepStartSession?.currentAnalysis;
  const selectedPaperIds = useMemo(
    () => new Set(activeDeepStartSession?.selectedPaperIds ?? []),
    [activeDeepStartSession?.selectedPaperIds]
  );
  const paperById = useMemo(
    () => new Map(currentResults.map((paper) => [paper.id, paper])),
    [currentResults]
  );
  const noteByPaperId = useMemo(
    () => new Map((currentAnalysis?.paperNotes ?? []).map((note) => [note.paperId, note])),
    [currentAnalysis?.paperNotes]
  );
  const recommendedPaperIds = useMemo(
    () => new Set(currentAnalysis?.recommendedPaperIds ?? []),
    [currentAnalysis?.recommendedPaperIds]
  );

  const groupedDirections = useMemo(() => {
    const assigned = new Set<string>();
    const groups = (currentAnalysis?.directions ?? []).map((direction) => {
      const uniquePaperIds = direction.paperIds.filter((paperId) => {
        if (!paperById.has(paperId) || assigned.has(paperId)) {
          return false;
        }
        assigned.add(paperId);
        return true;
      });
      return { ...direction, paperIds: uniquePaperIds };
    });

    const remainingPaperIds = currentResults
      .map((paper) => paper.id)
      .filter((paperId) => !assigned.has(paperId));

    if (remainingPaperIds.length > 0) {
      groups.push({
        id: 'remaining',
        name: '其他候选',
        summary: '这些论文还没有被放进明确方向，可以作为补充参考。',
        why: '保留它们，避免遗漏边缘但有价值的候选。',
        paperIds: remainingPaperIds,
      });
    }

    return groups.filter((direction) => direction.paperIds.length > 0 || direction.id !== 'remaining');
  }, [currentAnalysis?.directions, currentResults, paperById]);

  const persistSession = (detail: NonNullable<typeof activeDeepStartSession>) => {
    upsertDeepStartSession(detail);
    if (detail.summary.targetFolderId) {
      setActiveFolderId(detail.summary.targetFolderId);
    }
  };

  const handleLoadSession = async (sessionId: string) => {
    if (!sessionId) {
      setActiveDeepStartSession(null);
      return;
    }

    setBusyAction('loading');
    try {
      const detail = await getDeepStartSession(sessionId);
      persistSession(detail);
    } catch (error) {
      setError(error instanceof Error ? error.message : '加载探索会话失败');
    } finally {
      setBusyAction(null);
    }
  };

  const handleStartSession = async () => {
    const prompt = newPrompt.trim();
    if (!prompt) {
      setError('请先输入研究方向或问题');
      return;
    }

    setBusyAction('starting');
    try {
      const detail = await startDeepStartSession(
        prompt,
        activeDeepStartSession?.summary.targetFolderId || activeFolderId || folders[0]?.id || ''
      );
      persistSession(detail);
      setNewPrompt('');
      setReplyInput('');
    } catch (error) {
      setError(error instanceof Error ? error.message : '创建探索会话失败');
    } finally {
      setBusyAction(null);
    }
  };

  const handleReply = async (message: string) => {
    if (!activeDeepStartSession) {
      setError('请先开始一轮探索');
      return;
    }

    const content = message.trim();
    if (!content) {
      setError('请输入你想补充的筛选偏好');
      return;
    }

    setBusyAction('replying');
    try {
      const detail = await replyDeepStartSession(activeDeepStartSession.summary.id, content);
      persistSession(detail);
      setReplyInput('');
    } catch (error) {
      setError(error instanceof Error ? error.message : '发送 DeepStart 对话失败');
    } finally {
      setBusyAction(null);
    }
  };

  const handleRerun = async (query?: string) => {
    if (!activeDeepStartSession) {
      setError('请先开始一轮探索');
      return;
    }

    const nextQuery = (query ?? rerunQuery).trim();
    if (!nextQuery) {
      setError('请输入新的检索 query');
      return;
    }

    setBusyAction('rerunning');
    try {
      const detail = await rerunDeepStartSearch(activeDeepStartSession.summary.id, nextQuery);
      persistSession(detail);
      setRerunQuery(nextQuery);
    } catch (error) {
      setError(error instanceof Error ? error.message : '重新检索失败');
    } finally {
      setBusyAction(null);
    }
  };

  const handlePersistSelections = async (selectedIds: string[], targetFolderId?: string) => {
    if (!activeDeepStartSession) {
      return;
    }

    setBusyAction('selecting');
    try {
      const detail = await updateDeepStartSelections(
        activeDeepStartSession.summary.id,
        selectedIds,
        targetFolderId ?? activeDeepStartSession.summary.targetFolderId
      );
      persistSession(detail);
    } catch (error) {
      setError(error instanceof Error ? error.message : '保存 DeepStart 勾选状态失败');
    } finally {
      setBusyAction(null);
    }
  };

  const toggleSelect = async (paperId: string) => {
    const next = new Set(selectedPaperIds);
    if (next.has(paperId)) {
      next.delete(paperId);
    } else {
      next.add(paperId);
    }
    await handlePersistSelections([...next]);
  };

  const toggleDirection = async (direction: DeepStartDirection, shouldSelect: boolean) => {
    const next = new Set(selectedPaperIds);
    for (const paperId of direction.paperIds) {
      if (shouldSelect) {
        next.add(paperId);
      } else {
        next.delete(paperId);
      }
    }
    await handlePersistSelections([...next]);
  };

  const handleTargetFolderChange = async (targetFolderId: string) => {
    setActiveFolderId(targetFolderId);
    if (activeDeepStartSession) {
      await handlePersistSelections([...selectedPaperIds], targetFolderId);
    }
  };

  const handleCreateFolder = async () => {
    const name = window.prompt('输入新文件夹名称');
    if (!name?.trim()) return;

    setIsCreatingFolder(true);
    try {
      const folder = await createFolder(name.trim());
      const nextFolders = [...folders.filter((item) => item.id !== folder.id), folder].sort((a, b) =>
        a.createdAt.localeCompare(b.createdAt)
      );
      setFolders(nextFolders);
      await handleTargetFolderChange(folder.id);
    } catch (error) {
      setError(error instanceof Error ? error.message : '创建文件夹失败');
    } finally {
      setIsCreatingFolder(false);
    }
  };

  const handleImportSelected = async () => {
    if (!activeDeepStartSession) {
      setError('请先开始一轮探索');
      return;
    }

    const folderId =
      activeDeepStartSession.summary.targetFolderId || activeFolderId || folders[0]?.id || '';
    if (!folderId) {
      setError('请先创建或选择一个文件夹');
      return;
    }

    const selected = activeDeepStartSession.currentResults.filter((paper) =>
      selectedPaperIds.has(paper.id)
    );
    if (!selected.length) {
      setError('请先选中至少一篇论文');
      return;
    }

    setBusyAction('importing');
    try {
      const imported = await importPapers(folderId, selected);
      const papers = await getPapers(folderId);
      setActiveFolderId(folderId);
      setPapers(papers);
      if (imported[0]) {
        setSelectedPaper(imported[0]);
        setActivePanel('deepread');
      }
    } catch (error) {
      setError(error instanceof Error ? error.message : '导入论文失败');
    } finally {
      setBusyAction(null);
    }
  };

  const renderPaperCard = (paper: SearchPaper) => {
    const note = noteByPaperId.get(paper.id);
    const isSelected = selectedPaperIds.has(paper.id);
    const isRecommended = recommendedPaperIds.has(paper.id);

    return (
      <button
        key={paper.id}
        type="button"
        onClick={() => void toggleSelect(paper.id)}
        className={`rounded-3xl border p-5 text-left transition ${
          isSelected
            ? 'border-emerald-500 bg-emerald-50 shadow-sm dark:bg-emerald-950/20'
            : isRecommended
              ? 'border-amber-300 bg-white shadow-sm hover:border-amber-400 dark:border-amber-800 dark:bg-stone-900/70'
              : 'border-stone-200 bg-white hover:border-emerald-300 hover:shadow-sm dark:border-stone-800 dark:bg-stone-900/70'
        }`}
      >
        <div className="flex items-start justify-between gap-4">
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <h3 className="text-base font-semibold leading-6">{paper.title}</h3>
              {note && (
                <span className={`rounded-full px-2 py-1 text-[11px] font-medium ${tierStyle(note.tier)}`}>
                  {note.tier}
                </span>
              )}
            </div>
            <p className="mt-2 text-sm text-stone-600 dark:text-stone-300">
              {paper.authors || '作者信息缺失'}
            </p>
          </div>
          <div
            className={`mt-1 h-5 w-5 shrink-0 rounded-full border ${
              isSelected
                ? 'border-emerald-600 bg-emerald-600'
                : 'border-stone-300 dark:border-stone-600'
            }`}
          />
        </div>

        <div className="mt-3 flex flex-wrap items-center gap-2 text-xs text-stone-500 dark:text-stone-400">
          <span>{paper.journal || '未知来源'}</span>
          <span>·</span>
          <span>{paper.year || '年份未知'}</span>
          {paper.category && (
            <span className="rounded-full bg-stone-100 px-2 py-1 dark:bg-stone-800">{paper.category}</span>
          )}
        </div>

        {note?.reason && (
          <p className="mt-3 rounded-2xl bg-stone-50 px-3 py-2 text-sm leading-6 text-stone-600 dark:bg-stone-950/70 dark:text-stone-300">
            {note.reason}
          </p>
        )}

        <p className="mt-4 line-clamp-4 text-sm leading-6 text-stone-600 dark:text-stone-300">
          {paper.abstract || '暂无摘要。'}
        </p>
      </button>
    );
  };

  return (
    <div className="flex h-full min-h-0 flex-col bg-[#f9f6f0] dark:bg-[#141414]">
      <div className="border-b border-stone-200 p-6 dark:border-stone-800">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div>
            <h2 className="flex items-center gap-2 text-lg font-semibold">
              <Sparkles className="h-5 w-5 text-emerald-600" />
              DeepStart - AI 辅助论文筛选
            </h2>
            <p className="mt-1 text-sm text-stone-500 dark:text-stone-400">
              先检索候选论文，再让模型帮你分方向、标注代表作，并通过多轮对话继续缩窄。
            </p>
          </div>
          <button
            onClick={() => setActivePanel('deepread')}
            className="flex items-center gap-2 rounded-2xl border border-stone-200 bg-white px-4 py-2 text-sm text-stone-600 transition hover:border-emerald-400 hover:text-emerald-700 dark:border-stone-700 dark:bg-stone-900 dark:text-stone-300"
          >
            查看 DeepRead
            <ArrowRight className="h-4 w-4" />
          </button>
        </div>
      </div>

      <div className="border-b border-stone-200 p-4 dark:border-stone-800">
        <div className="grid gap-3 xl:grid-cols-[220px_minmax(180px,220px)_minmax(200px,1fr)_auto_auto]">
          <select
            value={activeDeepStartSession?.summary.id ?? ''}
            onChange={(event) => void handleLoadSession(event.target.value)}
            className="rounded-2xl border border-stone-200 bg-white px-3 py-3 text-sm shadow-sm outline-none transition focus:border-emerald-500 dark:border-stone-700 dark:bg-stone-900"
          >
            <option value="">选择探索会话</option>
            {deepStartSessions.map((session) => (
              <option key={session.id} value={session.id}>
                {session.title}
              </option>
            ))}
          </select>

          <button
            onClick={() => setIsComposerOpen((current) => !current)}
            className="flex items-center justify-center gap-2 rounded-2xl border border-stone-200 bg-white px-4 py-3 text-sm text-stone-600 transition hover:border-emerald-400 hover:text-emerald-700 dark:border-stone-700 dark:bg-stone-900 dark:text-stone-300"
          >
            <MessageSquarePlus className="h-4 w-4" />
            新探索
          </button>

          <select
            value={activeDeepStartSession?.summary.targetFolderId || activeFolderId || folders[0]?.id || ''}
            onChange={(event) => void handleTargetFolderChange(event.target.value)}
            className="rounded-2xl border border-stone-200 bg-white px-3 py-3 text-sm shadow-sm outline-none transition focus:border-emerald-500 dark:border-stone-700 dark:bg-stone-900"
          >
            {folders.map((folder) => (
              <option key={folder.id} value={folder.id}>
                入库到：{folder.name}
              </option>
            ))}
          </select>

          <div className="flex gap-3">
            <input
              type="text"
              value={rerunQuery}
              onChange={(event) => setRerunQuery(event.target.value)}
              onKeyDown={(event) => event.key === 'Enter' && void handleRerun()}
              placeholder="手动改 query 重新检索"
              disabled={!activeDeepStartSession}
              className="min-w-0 flex-1 rounded-2xl border border-stone-200 bg-white px-4 py-3 text-sm shadow-sm outline-none transition focus:border-emerald-500 disabled:cursor-not-allowed disabled:opacity-60 dark:border-stone-700 dark:bg-stone-900"
            />
            <button
              onClick={() => void handleRerun()}
              disabled={!activeDeepStartSession || busyAction !== null}
              className="flex items-center justify-center gap-2 rounded-2xl bg-emerald-600 px-4 py-3 text-sm font-medium text-white transition hover:bg-emerald-700 disabled:cursor-not-allowed disabled:opacity-60"
            >
              {busyAction === 'rerunning' ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : (
                <RefreshCw className="h-4 w-4" />
              )}
              重搜
            </button>
          </div>

          <div className="flex gap-2">
            <button
              onClick={() => void handleCreateFolder()}
              disabled={isCreatingFolder}
              className="flex items-center gap-2 rounded-2xl border border-stone-200 px-4 py-3 text-sm text-stone-600 transition hover:border-emerald-400 hover:text-emerald-700 disabled:opacity-60 dark:border-stone-700 dark:text-stone-300"
            >
              <FolderPlus className="h-4 w-4" />
              新建文件夹
            </button>
            <button
              onClick={() => void handleImportSelected()}
              disabled={!activeDeepStartSession || selectedPaperIds.size === 0 || busyAction !== null}
              className="flex items-center gap-2 rounded-2xl bg-stone-900 px-4 py-3 text-sm font-medium text-white transition hover:bg-stone-700 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-stone-100 dark:text-stone-900 dark:hover:bg-white"
            >
              <Plus className="h-4 w-4" />
              导入选中 ({selectedPaperIds.size})
            </button>
          </div>
        </div>

        {(isComposerOpen || !activeDeepStartSession) && (
          <div className="mt-4 rounded-[1.75rem] border border-dashed border-stone-300 bg-white/80 p-4 dark:border-stone-700 dark:bg-stone-900/60">
            <div className="flex flex-col gap-3 xl:flex-row">
              <input
                type="text"
                value={newPrompt}
                onChange={(event) => setNewPrompt(event.target.value)}
                onKeyDown={(event) => event.key === 'Enter' && void handleStartSession()}
                placeholder="输入研究问题，例如：我想先梳理 LLM code agent 的代表性论文和 tech report"
                className="flex-1 rounded-2xl border border-stone-200 bg-white px-4 py-3 text-sm shadow-sm outline-none transition focus:border-emerald-500 dark:border-stone-700 dark:bg-stone-900"
              />
              <button
                onClick={() => void handleStartSession()}
                disabled={busyAction !== null || !newPrompt.trim()}
                className="flex min-w-[132px] items-center justify-center gap-2 rounded-2xl bg-emerald-600 px-4 py-3 text-sm font-medium text-white transition hover:bg-emerald-700 disabled:cursor-not-allowed disabled:opacity-60"
              >
                {busyAction === 'starting' ? (
                  <>
                    <Loader2 className="h-4 w-4 animate-spin" />
                    生成中
                  </>
                ) : (
                  <>
                    <Search className="h-4 w-4" />
                    开始探索
                  </>
                )}
              </button>
            </div>
          </div>
        )}
      </div>

      {!activeDeepStartSession ? (
        <div className="flex flex-1 items-center justify-center p-6">
          <div className="max-w-2xl rounded-[2rem] border border-dashed border-stone-300 bg-white/70 px-8 py-10 text-center dark:border-stone-700 dark:bg-stone-900/50">
            <Bot className="mx-auto mb-4 h-12 w-12 text-stone-300 dark:text-stone-600" />
            <h3 className="text-lg font-medium">从一个问题开始，让 AI 先帮你搭领域地图</h3>
            <p className="mt-3 text-sm leading-7 text-stone-500 dark:text-stone-400">
              例如“我想先梳理 VLA 的代表性论文和最近进展”。DeepStart 会先检索候选论文，再按方向分组、推荐代表作，并通过对话继续缩窄。
            </p>
          </div>
        </div>
      ) : (
        <div className="grid flex-1 min-h-0 grid-rows-[minmax(280px,42%)_minmax(0,58%)]">
          <div className="min-h-0 border-b border-stone-200 p-6 dark:border-stone-800">
            <div className="grid h-full min-h-0 gap-4 xl:grid-cols-[1.4fr_0.9fr]">
              <div className="flex min-h-0 flex-col rounded-[1.75rem] border border-stone-200 bg-white/80 dark:border-stone-800 dark:bg-stone-900/70">
                <div className="border-b border-stone-200 px-5 py-4 dark:border-stone-800">
                  <div className="flex items-center justify-between gap-4">
                    <div>
                      <p className="text-sm font-medium">{activeDeepStartSession.summary.title}</p>
                      <p className="mt-1 text-xs text-stone-500 dark:text-stone-400">
                        当前检索 query：{activeDeepStartSession.summary.currentQuery}
                      </p>
                    </div>
                    <span className="rounded-full bg-stone-100 px-3 py-1 text-[11px] uppercase tracking-[0.18em] text-stone-500 dark:bg-stone-800 dark:text-stone-400">
                      AI Chat
                    </span>
                  </div>
                </div>

                <div className="flex-1 space-y-3 overflow-y-auto px-5 py-4">
                  {activeDeepStartSession.messages.map((message) => (
                    <div
                      key={message.id}
                      className={`max-w-[88%] rounded-3xl px-4 py-3 text-sm leading-7 ${
                        message.role === 'assistant'
                          ? 'bg-stone-100 text-stone-700 dark:bg-stone-800 dark:text-stone-200'
                          : 'ml-auto bg-emerald-600 text-white'
                      }`}
                    >
                      {message.content}
                    </div>
                  ))}
                </div>

                <div className="border-t border-stone-200 p-4 dark:border-stone-800">
                  <div className="flex gap-3">
                    <input
                      type="text"
                      value={replyInput}
                      onChange={(event) => setReplyInput(event.target.value)}
                      onKeyDown={(event) => event.key === 'Enter' && void handleReply(replyInput)}
                      placeholder="继续告诉 AI：我更想先看 survey / benchmark / 最近两年的工作..."
                      className="flex-1 rounded-2xl border border-stone-200 bg-white px-4 py-3 text-sm shadow-sm outline-none transition focus:border-emerald-500 dark:border-stone-700 dark:bg-stone-900"
                    />
                    <button
                      onClick={() => void handleReply(replyInput)}
                      disabled={busyAction !== null || !replyInput.trim()}
                      className="flex min-w-[120px] items-center justify-center gap-2 rounded-2xl bg-emerald-600 px-4 py-3 text-sm font-medium text-white transition hover:bg-emerald-700 disabled:cursor-not-allowed disabled:opacity-60"
                    >
                      {busyAction === 'replying' ? (
                        <Loader2 className="h-4 w-4 animate-spin" />
                      ) : (
                        <Bot className="h-4 w-4" />
                      )}
                      继续筛选
                    </button>
                  </div>
                </div>
              </div>

              <div className="flex min-h-0 flex-col gap-4">
                <div className="rounded-[1.75rem] border border-stone-200 bg-white/80 p-5 dark:border-stone-800 dark:bg-stone-900/70">
                  <div className="flex items-center gap-2">
                    <Sparkles className="h-4 w-4 text-emerald-600" />
                    <span className="text-sm font-medium">当前 AI 判断</span>
                  </div>
                  <p className="mt-3 text-sm leading-7 text-stone-600 dark:text-stone-300">
                    {currentAnalysis?.overview || '这一轮还没有生成结构化分析。'}
                  </p>
                </div>

                <div className="min-h-0 rounded-[1.75rem] border border-stone-200 bg-white/80 p-5 dark:border-stone-800 dark:bg-stone-900/70">
                  <div className="space-y-4 overflow-y-auto">
                    <div>
                      <p className="text-xs uppercase tracking-[0.2em] text-stone-500 dark:text-stone-400">
                        建议追问
                      </p>
                      <div className="mt-3 flex flex-wrap gap-2">
                        {(currentAnalysis?.followUpQuestions ?? []).map((question) => (
                          <button
                            key={question}
                            onClick={() => void handleReply(question)}
                            disabled={busyAction !== null}
                            className="rounded-full border border-stone-200 bg-white px-3 py-2 text-sm text-stone-600 transition hover:border-emerald-400 hover:text-emerald-700 disabled:opacity-60 dark:border-stone-700 dark:bg-stone-950 dark:text-stone-300"
                          >
                            {question}
                          </button>
                        ))}
                      </div>
                    </div>

                    <div>
                      <p className="text-xs uppercase tracking-[0.2em] text-stone-500 dark:text-stone-400">
                        建议扩搜 Query
                      </p>
                      <div className="mt-3 flex flex-wrap gap-2">
                        {(currentAnalysis?.suggestedQueries ?? []).map((query) => (
                          <button
                            key={query}
                            onClick={() => void handleRerun(query)}
                            disabled={busyAction !== null}
                            className="rounded-full bg-stone-900 px-3 py-2 text-sm text-white transition hover:bg-stone-700 disabled:opacity-60 dark:bg-stone-100 dark:text-stone-900 dark:hover:bg-white"
                          >
                            {query}
                          </button>
                        ))}
                      </div>
                    </div>

                    <div className="rounded-2xl bg-stone-50 p-4 dark:bg-stone-950/70">
                      <div className="flex items-center gap-2 text-sm font-medium">
                        <CheckCheck className="h-4 w-4 text-emerald-600" />
                        当前已选 {selectedPaperIds.size} 篇论文
                      </div>
                      <p className="mt-2 text-sm leading-6 text-stone-500 dark:text-stone-400">
                        你可以先让 AI 帮你缩窄，再在下方按方向分组的卡片里手动确认导入。
                      </p>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div className="min-h-0 overflow-y-auto p-6">
            {currentResults.length === 0 ? (
              <div className="flex h-full flex-col items-center justify-center rounded-[2rem] border border-dashed border-stone-300 bg-white/60 px-6 text-center dark:border-stone-700 dark:bg-stone-900/50">
                <Search className="mb-4 h-12 w-12 text-stone-300 dark:text-stone-600" />
                <h3 className="text-lg font-medium">这一轮还没有拿到候选论文</h3>
                <p className="mt-2 max-w-xl text-sm leading-7 text-stone-500 dark:text-stone-400">
                  你可以直接点击上面的建议 query，或者告诉 AI 你更想看 survey、benchmark、recent progress 还是具体应用。
                </p>
              </div>
            ) : (
              <div className="space-y-6">
                {groupedDirections.map((direction) => {
                  const directionSelectedCount = direction.paperIds.filter((paperId) =>
                    selectedPaperIds.has(paperId)
                  ).length;
                  return (
                    <section
                      key={direction.id}
                      className="rounded-[2rem] border border-stone-200 bg-white/80 p-5 dark:border-stone-800 dark:bg-stone-900/70"
                    >
                      <div className="flex flex-wrap items-start justify-between gap-4">
                        <div>
                          <div className="flex items-center gap-2">
                            <h3 className="text-lg font-semibold">{direction.name}</h3>
                            <span className="rounded-full bg-stone-100 px-2.5 py-1 text-[11px] uppercase tracking-[0.18em] text-stone-500 dark:bg-stone-800 dark:text-stone-400">
                              {direction.paperIds.length} papers
                            </span>
                          </div>
                          <p className="mt-2 text-sm leading-7 text-stone-600 dark:text-stone-300">
                            {direction.summary}
                          </p>
                          <p className="mt-1 text-sm leading-7 text-stone-500 dark:text-stone-400">
                            {direction.why}
                          </p>
                        </div>
                        <div className="flex gap-2">
                          <button
                            onClick={() => void toggleDirection(direction, true)}
                            disabled={busyAction !== null}
                            className="rounded-full border border-stone-200 px-3 py-2 text-sm text-stone-600 transition hover:border-emerald-400 hover:text-emerald-700 disabled:opacity-60 dark:border-stone-700 dark:text-stone-300"
                          >
                            全选这组 ({directionSelectedCount}/{direction.paperIds.length})
                          </button>
                          <button
                            onClick={() => void toggleDirection(direction, false)}
                            disabled={busyAction !== null}
                            className="rounded-full border border-stone-200 px-3 py-2 text-sm text-stone-600 transition hover:border-stone-400 hover:text-stone-900 disabled:opacity-60 dark:border-stone-700 dark:text-stone-300"
                          >
                            取消这组
                          </button>
                        </div>
                      </div>

                      <div className="mt-5 grid gap-4 xl:grid-cols-2">
                        {direction.paperIds
                          .map((paperId) => paperById.get(paperId))
                          .filter((paper): paper is SearchPaper => Boolean(paper))
                          .map((paper) => renderPaperCard(paper))}
                      </div>
                    </section>
                  );
                })}
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
