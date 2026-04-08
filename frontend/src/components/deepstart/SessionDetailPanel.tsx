import { useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  ArrowLeft,
  Bot,
  CheckCheck,
  FolderPlus,
  Loader2,
  Plus,
  RefreshCw,
  Sparkles,
} from 'lucide-react';
import {
  createFolder,
  getPapers,
  importPapers,
  replyDeepStartSession,
  rerunDeepStartSearch,
  updateDeepStartSelections,
} from '../../lib/backend';
import { useAppStore } from '../../stores/appStore';
import type { DeepStartDirection, SearchPaper } from '../../types';

type BusyAction = 'replying' | 'rerunning' | 'selecting' | 'importing' | null;

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

export function SessionDetailPanel() {
  const navigate = useNavigate();
  const {
    activeDeepStartSession,
    activeFolderId,
    folders,
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
  const [replyInput, setReplyInput] = useState('');
  const [rerunQuery, setRerunQuery] = useState(activeDeepStartSession?.summary.currentQuery ?? '');

  useEffect(() => {
    if (activeDeepStartSession) {
      setRerunQuery(activeDeepStartSession.summary.currentQuery);
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

  if (!activeDeepStartSession) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="text-center">
          <p className="text-stone-600 dark:text-stone-400">未找到会话</p>
          <button
            onClick={() => navigate('/history')}
            className="mt-4 rounded-lg bg-blue-600 px-4 py-2 text-white hover:bg-blue-700"
          >
            返回历史
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="flex h-full min-h-0 flex-col bg-[#f9f6f0] dark:bg-[#141414]">
      <div className="border-b border-stone-200 p-4 dark:border-stone-800">
        <div className="flex items-center gap-4 mb-4">
          <button
            onClick={() => navigate('/history')}
            className="flex items-center gap-2 text-stone-600 hover:text-stone-900 dark:text-stone-400 dark:hover:text-stone-100"
          >
            <ArrowLeft className="h-4 w-4" />
            返回历史
          </button>
          <div className="flex-1">
            <h2 className="text-lg font-semibold">{activeDeepStartSession.summary.title}</h2>
            <p className="text-sm text-stone-500 dark:text-stone-400">
              当前检索: {activeDeepStartSession.summary.currentQuery}
            </p>
          </div>
        </div>

        <div className="flex flex-wrap gap-3">
          <select
            value={activeDeepStartSession.summary.targetFolderId || activeFolderId || folders[0]?.id || ''}
            onChange={(event) => void handleTargetFolderChange(event.target.value)}
            className="rounded-lg border border-stone-200 bg-white px-3 py-2 text-sm dark:border-stone-700 dark:bg-stone-900"
          >
            {folders.map((folder) => (
              <option key={folder.id} value={folder.id}>
                入库到：{folder.name}
              </option>
            ))}
          </select>

          <input
            type="text"
            value={rerunQuery}
            onChange={(event) => setRerunQuery(event.target.value)}
            onKeyDown={(event) => event.key === 'Enter' && void handleRerun()}
            placeholder="修改query重新检索"
            className="flex-1 rounded-lg border border-stone-200 bg-white px-3 py-2 text-sm dark:border-stone-700 dark:bg-stone-900"
          />

          <button
            onClick={() => void handleRerun()}
            disabled={busyAction !== null}
            className="flex items-center gap-2 rounded-lg bg-blue-600 px-4 py-2 text-sm text-white hover:bg-blue-700 disabled:opacity-60"
          >
            {busyAction === 'rerunning' ? (
              <Loader2 className="h-4 w-4 animate-spin" />
            ) : (
              <RefreshCw className="h-4 w-4" />
            )}
            重搜
          </button>

          <button
            onClick={() => void handleCreateFolder()}
            disabled={isCreatingFolder}
            className="flex items-center gap-2 rounded-lg border border-stone-200 px-4 py-2 text-sm hover:bg-stone-50 dark:border-stone-700 dark:hover:bg-stone-800"
          >
            <FolderPlus className="h-4 w-4" />
            新建文件夹
          </button>

          <button
            onClick={() => void handleImportSelected()}
            disabled={selectedPaperIds.size === 0 || busyAction !== null}
            className="flex items-center gap-2 rounded-lg bg-emerald-600 px-4 py-2 text-sm text-white hover:bg-emerald-700 disabled:opacity-60"
          >
            <Plus className="h-4 w-4" />
            导入选中 ({selectedPaperIds.size})
          </button>
        </div>
      </div>

      <div className="flex-1 min-h-0 overflow-y-auto p-6">
        <div className="grid gap-6">
          {/* AI Chat */}
          <div className="rounded-2xl border border-stone-200 bg-white p-5 dark:border-stone-800 dark:bg-stone-900">
            <div className="flex items-center gap-2 mb-4">
              <Sparkles className="h-5 w-5 text-emerald-600" />
              <span className="font-medium">AI 分析</span>
            </div>

            <div className="space-y-3 mb-4">
              {activeDeepStartSession.messages.map((message) => (
                <div
                  key={message.id}
                  className={`rounded-2xl px-4 py-3 text-sm ${
                    message.role === 'assistant'
                      ? 'bg-stone-100 text-stone-700 dark:bg-stone-800 dark:text-stone-200'
                      : 'ml-auto bg-blue-600 text-white max-w-[80%]'
                  }`}
                >
                  {message.content}
                </div>
              ))}
            </div>

            <div className="flex gap-3">
              <input
                type="text"
                value={replyInput}
                onChange={(event) => setReplyInput(event.target.value)}
                onKeyDown={(event) => event.key === 'Enter' && void handleReply(replyInput)}
                placeholder="告诉AI你的筛选偏好..."
                className="flex-1 rounded-lg border border-stone-200 bg-white px-3 py-2 text-sm dark:border-stone-700 dark:bg-stone-900"
              />
              <button
                onClick={() => void handleReply(replyInput)}
                disabled={busyAction !== null || !replyInput.trim()}
                className="flex items-center gap-2 rounded-lg bg-blue-600 px-4 py-2 text-sm text-white hover:bg-blue-700 disabled:opacity-60"
              >
                {busyAction === 'replying' ? (
                  <Loader2 className="h-4 w-4 animate-spin" />
                ) : (
                  <Bot className="h-4 w-4" />
                )}
                发送
              </button>
            </div>
          </div>

          {/* Suggested Queries */}
          {(currentAnalysis?.suggestedQueries ?? []).length > 0 && (
            <div className="rounded-2xl border border-stone-200 bg-white p-5 dark:border-stone-800 dark:bg-stone-900">
              <p className="text-sm font-medium mb-3">建议扩搜Query</p>
              <div className="flex flex-wrap gap-2">
                {(currentAnalysis?.suggestedQueries ?? []).map((query) => (
                  <button
                    key={query}
                    onClick={() => void handleRerun(query)}
                    disabled={busyAction !== null}
                    className="rounded-full bg-stone-900 px-3 py-1.5 text-sm text-white hover:bg-stone-700 disabled:opacity-60 dark:bg-stone-100 dark:text-stone-900 dark:hover:bg-white"
                  >
                    {query}
                  </button>
                ))}
              </div>
            </div>
          )}

          {/* Papers by Direction */}
          {currentResults.length > 0 ? (
            groupedDirections.map((direction) => {
              const directionSelectedCount = direction.paperIds.filter((paperId) =>
                selectedPaperIds.has(paperId)
              ).length;
              return (
                <div
                  key={direction.id}
                  className="rounded-2xl border border-stone-200 bg-white p-5 dark:border-stone-800 dark:bg-stone-900"
                >
                  <div className="flex items-start justify-between mb-4">
                    <div>
                      <h3 className="text-lg font-semibold">{direction.name}</h3>
                      <p className="text-sm text-stone-600 dark:text-stone-400 mt-1">
                        {direction.summary}
                      </p>
                      <p className="text-xs text-stone-500 dark:text-stone-500 mt-1">
                        {direction.why}
                      </p>
                    </div>
                    <div className="flex gap-2">
                      <button
                        onClick={() => void toggleDirection(direction, true)}
                        disabled={busyAction !== null}
                        className="rounded-lg border border-stone-200 px-3 py-1.5 text-sm hover:bg-stone-50 dark:border-stone-700 dark:hover:bg-stone-800"
                      >
                        全选 ({directionSelectedCount}/{direction.paperIds.length})
                      </button>
                      <button
                        onClick={() => void toggleDirection(direction, false)}
                        disabled={busyAction !== null}
                        className="rounded-lg border border-stone-200 px-3 py-1.5 text-sm hover:bg-stone-50 dark:border-stone-700 dark:hover:bg-stone-800"
                      >
                        取消
                      </button>
                    </div>
                  </div>

                  <div className="grid gap-4 grid-cols-1">
                    {direction.paperIds
                      .map((paperId) => paperById.get(paperId))
                      .filter((paper): paper is SearchPaper => Boolean(paper))
                      .map((paper) => renderPaperCard(paper))}
                  </div>
                </div>
              );
            })
          ) : (
            <div className="rounded-2xl border border-dashed border-stone-300 bg-white/60 p-12 text-center dark:border-stone-700 dark:bg-stone-900/50">
              <p className="text-stone-600 dark:text-stone-400">这一轮还没有拿到候选论文</p>
              <p className="text-sm text-stone-500 dark:text-stone-500 mt-2">
                点击上面的建议query，或告诉AI你想看什么类型的论文
              </p>
            </div>
          )}

          {/* Summary */}
          <div className="rounded-2xl bg-stone-50 p-4 dark:bg-stone-950/70">
            <div className="flex items-center gap-2 text-sm font-medium">
              <CheckCheck className="h-4 w-4 text-emerald-600" />
              当前已选 {selectedPaperIds.size} 篇论文
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
