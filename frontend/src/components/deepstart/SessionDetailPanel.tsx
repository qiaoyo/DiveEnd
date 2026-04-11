import { useEffect, useMemo, useState, type ReactNode } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  ArrowLeft,
  CheckCheck,
  ChevronDown,
  ChevronUp,
  ExternalLink,
  FolderPlus,
  Loader2,
  Plus,
  RefreshCw,
  Sparkles,
  X,
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
import type { DeepStartDirection, DeepStartPaperNote, SearchPaper } from '../../types';

type BusyAction = 'replying' | 'rerunning' | 'selecting' | 'importing' | null;

function tierStyle(tier: string) {
  switch (tier) {
    case 'core':
      return 'bg-indigo-100 text-indigo-700 dark:bg-indigo-500/20 dark:text-indigo-200';
    case 'important':
      return 'bg-violet-100 text-violet-700 dark:bg-violet-500/20 dark:text-violet-200';
    default:
      return 'bg-slate-100 text-slate-600 dark:bg-slate-700/60 dark:text-slate-200';
  }
}

function sourceLabel(paper: SearchPaper): string {
  return paper.sourceLabel || paper.journal || '未知来源';
}

function institutionLabel(paper: SearchPaper): string {
  if (paper.institutions.length === 0) {
    return '机构未提供';
  }
  return paper.institutions.slice(0, 3).join(' · ');
}

function paperKeywords(paper: SearchPaper): string[] {
  const values = [...paper.keywords, ...paper.tags].map((item) => item.trim()).filter(Boolean);
  return [...new Set(values)].slice(0, 6);
}

function splitAuthors(authors: string): string[] {
  return authors
    .split(/[,;，；]\s*/)
    .map((author) => author.trim())
    .filter(Boolean);
}

function escapeRegExp(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

function highlightTokens(query: string, note: DeepStartPaperNote | undefined, paper: SearchPaper | null): string[] {
  const tokens = new Set<string>();
  for (const item of query.split(/\s+/)) {
    const normalized = item.trim();
    if (normalized.length >= 2) {
      tokens.add(normalized);
    }
  }
  for (const item of (note?.reason ?? '').split(/[\s,，;；。]+/)) {
    const normalized = item.trim();
    if (normalized.length >= 2) {
      tokens.add(normalized);
    }
  }
  for (const keyword of paper ? paperKeywords(paper) : []) {
    if (keyword.length >= 2) {
      tokens.add(keyword);
    }
  }
  return [...tokens].slice(0, 16);
}

function renderHighlightedText(text: string, tokens: string[]): ReactNode {
  const content = text.trim();
  if (!content) {
    return '暂无摘要。';
  }
  if (tokens.length === 0) {
    return content;
  }

  const escaped = tokens.map((token) => escapeRegExp(token)).join('|');
  if (!escaped) {
    return content;
  }
  const pattern = new RegExp(`(${escaped})`, 'ig');
  const chunks = content.split(pattern);
  if (chunks.length === 1) {
    return content;
  }

  return chunks.map((chunk, index) => {
    if (index % 2 === 1) {
      return (
        <mark
          key={`${chunk}-${index}`}
          className="rounded bg-amber-200/80 px-1 text-slate-900 dark:bg-amber-400/30 dark:text-amber-100"
        >
          {chunk}
        </mark>
      );
    }
    return <span key={`${chunk}-${index}`}>{chunk}</span>;
  });
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
  const [chatCollapsed, setChatCollapsed] = useState(false);
  const [activePaperId, setActivePaperId] = useState<string | null>(null);

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
  const activePaper = useMemo(
    () => (activePaperId ? paperById.get(activePaperId) ?? null : null),
    [activePaperId, paperById]
  );
  const activePaperNote = useMemo(
    () => (activePaper ? noteByPaperId.get(activePaper.id) : undefined),
    [activePaper, noteByPaperId]
  );
  const summaryHighlightTokens = useMemo(
    () => highlightTokens(activeDeepStartSession?.summary.currentQuery ?? '', activePaperNote, activePaper),
    [activeDeepStartSession?.summary.currentQuery, activePaper, activePaperNote]
  );

  useEffect(() => {
    if (!activePaperId) {
      return;
    }
    if (!paperById.has(activePaperId)) {
      setActivePaperId(null);
    }
  }, [activePaperId, paperById]);

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
      setActivePaperId(null);
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
    const keywords = paperKeywords(paper);

    return (
      <article
        key={paper.id}
        role="button"
        tabIndex={0}
        onClick={() => setActivePaperId(paper.id)}
        onKeyDown={(event) => {
          if (event.key === 'Enter' || event.key === ' ') {
            event.preventDefault();
            setActivePaperId(paper.id);
          }
        }}
        className={`cursor-pointer rounded-2xl border p-4 text-left transition focus:outline-none focus:ring-2 focus:ring-indigo-400 ${
          isSelected
            ? 'border-indigo-400 bg-indigo-50/90 dark:border-indigo-500/60 dark:bg-indigo-500/15'
            : isRecommended
              ? 'border-violet-300 bg-violet-50/80 hover:border-violet-400 dark:border-violet-500/60 dark:bg-violet-500/15'
              : 'border-slate-200 bg-white/85 hover:-translate-y-0.5 hover:border-indigo-300 hover:shadow-lg dark:border-slate-700 dark:bg-slate-900/80'
        }`}
      >
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <h3 className="line-clamp-2 text-sm font-semibold leading-6">{paper.title}</h3>
              {note && (
                <span className={`rounded-full px-2 py-0.5 text-[11px] font-medium ${tierStyle(note.tier)}`}>
                  {note.tier}
                </span>
              )}
            </div>
            <p className="mt-1 text-xs text-slate-500 dark:text-slate-300">
              {sourceLabel(paper)} · {paper.year || '年份未知'}
            </p>
            <p className="mt-1 text-xs text-slate-500 dark:text-slate-300">{institutionLabel(paper)}</p>
          </div>
          <button
            type="button"
            disabled={busyAction !== null}
            onClick={(event) => {
              event.stopPropagation();
              void toggleSelect(paper.id);
            }}
            className={`mt-1 inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-full border text-xs ${
              isSelected
                ? 'border-indigo-500 bg-indigo-500 text-white'
                : 'border-slate-300 text-slate-500 dark:border-slate-500 dark:text-slate-200'
            }`}
          >
            {isSelected ? '✓' : ''}
          </button>
        </div>
        {keywords.length > 0 && (
          <div className="mt-3 flex flex-wrap gap-1.5">
            {keywords.slice(0, 4).map((keyword) => (
              <span
                key={`${paper.id}-${keyword}`}
                className="rounded-full bg-slate-100 px-2 py-0.5 text-[11px] text-slate-600 dark:bg-slate-700/70 dark:text-slate-200"
              >
                {keyword}
              </span>
            ))}
          </div>
        )}
        {paper.enrichmentNote && (
          <p className="mt-2 text-xs text-amber-700 dark:text-amber-300">{paper.enrichmentNote}</p>
        )}
        <p className="mt-3 line-clamp-4 text-sm leading-6 text-slate-600 dark:text-slate-200">
          {paper.abstract || '暂无摘要。'}
        </p>
      </article>
    );
  };

  if (!activeDeepStartSession) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="text-center">
          <p className="text-slate-600 dark:text-slate-300">未找到会话</p>
          <button
            onClick={() => navigate('/history')}
            className="mt-4 rounded-xl bg-indigo-600 px-4 py-2 text-white hover:bg-indigo-500"
          >
            返回历史
          </button>
        </div>
      </div>
    );
  }

  const activeTargetFolderId =
    activeDeepStartSession.summary.targetFolderId || activeFolderId || folders[0]?.id || '';
  const searchStats = currentAnalysis?.searchStats;

  return (
    <div className="flex h-full min-h-0 flex-col overflow-hidden bg-slate-50 text-slate-900 dark:bg-slate-950 dark:text-slate-100">
      <div className="border-b border-slate-200/80 bg-white/70 px-6 py-4 backdrop-blur-md dark:border-slate-700/50 dark:bg-slate-900/55">
        <div className="mx-auto flex max-w-7xl flex-wrap items-center gap-3">
          <button
            onClick={() => navigate('/history')}
            className="inline-flex items-center gap-1.5 rounded-xl border border-slate-200 bg-white/70 px-3 py-1.5 text-sm text-slate-600 transition hover:border-indigo-300 hover:text-indigo-700 dark:border-slate-700 dark:bg-slate-900/70 dark:text-slate-300 dark:hover:border-indigo-500/60 dark:hover:text-indigo-300"
          >
            <ArrowLeft className="h-4 w-4" />
            返回历史
          </button>

          <div className="min-w-[220px] flex-1">
            <p className="text-xs uppercase tracking-[0.16em] text-slate-500 dark:text-slate-400">Current Session</p>
            <h2 className="truncate text-base font-semibold">{activeDeepStartSession.summary.title}</h2>
          </div>

          <select
            value={activeTargetFolderId}
            onChange={(event) => void handleTargetFolderChange(event.target.value)}
            className="rounded-xl border border-slate-200 bg-white/80 px-3 py-2 text-sm outline-none transition focus:border-indigo-500 dark:border-slate-700 dark:bg-slate-900/80"
          >
            {folders.map((folder) => (
              <option key={folder.id} value={folder.id}>
                入库到：{folder.name}
              </option>
            ))}
          </select>

          <button
            onClick={() => void handleCreateFolder()}
            disabled={isCreatingFolder}
            className="inline-flex items-center gap-1.5 rounded-xl border border-slate-200 bg-white/80 px-3 py-2 text-sm text-slate-600 transition hover:border-indigo-300 hover:text-indigo-700 disabled:opacity-60 dark:border-slate-700 dark:bg-slate-900/80 dark:text-slate-300 dark:hover:border-indigo-500/60 dark:hover:text-indigo-300"
          >
            <FolderPlus className="h-4 w-4" />
            新建文件夹
          </button>
        </div>

        <div className="mx-auto mt-3 max-w-7xl rounded-2xl border border-white/80 bg-white/85 p-3 shadow-sm dark:border-slate-500/30 dark:bg-slate-900/80">
          <div className="flex flex-wrap items-center gap-2">
            <Sparkles className="h-4 w-4 text-violet-500" />
            <input
              type="text"
              value={rerunQuery}
              onChange={(event) => setRerunQuery(event.target.value)}
              onKeyDown={(event) => event.key === 'Enter' && void handleRerun()}
              placeholder="修改 query 重新检索"
              className="min-w-[240px] flex-1 border-none bg-transparent px-1 py-1.5 text-sm outline-none placeholder:text-slate-400"
            />
            <button
              onClick={() => void handleRerun()}
              disabled={busyAction !== null}
              className="inline-flex items-center gap-1.5 rounded-xl bg-indigo-600 px-3 py-1.5 text-sm text-white transition hover:bg-indigo-500 disabled:opacity-60"
            >
              {busyAction === 'rerunning' ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
              重搜
            </button>
          </div>
        </div>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto px-6 pb-28 pt-5">
        <div className="mx-auto max-w-7xl">
          <section className="de-glass sticky top-4 z-20 rounded-2xl border border-indigo-200/70 p-4 dark:border-indigo-500/30">
            <div className="flex items-center justify-between gap-3">
              <h3 className="text-sm font-semibold uppercase tracking-[0.18em] text-slate-500 dark:text-slate-300">AI Chat</h3>
              <button
                type="button"
                onClick={() => setChatCollapsed((value) => !value)}
                className="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white/80 px-2.5 py-1 text-xs text-slate-600 dark:border-slate-700 dark:bg-slate-900/80 dark:text-slate-200"
              >
                {chatCollapsed ? <ChevronDown className="h-3.5 w-3.5" /> : <ChevronUp className="h-3.5 w-3.5" />}
                {chatCollapsed ? '展开' : '收起'}
              </button>
            </div>

            {!chatCollapsed && (
              <div className="mt-3 space-y-3">
                {(currentAnalysis?.suggestedQueries ?? []).length > 0 && (
                  <div className="flex flex-wrap gap-2">
                    {(currentAnalysis?.suggestedQueries ?? []).map((query) => (
                      <button
                        key={query}
                        onClick={() => void handleRerun(query)}
                        disabled={busyAction !== null}
                        className="rounded-full border border-slate-200 bg-white/80 px-3 py-1.5 text-xs text-slate-600 transition hover:border-indigo-300 hover:text-indigo-700 disabled:opacity-60 dark:border-slate-700 dark:bg-slate-900/80 dark:text-slate-200 dark:hover:border-indigo-500/60 dark:hover:text-indigo-300"
                      >
                        {query}
                      </button>
                    ))}
                  </div>
                )}

                <div className="max-h-48 space-y-2 overflow-y-auto">
                  {activeDeepStartSession.messages.slice(-8).map((message) => (
                    <div
                      key={message.id}
                      className={`rounded-xl px-3 py-2 text-xs leading-6 ${
                        message.role === 'assistant'
                          ? 'bg-white/80 text-slate-600 dark:bg-slate-900/80 dark:text-slate-200'
                          : 'ml-4 bg-indigo-600 text-white'
                      }`}
                    >
                      {message.content}
                    </div>
                  ))}
                </div>

                <div className="flex gap-2">
                  <input
                    type="text"
                    value={replyInput}
                    onChange={(event) => setReplyInput(event.target.value)}
                    onKeyDown={(event) => event.key === 'Enter' && void handleReply(replyInput)}
                    placeholder="补充你的筛选偏好"
                    className="min-w-0 flex-1 rounded-xl border border-slate-200 bg-white/80 px-3 py-2 text-xs outline-none transition focus:border-indigo-500 dark:border-slate-700 dark:bg-slate-900/80"
                  />
                  <button
                    onClick={() => void handleReply(replyInput)}
                    disabled={busyAction !== null || !replyInput.trim()}
                    className="rounded-xl bg-violet-600 px-3 py-2 text-xs text-white transition hover:bg-violet-500 disabled:opacity-60"
                  >
                    {busyAction === 'replying' ? '发送中' : '发送'}
                  </button>
                </div>
              </div>
            )}
          </section>

          <div className="mt-5 grid gap-6 xl:grid-cols-[320px_1px_1fr]">
            <aside className="space-y-4">
              <section className="de-glass rounded-2xl p-4">
                <h3 className="text-xs uppercase tracking-[0.18em] text-slate-500 dark:text-slate-300">AI Overview</h3>
                <p className="mt-2 text-sm leading-7 text-slate-600 dark:text-slate-200">
                  {currentAnalysis?.overview || 'AI 正在构建这轮检索的分组策略。'}
                </p>
                {searchStats && (
                  <div className="mt-3 grid gap-2 text-xs text-slate-600 dark:text-slate-200">
                    <div className="rounded-xl border border-slate-200 bg-white/80 px-3 py-2 dark:border-slate-700 dark:bg-slate-900/80">
                      检索统计：原始 {searchStats.rawCount} · 去重后 {searchStats.dedupCount} · 入池 {searchStats.finalCount}
                    </div>
                  </div>
                )}
              </section>

              <section className="de-glass rounded-2xl p-4">
                <h3 className="text-xs uppercase tracking-[0.18em] text-slate-500 dark:text-slate-300">Category Tree</h3>
                <div className="mt-3 space-y-2">
                  {groupedDirections.map((direction, index) => {
                    const selectedCount = direction.paperIds.filter((paperId) => selectedPaperIds.has(paperId)).length;
                    return (
                      <div key={direction.id} className="rounded-xl border border-slate-200 bg-white/80 p-3 dark:border-slate-700 dark:bg-slate-900/80">
                        <div className="flex items-center justify-between">
                          <div className="text-sm font-medium">{direction.name}</div>
                          <span className="rounded-full bg-violet-100 px-2 py-0.5 text-[11px] text-violet-700 dark:bg-violet-500/20 dark:text-violet-200">
                            {direction.paperIds.length}
                          </span>
                        </div>
                        <p className="mt-1 text-xs leading-5 text-slate-500 dark:text-slate-300">{direction.summary}</p>
                        <div className="mt-2 flex gap-2">
                          <button
                            onClick={() => void toggleDirection(direction, true)}
                            disabled={busyAction !== null}
                            className="rounded-lg border border-slate-200 px-2.5 py-1 text-[11px] text-slate-600 transition hover:bg-slate-100 disabled:opacity-60 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800"
                          >
                            全选 {selectedCount}/{direction.paperIds.length}
                          </button>
                          <button
                            onClick={() => void toggleDirection(direction, false)}
                            disabled={busyAction !== null}
                            className="rounded-lg border border-slate-200 px-2.5 py-1 text-[11px] text-slate-600 transition hover:bg-slate-100 disabled:opacity-60 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800"
                          >
                            清空
                          </button>
                        </div>
                        {index < groupedDirections.length - 1 && (
                          <div className="mx-auto mt-2 h-3 w-px border-l border-dashed border-indigo-300 dark:border-indigo-500/40" />
                        )}
                      </div>
                    );
                  })}
                </div>
              </section>
            </aside>

            <div className="hidden border-l border-dashed border-slate-300 xl:block dark:border-slate-600/60" />

            <section className="space-y-4">
              {currentResults.length > 0 ? (
                groupedDirections.map((direction) => (
                  <div key={direction.id} className="de-glass rounded-2xl p-4">
                    <div className="mb-3 flex items-start justify-between gap-3">
                      <div>
                        <h3 className="text-base font-semibold">{direction.name}</h3>
                        <p className="mt-1 text-sm text-slate-500 dark:text-slate-300">{direction.why}</p>
                      </div>
                      <span className="rounded-full bg-indigo-100 px-3 py-1 text-xs font-medium text-indigo-700 dark:bg-indigo-500/25 dark:text-indigo-200">
                        {direction.paperIds.length} papers
                      </span>
                    </div>
                    <div className="grid gap-3 sm:grid-cols-2">
                      {direction.paperIds
                        .map((paperId) => paperById.get(paperId))
                        .filter((paper): paper is SearchPaper => Boolean(paper))
                        .map((paper) => renderPaperCard(paper))}
                    </div>
                  </div>
                ))
              ) : (
                <div className="de-glass rounded-2xl p-10 text-center">
                  <p className="text-slate-600 dark:text-slate-200">这一轮还没有拿到候选论文</p>
                  <p className="mt-2 text-sm text-slate-500 dark:text-slate-300">
                    可以点击顶部建议 query 重跑，或告诉 AI 你更偏好的论文类型。
                  </p>
                </div>
              )}
            </section>
          </div>
        </div>
      </div>

      {activePaper && (
        <>
          <button
            type="button"
            onClick={() => setActivePaperId(null)}
            className="fixed inset-0 z-30 bg-slate-950/35 backdrop-blur-[1px]"
            aria-label="关闭详情抽屉"
          />
          <aside className="fixed right-0 top-0 z-40 flex h-full w-full max-w-2xl flex-col border-l border-slate-200 bg-white/98 p-5 shadow-2xl dark:border-slate-700 dark:bg-slate-950/98">
            <div className="flex items-start justify-between gap-4">
              <div>
                <p className="text-xs uppercase tracking-[0.16em] text-slate-500 dark:text-slate-400">Paper Detail</p>
                <h3 className="mt-2 text-lg font-semibold leading-7">{activePaper.title}</h3>
                <p className="mt-1 text-sm text-slate-500 dark:text-slate-300">
                  {sourceLabel(activePaper)} · {activePaper.year || '年份未知'}
                </p>
              </div>
              <button
                type="button"
                onClick={() => setActivePaperId(null)}
                className="rounded-lg border border-slate-200 p-2 text-slate-500 hover:text-slate-700 dark:border-slate-700 dark:text-slate-300 dark:hover:text-white"
              >
                <X className="h-4 w-4" />
              </button>
            </div>

            <div className="mt-4 flex flex-wrap gap-2">
              <button
                type="button"
                onClick={() => void toggleSelect(activePaper.id)}
                disabled={busyAction !== null}
                className={`rounded-xl px-3 py-1.5 text-xs font-medium ${
                  selectedPaperIds.has(activePaper.id)
                    ? 'bg-indigo-600 text-white'
                    : 'border border-slate-200 bg-white text-slate-600 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-200'
                }`}
              >
                {selectedPaperIds.has(activePaper.id) ? '已选中' : '加入选中'}
              </button>
              {activePaper.url && (
                <a
                  href={activePaper.url}
                  target="_blank"
                  rel="noreferrer"
                  className="inline-flex items-center gap-1 rounded-xl border border-slate-200 bg-white px-3 py-1.5 text-xs text-slate-600 hover:text-indigo-700 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-200"
                >
                  <ExternalLink className="h-3.5 w-3.5" />
                  打开链接
                </a>
              )}
            </div>

            <div className="mt-4 grid gap-3 overflow-y-auto pr-1">
              <section className="rounded-xl border border-slate-200 bg-slate-50/70 p-3 dark:border-slate-700 dark:bg-slate-900/70">
                <h4 className="text-xs uppercase tracking-[0.16em] text-slate-500 dark:text-slate-400">机构 / 学校</h4>
                <p className="mt-2 text-sm leading-6 text-slate-700 dark:text-slate-200">{institutionLabel(activePaper)}</p>
                {activePaper.enrichmentNote && (
                  <p className="mt-2 text-xs leading-5 text-amber-700 dark:text-amber-300">{activePaper.enrichmentNote}</p>
                )}
              </section>

              <section className="rounded-xl border border-slate-200 bg-slate-50/70 p-3 dark:border-slate-700 dark:bg-slate-900/70">
                <h4 className="text-xs uppercase tracking-[0.16em] text-slate-500 dark:text-slate-400">关键词</h4>
                <div className="mt-2 flex flex-wrap gap-1.5">
                  {paperKeywords(activePaper).length > 0 ? (
                    paperKeywords(activePaper).map((keyword) => (
                      <span
                        key={`drawer-${activePaper.id}-${keyword}`}
                        className="rounded-full bg-white px-2 py-0.5 text-xs text-slate-600 dark:bg-slate-800 dark:text-slate-200"
                      >
                        {keyword}
                      </span>
                    ))
                  ) : (
                    <span className="text-sm text-slate-500 dark:text-slate-300">未提取到关键词</span>
                  )}
                </div>
              </section>

              <section className="rounded-xl border border-slate-200 bg-slate-50/70 p-3 dark:border-slate-700 dark:bg-slate-900/70">
                <h4 className="text-xs uppercase tracking-[0.16em] text-slate-500 dark:text-slate-400">作者列表</h4>
                <div className="mt-2 flex flex-wrap gap-1.5">
                  {splitAuthors(activePaper.authors).length > 0 ? (
                    splitAuthors(activePaper.authors).map((author) => (
                      <span
                        key={`author-${activePaper.id}-${author}`}
                        className="rounded-full bg-white px-2 py-0.5 text-xs text-slate-700 dark:bg-slate-800 dark:text-slate-200"
                      >
                        {author}
                      </span>
                    ))
                  ) : (
                    <span className="text-sm text-slate-500 dark:text-slate-300">作者信息缺失</span>
                  )}
                </div>
              </section>

              <section className="rounded-xl border border-slate-200 bg-slate-50/70 p-3 dark:border-slate-700 dark:bg-slate-900/70">
                <div className="flex items-center justify-between gap-2">
                  <h4 className="text-xs uppercase tracking-[0.16em] text-slate-500 dark:text-slate-400">完整摘要（规则高亮）</h4>
                  <button
                    type="button"
                    onClick={() => void navigator.clipboard.writeText(activePaper.abstract || '')}
                    className="rounded-lg border border-slate-200 bg-white px-2 py-1 text-xs text-slate-600 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-200"
                  >
                    复制摘要
                  </button>
                </div>
                <p className="mt-2 whitespace-pre-wrap text-sm leading-7 text-slate-700 dark:text-slate-200">
                  {renderHighlightedText(activePaper.abstract || '', summaryHighlightTokens)}
                </p>
              </section>
            </div>
          </aside>
        </>
      )}

      <div className="pointer-events-none fixed bottom-6 left-1/2 z-30 w-full max-w-7xl -translate-x-1/2 px-6">
        <div className="pointer-events-auto mx-auto flex max-w-xl items-center justify-between rounded-2xl border border-slate-200 bg-white/90 px-4 py-3 shadow-xl backdrop-blur-md dark:border-slate-700 dark:bg-slate-900/90">
          <div className="flex items-center gap-2 text-sm text-slate-600 dark:text-slate-200">
            <CheckCheck className="h-4 w-4 text-indigo-500" />
            当前已选 <span className="font-semibold text-indigo-600 dark:text-indigo-300">{selectedPaperIds.size}</span> 篇论文
          </div>
          <button
            onClick={() => void handleImportSelected()}
            disabled={selectedPaperIds.size === 0 || busyAction !== null}
            className="inline-flex items-center gap-1.5 rounded-xl bg-indigo-600 px-4 py-2 text-sm font-medium text-white transition hover:bg-indigo-500 disabled:cursor-not-allowed disabled:opacity-60"
          >
            {busyAction === 'importing' ? <Loader2 className="h-4 w-4 animate-spin" /> : <Plus className="h-4 w-4" />}
            导入选中
          </button>
        </div>
      </div>
    </div>
  );
}
