import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
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
  Search,
  X,
} from 'lucide-react';
import {
  cancelDeepStartTask,
  createFolderNode,
  deleteFolderNode,
  getDeepStartSession,
  getFolderStorageTreeOverview,
  getFolderTree,
  getPapers,
  importPapersWithAssets,
  onDeepStartProgress,
  replyDeepStartSession,
  rerunDeepStartSearch,
  supplementDeepStartSearch,
  undoDeepStartNarrow,
  updateDeepStartSelections,
} from '../../lib/backend';
import { errorToUserMessage, isCancellationError, sanitizeUserVisibleError } from '../../lib/errors';
import { useAppStore } from '../../stores/appStore';
import type {
  DeepStartDirection,
  DeepStartPaperNote,
  DeepStartProgressEvent,
  Folder,
  FolderNode,
  FolderStorageTreeOverview,
  SearchPaper,
} from '../../types';

type BusyAction = 'replying' | 'rerunning' | 'supplementing' | 'selecting' | 'importing' | 'undoing' | null;

type ChatRuntimeState = {
  status: 'idle' | 'running' | 'cancelling' | 'cancelled' | 'failed' | 'completed';
  phase: DeepStartProgressEvent['phase'] | 'idle';
  percent: number;
  eta: number;
  message: string;
};

function runtimeMessage(message: string | undefined, fallback: string): string {
  return sanitizeUserVisibleError((message || fallback).trim());
}

function tierStyle(tier: string) {
  switch (tier) {
    case 'core':
      return 'bg-[var(--de-accent-soft)] text-[var(--de-accent)]';
    case 'important':
      return 'border border-[var(--de-rule-strong)] text-[var(--de-ink)]';
    default:
      return 'bg-[var(--de-surface-muted)] text-[var(--de-ink-muted)]';
  }
}

function sourceLabel(paper: SearchPaper): string {
  return paper.sourceLabel || paper.source || '未知来源';
}

function publicationLabel(paper: SearchPaper): string {
  const venue = (paper.publicationVenue || paper.journal || '').trim() || '发表未提供';
  const year = paper.publicationYear || paper.year;
  const citation = paper.citationCount > 0 ? `引用 ${paper.citationCount}` : '引用未提供';
  return `${venue} · ${year || '年份未知'} · ${citation}`;
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

function splitAbstractSentences(text: string): string[] {
  const content = text.trim();
  if (!content) {
    return [];
  }

  const chunks = content
    .split(/(?<=[。！？.!?])\s+|\n+/)
    .map((chunk) => chunk.trim())
    .filter(Boolean);
  if (chunks.length > 0) {
    return chunks;
  }
  return [content];
}

function scoreSentence(sentence: string, tokens: string[]): number {
  if (!sentence || tokens.length === 0) {
    return 0;
  }

  const loweredSentence = sentence.toLowerCase();
  let score = 0;
  for (const token of tokens) {
    const normalized = token.toLowerCase().trim();
    if (!normalized) {
      continue;
    }
    if (loweredSentence.includes(normalized)) {
      score += normalized.length >= 6 ? 3 : 2;
    }
  }
  return score;
}

function extractKeySentences(text: string, tokens: string[]): string[] {
  const sentences = splitAbstractSentences(text);
  if (sentences.length === 0) {
    return [];
  }

  const ranked = sentences.map((sentence, index) => ({
    sentence,
    index,
    score: scoreSentence(sentence, tokens),
  }));

  const topCandidates = ranked
    .filter((item) => item.score > 0)
    .sort((a, b) => {
      if (b.score !== a.score) {
        return b.score - a.score;
      }
      return a.index - b.index;
    })
    .slice(0, 3)
    .sort((a, b) => a.index - b.index)
    .map((item) => item.sentence);

  if (topCandidates.length > 0) {
    return topCandidates;
  }
  return sentences.slice(0, Math.min(2, sentences.length));
}

function escapeRegExp(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

// 保留旧版词级高亮逻辑，默认不启用，必要时可快速回滚。
function renderLegacyHighlightedText(text: string, tokens: string[]): ReactNode {
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
      return <mark key={`${chunk}-${index}`}>{chunk}</mark>;
    }
    return <span key={`${chunk}-${index}`}>{chunk}</span>;
  });
}

function renderInlineHighlightedAbstract(text: string, tokens: string[]): ReactNode {
  const content = text.trim();
  if (!content) {
    return '暂无摘要。';
  }

  const keySentences = new Set(extractKeySentences(content, tokens));
  if (keySentences.size === 0) {
    return content;
  }

  const sentences = splitAbstractSentences(content);
  if (sentences.length === 0) {
    return content;
  }

  return (
    <>
      {sentences.map((sentence, index) => {
        const highlighted = keySentences.has(sentence);
        return (
          <span key={`abstract-sentence-${index}`}>
            {highlighted ? (
              <mark className="rounded bg-amber-100/90 px-1 py-0.5 text-slate-900 dark:bg-amber-400/30 dark:text-amber-50">
                <strong>{sentence}</strong>
              </mark>
            ) : (
              sentence
            )}
            {index < sentences.length - 1 ? ' ' : ''}
          </span>
        );
      })}
    </>
  );
}

type FolderOption = {
  id: string;
  label: string;
};

const folderPathSegmentPattern = /^[A-Za-z_\u4E00-\u9FFF ]+$/;

function validateFolderPathInput(path: string): string | null {
  const trimmed = path.trim();
  if (!trimmed) {
    return null;
  }

  if (trimmed.length > 180) {
    return '目录路径过长，请缩短后重试';
  }

  const normalized = trimmed.replace(/\\/g, '/');
  const segments = normalized.split('/');
  for (const segment of segments) {
    const compact = segment.trim().replace(/\s+/g, ' ');
    if (!compact) {
      return '目录路径不能包含空层级，请检查 / 分隔';
    }
    if (!folderPathSegmentPattern.test(compact)) {
      return '目录路径仅支持中文、英文、空格和下划线（_）';
    }
  }
  return null;
}

function flattenFolderTree(nodes: FolderNode[]): FolderOption[] {
  const options: FolderOption[] = [];
  for (const node of nodes) {
    options.push({
      id: node.folder.id,
      label: node.folder.path || node.folder.name,
    });
    options.push(...flattenFolderTree(node.children));
  }
  return options;
}

function flattenFolderNodes(nodes: FolderNode[]): Folder[] {
  const list: Folder[] = [];
  const walk = (nodeList: FolderNode[]) => {
    for (const node of nodeList) {
      list.push(node.folder);
      walk(node.children);
    }
  };
  walk(nodes);
  return list;
}

export function SessionDetailPanel() {
  const navigate = useNavigate();
  const routeSessionId = typeof window === 'undefined'
    ? ''
    : decodeURIComponent(window.location.hash.match(/\/session\/([^/?#]+)/)?.[1] ?? '');
  const {
    activeDeepStartSession,
    activeFolderId,
    folders,
    setActiveFolderId,
    setError,
    setFolders,
    setPapers,
    setSelectedPaper,
    upsertDeepStartSession,
  } = useAppStore();

  const [busyAction, setBusyAction] = useState<BusyAction>(null);
  const [isCreatingFolder, setIsCreatingFolder] = useState(false);
  const [isFolderModalOpen, setIsFolderModalOpen] = useState(false);
  const [newFolderPath, setNewFolderPath] = useState('');
  const [newFolderError, setNewFolderError] = useState('');
  const [replyInput, setReplyInput] = useState('');
  const [supplementPerSourceLimit, setSupplementPerSourceLimit] = useState(20);
  const [rerunQuery, setRerunQuery] = useState(activeDeepStartSession?.summary.currentQuery ?? '');
  const [chatCollapsed, setChatCollapsed] = useState(false);
  const [activePaperId, setActivePaperId] = useState<string | null>(null);
  const [importFeedback, setImportFeedback] = useState('');
  const [copyFeedback, setCopyFeedback] = useState('');
  const [folderTree, setFolderTree] = useState<FolderNode[]>([]);
  const [storageTreeOverview, setStorageTreeOverview] = useState<FolderStorageTreeOverview | null>(null);
  const [loadingStorageOverview, setLoadingStorageOverview] = useState(false);
  const [optimisticUserMessage, setOptimisticUserMessage] = useState('');

  useEffect(() => {
    const requestedSessionId = routeSessionId.trim();
    if (!requestedSessionId || activeDeepStartSession?.summary.id === requestedSessionId) {
      return;
    }

    let cancelled = false;
    getDeepStartSession(requestedSessionId)
      .then((detail) => {
        if (!cancelled) {
          upsertDeepStartSession(detail);
        }
      })
      .catch((error) => {
        if (!cancelled) {
          setError(errorToUserMessage(error, '加载探索会话失败'));
        }
      });

    return () => {
      cancelled = true;
    };
  }, [routeSessionId, activeDeepStartSession?.summary.id, upsertDeepStartSession, setError]);
  const [initialSuggestedQueries, setInitialSuggestedQueries] = useState<string[]>([]);
  const [chatRuntime, setChatRuntime] = useState<ChatRuntimeState>({
    status: 'idle',
    phase: 'idle',
    percent: 0,
    eta: 0,
    message: '',
  });
  const useLegacyAbstractHighlight = false;
  const skipStoragePollingInTests = import.meta.env.MODE === 'test';
  const replyInputRef = useRef<HTMLTextAreaElement | null>(null);
  const lastSessionRefreshAtRef = useRef(0);

  useEffect(() => {
    if (activeDeepStartSession) {
      setRerunQuery(activeDeepStartSession.summary.currentQuery);
      setInitialSuggestedQueries(activeDeepStartSession.currentAnalysis?.suggestedQueries ?? []);
      setChatRuntime({
        status: 'idle',
        phase: 'idle',
        percent: 0,
        eta: 0,
        message: '',
      });
      setOptimisticUserMessage('');
    }
  }, [activeDeepStartSession?.summary.currentQuery, activeDeepStartSession, activeFolderId]);

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
          setError(errorToUserMessage(error, '同步目标文件夹失败'));
        }
      });

    return () => {
      cancelled = true;
    };
  }, [activeDeepStartSession?.summary.targetFolderId, setActiveFolderId, setError, setPapers]);

  useEffect(() => {
    if (skipStoragePollingInTests) {
      return;
    }

    let cancelled = false;

    const refreshOverview = async (markLoading: boolean) => {
      if (markLoading) {
        setLoadingStorageOverview(true);
      }
      try {
        const [overview, tree] = await Promise.all([
          getFolderStorageTreeOverview(),
          getFolderTree(),
        ]);
        if (!cancelled) {
          setStorageTreeOverview(overview);
          setFolderTree(tree);
          setFolders(flattenFolderNodes(tree));
        }
      } catch (error) {
        if (!cancelled) {
          setStorageTreeOverview(null);
          setError(errorToUserMessage(error, '读取本地存储速览失败'));
        }
      } finally {
        if (!cancelled && markLoading) {
          setLoadingStorageOverview(false);
        }
      }
    };

    void refreshOverview(true);
    const timer = window.setInterval(() => {
      void refreshOverview(false);
    }, 3000);

    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, [setError, setFolders, skipStoragePollingInTests]);

  useEffect(() => {
    const sessionID = activeDeepStartSession?.summary.id;
    if (!sessionID) {
      return;
    }

    const refreshSessionIfNeeded = (force = false) => {
      const now = Date.now();
      if (!force && now-lastSessionRefreshAtRef.current < 900) {
        return;
      }
      lastSessionRefreshAtRef.current = now;
      void getDeepStartSession(sessionID)
        .then((detail) => {
          upsertDeepStartSession(detail);
          if (detail.summary.targetFolderId) {
            setActiveFolderId(detail.summary.targetFolderId);
          }
        })
        .catch((error) => {
          setError(errorToUserMessage(error, '刷新会话状态失败'));
        });
    };

    return onDeepStartProgress((progress) => {
      if (progress.sessionId && progress.sessionId !== sessionID) {
        return;
      }

      if (progress.phase === 'cancelling') {
        setChatRuntime({
          status: 'cancelling',
          phase: progress.phase,
          percent: progress.overallPercent,
          eta: progress.estimatedRemainingSeconds,
          message: runtimeMessage(progress.message, '正在停止本次任务'),
        });
        return;
      }

      if (progress.phase === 'cancelled') {
        if (
          (busyAction === 'rerunning' || busyAction === 'supplementing') &&
          (progress.message || '').includes('后台补全已停止')
        ) {
          return;
        }
        setChatRuntime({
          status: 'cancelled',
          phase: progress.phase,
          percent: progress.overallPercent,
          eta: 0,
          message: runtimeMessage(progress.message, '本次任务已停止并回滚'),
        });
        setOptimisticUserMessage('');
        setBusyAction(null);
        refreshSessionIfNeeded(true);
        return;
      }

      if (progress.phase === 'completed') {
        setChatRuntime({
          status: 'completed',
          phase: progress.phase,
          percent: progress.overallPercent,
          eta: 0,
          message: runtimeMessage(progress.message, '任务完成'),
        });
        if (activeDeepStartSession?.summary.processingStatus === 'background_processing' || (progress.backgroundCompleted ?? 0) > 0) {
          refreshSessionIfNeeded(true);
        }
        return;
      }

      if (
        progress.phase === 'searching' ||
        progress.phase === 'enriching' ||
        progress.phase === 'downloading' ||
        progress.phase === 'parsing' ||
        progress.phase === 'weak_extracting' ||
        progress.phase === 'initial_batch_ready' ||
        progress.phase === 'background_processing' ||
        progress.phase === 'analyzing' ||
        progress.phase === 'persisting'
      ) {
        setChatRuntime({
          status: 'running',
          phase: progress.phase,
          percent: progress.overallPercent,
          eta: progress.estimatedRemainingSeconds,
          message: runtimeMessage(progress.message, '后台正在处理中'),
        });
        if (progress.phase === 'initial_batch_ready' || progress.phase === 'background_processing') {
          refreshSessionIfNeeded();
        }
      }
    });
  }, [
    activeDeepStartSession?.summary.id,
    activeDeepStartSession?.summary.processingStatus,
    busyAction,
    setActiveFolderId,
    setError,
    upsertDeepStartSession,
  ]);

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
  const chatMessages = useMemo(() => {
    const base = activeDeepStartSession?.messages ?? [];
    if (!optimisticUserMessage.trim()) {
      return base.slice(-20);
    }
    const optimistic = {
      id: `optimistic-${Date.now()}`,
      sessionId: activeDeepStartSession?.summary.id ?? '',
      role: 'user' as const,
      content: optimisticUserMessage,
      createdAt: new Date().toISOString(),
    };
    return [...base, optimistic].slice(-20);
  }, [activeDeepStartSession?.messages, activeDeepStartSession?.summary.id, optimisticUserMessage]);
  const firstAssistantMessageId = useMemo(
    () => (activeDeepStartSession?.messages ?? []).find((message) => message.role === 'assistant')?.id ?? '',
    [activeDeepStartSession?.messages]
  );

  useEffect(() => {
    if (!activePaperId) {
      return;
    }
    if (!paperById.has(activePaperId)) {
      setActivePaperId(null);
    }
  }, [activePaperId, paperById]);

  useEffect(() => {
    setCopyFeedback('');
  }, [activePaperId]);

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

  const folderOptions = useMemo(() => {
    if (folderTree.length > 0) {
      return flattenFolderTree(folderTree);
    }
    return folders.map((folder) => ({ id: folder.id, label: folder.path || folder.name }));
  }, [folderTree, folders]);

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
    if (busyAction === 'replying' || busyAction === 'rerunning') {
      return;
    }

    setBusyAction('replying');
    setOptimisticUserMessage(content);
    setReplyInput('');
    setChatRuntime({
      status: 'running',
      phase: 'analyzing',
      percent: 35,
      eta: 0,
      message: '正在基于当前候选池生成会话内缩窄建议',
    });
    try {
      const detail = await replyDeepStartSession(activeDeepStartSession.summary.id, content);
      persistSession(detail);
      setOptimisticUserMessage('');
      setChatRuntime((prev) => ({
        ...prev,
        status: 'completed',
        phase: 'completed',
        percent: 100,
        eta: 0,
        message: '会话内缩窄完成',
      }));
    } catch (error) {
      setOptimisticUserMessage('');
      if (isCancellationError(error)) {
        setChatRuntime({
          status: 'cancelled',
          phase: 'cancelled',
          percent: 0,
          eta: 0,
          message: '本次会话内缩窄已停止',
        });
        return;
      }
      const message = errorToUserMessage(error, '发送 DeepStart 对话失败');
      setChatRuntime((prev) => ({
        ...prev,
        status: 'failed',
        phase: 'failed',
        message,
      }));
      setError(message);
    } finally {
      setBusyAction(null);
    }
  };

  const handleUndoNarrow = async () => {
    if (!activeDeepStartSession) {
      return;
    }
    if (busyAction) {
      return;
    }

    setBusyAction('undoing');
    try {
      const detail = await undoDeepStartNarrow(activeDeepStartSession.summary.id);
      persistSession(detail);
      setImportFeedback(`已回退上一轮缩窄，当前候选池 ${detail.currentResults.length} 篇。`);
    } catch (error) {
      setError(errorToUserMessage(error, '回退上一轮失败'));
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
    if (busyAction === 'rerunning' || busyAction === 'replying') {
      return;
    }

    setBusyAction('rerunning');
    setChatRuntime({
      status: 'running',
      phase: 'searching',
      percent: 3,
      eta: 60,
      message: '正在根据新 query 重新检索候选',
    });
    try {
      const detail = await rerunDeepStartSearch(activeDeepStartSession.summary.id, nextQuery);
      persistSession(detail);
      setRerunQuery(nextQuery);
      setActivePaperId(null);
      setChatRuntime((prev) => ({
        ...prev,
        status: 'completed',
        phase: 'completed',
        percent: 100,
        eta: 0,
        message: '新一轮候选已准备完成',
      }));
    } catch (error) {
      if (isCancellationError(error)) {
        setChatRuntime((prev) => ({
          ...prev,
          status: 'cancelled',
          phase: 'cancelled',
          eta: 0,
          message: '本次重搜已停止，原会话保持不变',
        }));
        return;
      }
      const message = errorToUserMessage(error, '重新检索失败');
      setChatRuntime((prev) => ({
        ...prev,
        status: 'failed',
        phase: 'failed',
        message,
      }));
      setError(message);
    } finally {
      setBusyAction(null);
    }
  };

  const handleSupplementSearch = async (query?: string) => {
    if (!activeDeepStartSession) {
      setError('请先开始一轮探索');
      return;
    }

    const nextQuery = (query ?? replyInput).trim();
    if (!nextQuery) {
      setError('请输入补充检索 query');
      return;
    }
    if (busyAction === 'supplementing' || busyAction === 'replying' || busyAction === 'rerunning') {
      return;
    }

    const limit = Math.max(5, Math.min(100, Number(supplementPerSourceLimit) || 20));
    setBusyAction('supplementing');
    setChatRuntime({
      status: 'running',
      phase: 'searching',
      percent: 3,
      eta: 60,
      message: `正在发起补充检索（每源 ${limit} 篇）`,
    });
    try {
      const detail = await supplementDeepStartSearch(activeDeepStartSession.summary.id, nextQuery, limit);
      persistSession(detail);
      setReplyInput('');
      setRerunQuery(nextQuery);
      setChatRuntime({
        status: 'completed',
        phase: 'completed',
        percent: 100,
        eta: 0,
        message: `补充检索完成，当前候选池 ${detail.currentResults.length} 篇`,
      });
    } catch (error) {
      if (isCancellationError(error)) {
        setChatRuntime((prev) => ({
          ...prev,
          status: 'cancelled',
          phase: 'cancelled',
          eta: 0,
          message: '补充检索已停止，原会话保持不变',
        }));
        return;
      }
      const message = errorToUserMessage(error, '补充检索失败');
      setChatRuntime((prev) => ({
        ...prev,
        status: 'failed',
        phase: 'failed',
        message,
      }));
      setError(message);
    } finally {
      setBusyAction(null);
    }
  };

  const handleCancelRunningTask = async () => {
    if (!activeDeepStartSession) {
      return;
    }

    setChatRuntime((prev) => ({
      ...prev,
      status: 'cancelling',
      phase: 'cancelling',
      message: '正在停止本次任务并回滚暂存结果',
      eta: 1,
    }));
    try {
      await cancelDeepStartTask(activeDeepStartSession.summary.id);
    } catch (error) {
      const message = errorToUserMessage(error, '停止任务失败');
      setChatRuntime((prev) => ({
        ...prev,
        status: 'failed',
        phase: 'failed',
        message,
      }));
      setError(message);
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
      setError(errorToUserMessage(error, '保存 DeepStart 勾选状态失败'));
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

  const handleCreateFolder = () => {
    setNewFolderError('');
    setNewFolderPath('');
    setIsFolderModalOpen(true);
  };

  const submitCreateFolder = async () => {
    const path = newFolderPath.trim();
    if (!path) {
      setNewFolderError('请填写目录路径');
      return;
    }
    const pathValidationError = validateFolderPathInput(path);
    if (pathValidationError) {
      setNewFolderError(pathValidationError);
      return;
    }

    setIsCreatingFolder(true);
    try {
      const folder = await createFolderNode({
        path,
      });
      const [tree, overview] = await Promise.all([
        getFolderTree(),
        getFolderStorageTreeOverview(),
      ]);
      setFolderTree(tree);
      setFolders(flattenFolderNodes(tree));
      setStorageTreeOverview(overview);
      await handleTargetFolderChange(folder.id);
      setIsFolderModalOpen(false);
      setImportFeedback(`已创建文件夹「${folder.path || folder.name}」`);
    } catch (error) {
      const message = errorToUserMessage(error, '创建文件夹失败');
      setNewFolderError(message);
      setError(message);
    } finally {
      setIsCreatingFolder(false);
    }
  };

  const handleDeleteFolder = async (folderId: string) => {
    if (busyAction) {
      return;
    }

    setBusyAction('selecting');
    try {
      await deleteFolderNode(folderId);
      const [tree, overview] = await Promise.all([
        getFolderTree(),
        getFolderStorageTreeOverview(),
      ]);
      setFolderTree(tree);
      const flattened = flattenFolderNodes(tree);
      setFolders(flattened);
      setStorageTreeOverview(overview);

      const nextTarget = flattened.find((folder) => folder.isSystem) ?? flattened[0];
      if (nextTarget && activeDeepStartSession) {
        await handlePersistSelections([...selectedPaperIds], nextTarget.id);
      }
      setImportFeedback('目录已删除。');
    } catch (error) {
      setError(errorToUserMessage(error, '删除目录失败'));
    } finally {
      setBusyAction(null);
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
    setImportFeedback('');
    try {
      const result = await importPapersWithAssets(folderId, selected);
      const [papers, treeOverview] = await Promise.all([
        getPapers(folderId),
        getFolderStorageTreeOverview(),
      ]);
      setActiveFolderId(folderId);
      setPapers(papers);
      setStorageTreeOverview(treeOverview);
      if (result.imported[0]) {
        setSelectedPaper(result.imported[0]);
      }
      if (activeDeepStartSession) {
        const cleared = await updateDeepStartSelections(
          activeDeepStartSession.summary.id,
          [],
          folderId
        );
        persistSession(cleared);
      }
      const skippedText = result.skipped.length > 0 ? `，跳过重复 ${result.skipped.length} 篇` : '';
      setImportFeedback(
        result.message ||
          `已导入并清空选中：新增 ${result.imported.length} 篇${skippedText}，后台继续下载 PDF。`
      );
    } catch (error) {
      setError(errorToUserMessage(error, '导入论文失败'));
    } finally {
      setBusyAction(null);
    }
  };

  const handleCopyActiveAbstract = async () => {
    const abstract = (activePaper?.abstract || '').trim();
    if (!abstract) {
      setCopyFeedback('没有可复制的摘要');
      return;
    }
    if (!navigator.clipboard?.writeText) {
      setCopyFeedback('当前环境不支持剪贴板写入');
      return;
    }
    try {
      await navigator.clipboard.writeText(abstract);
      setCopyFeedback('摘要已复制到剪贴板');
    } catch (error) {
      const message = errorToUserMessage(error, '复制摘要失败');
      setCopyFeedback(message);
      setError(message);
    }
  };

  const renderStorageNode = (node: FolderStorageTreeOverview['directories'][number], depth = 0) => {
    const queueing = node.queued + node.downloading;
    const isTarget = activeTargetFolderId === node.folderId;

    return (
      <div key={node.folderId} className="space-y-1">
        <div
          className={`rounded-[var(--de-radius)] border px-3 py-2 text-xs ${
            isTarget
              ? 'border-[var(--de-accent)] bg-[var(--de-accent-soft)] text-[var(--de-accent)]'
              : 'border-[var(--de-rule)] bg-[var(--de-surface)] text-[var(--de-ink)]'
          }`}
          style={{ marginLeft: depth * 10 }}
        >
          <div className="flex items-center justify-between gap-2">
            <button
              type="button"
              onClick={() => void handleTargetFolderChange(node.folderId)}
              className="truncate text-left font-medium hover:text-[var(--de-accent)]"
            >
              {node.folderName}
            </button>
            {(() => {
              const folder = folders.find((item) => item.id === node.folderId);
              if (folder?.isSystem) {
                return (
                  <span className="rounded-full bg-slate-100 px-2 py-0.5 text-[10px] text-slate-500 dark:bg-slate-700 dark:text-slate-300">
                    系统
                  </span>
                );
              }
              return (
                <button
                  type="button"
                  onClick={() => void handleDeleteFolder(node.folderId)}
                  className="rounded border border-rose-200 px-1.5 py-0.5 text-[10px] text-rose-600 hover:bg-rose-50 dark:border-rose-500/40 dark:text-rose-200 dark:hover:bg-rose-500/15"
                >
                  删除
                </button>
              );
            })()}
          </div>
          <p className="mt-1 text-[11px] text-slate-500 dark:text-slate-300">
            论文卡片: {node.paperCount} · 排队中: {queueing}/{node.paperCount || 0} · 已下载: {node.downloaded}/
            {node.paperCount || 0} · 下载失败: {node.failed}/{node.paperCount || 0}
          </p>
        </div>
        {node.children.map((child) => renderStorageNode(child, depth + 1))}
      </div>
    );
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
        className={`cursor-pointer rounded-[var(--de-radius)] border p-4 text-left transition-colors focus:outline-none focus:ring-2 focus:ring-[var(--de-accent)] ${
          isSelected
            ? 'border-[var(--de-accent)] bg-[var(--de-accent-soft)]'
            : isRecommended
              ? 'border-[var(--de-rule-strong)] bg-[var(--de-surface)] hover:border-[var(--de-accent)]'
              : 'border-[var(--de-rule)] bg-[var(--de-surface)] hover:border-[var(--de-rule-strong)]'
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
              {publicationLabel(paper)}
            </p>
            <p className="mt-1 text-xs text-slate-500 dark:text-slate-300">
              来源：{sourceLabel(paper)} · {institutionLabel(paper)}
            </p>
            {paper.matchReason ? (
              <p className="mt-1.5 flex items-start gap-1.5 text-xs leading-5 text-[var(--de-accent)]">
                <Search className="mt-0.5 h-3.5 w-3.5 shrink-0" />
                <span>{paper.matchReason}</span>
              </p>
            ) : null}
          </div>
          <button
            type="button"
            disabled={isSelectionActionBlocked}
            onClick={(event) => {
              event.stopPropagation();
              void toggleSelect(paper.id);
            }}
            className={`mt-1 inline-flex shrink-0 items-center justify-center rounded-[var(--de-radius)] border px-2.5 py-1 text-xs font-medium ${
              isSelected
                ? 'border-[var(--de-accent)] bg-[var(--de-accent)] text-white'
                : 'border-[var(--de-rule-strong)] text-[var(--de-ink-muted)] hover:border-[var(--de-accent)] hover:text-[var(--de-accent)]'
            }`}
          >
            {isSelected ? '已选择' : '选择'}
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
            className="de-button-primary mt-4 px-4 py-2"
          >
            返回历史
          </button>
        </div>
      </div>
    );
  }

  const activeTargetFolderId =
    activeDeepStartSession.summary.targetFolderId || activeFolderId || folderOptions[0]?.id || folders[0]?.id || '';
  const searchStats = currentAnalysis?.searchStats;
  const currentPoolCount = activeDeepStartSession.currentResults.length;
  const selectedCount = selectedPaperIds.size;
  const isSelectionActionBlocked = busyAction === 'selecting' || busyAction === 'importing' || busyAction === 'undoing';
  const isRerunBlocked =
    busyAction === 'rerunning' || busyAction === 'replying' || busyAction === 'supplementing' || busyAction === 'undoing';
  const isReplyBlocked =
    busyAction === 'replying' || busyAction === 'rerunning' || busyAction === 'supplementing' || busyAction === 'undoing';
  const showRerunStop =
    busyAction === 'rerunning' || busyAction === 'supplementing' || chatRuntime.status === 'cancelling';
  const isChatThinking =
    chatRuntime.status === 'running' &&
    (chatRuntime.phase === 'analyzing' || chatRuntime.phase === 'persisting');

  let importDisabledReason = '';
  if (busyAction === 'importing') {
    importDisabledReason = '导入进行中';
  } else if (busyAction === 'rerunning') {
    importDisabledReason = '重搜进行中，请等待结果稳定后再导入';
  } else if (!activeTargetFolderId) {
    importDisabledReason = '请选择目标文件夹';
  } else if (selectedCount === 0) {
    importDisabledReason = '请先选择至少一篇论文';
  }

  return (
    <div className="de-session flex h-full min-h-0 flex-col overflow-hidden bg-[var(--de-paper)] text-[var(--de-ink)]">
      <div className="border-b border-[var(--de-rule)] bg-[var(--de-surface)] px-6 py-3">
        <div className="mx-auto flex max-w-7xl flex-wrap items-center gap-3">
          <button
            onClick={() => navigate('/history')}
            className="de-button-secondary inline-flex items-center gap-1.5 px-3 py-1.5 text-sm"
          >
            <ArrowLeft className="h-4 w-4" />
            返回历史
          </button>

          <div className="min-w-[220px] flex-1">
            <p className="text-xs font-medium text-[var(--de-accent)]">研究工作区</p>
            <h2 className="de-display truncate text-lg font-semibold">{activeDeepStartSession.summary.title}</h2>
          </div>

          <select
            value={activeTargetFolderId}
            onChange={(event) => void handleTargetFolderChange(event.target.value)}
            className="de-field px-3 py-2 text-sm"
          >
            {folderOptions.map((folder) => (
              <option key={folder.id} value={folder.id}>
                入库到：{folder.label}
              </option>
            ))}
          </select>

          <button
            onClick={handleCreateFolder}
            disabled={isCreatingFolder}
            className="de-button-secondary inline-flex items-center gap-1.5 px-3 py-2 text-sm disabled:opacity-60"
          >
            <FolderPlus className="h-4 w-4" />
            新建文件夹
          </button>
        </div>

        <div className="mx-auto mt-3 max-w-7xl border-t border-[var(--de-rule)] pt-3">
          <div className="flex flex-wrap items-center gap-2">
            <Search className="h-4 w-4 text-[var(--de-ink-muted)]" />
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
              disabled={isRerunBlocked}
              className="de-button-primary inline-flex items-center gap-1.5 px-3 py-1.5 text-sm disabled:opacity-60"
            >
              {busyAction === 'rerunning' ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
              重搜
            </button>
            {showRerunStop && (
              <button
                onClick={() => void handleCancelRunningTask()}
                className="de-button-secondary inline-flex items-center gap-1.5 px-3 py-1.5 text-sm text-[var(--de-danger)]"
              >
                <Loader2 className={`h-4 w-4 ${chatRuntime.status === 'cancelling' ? 'animate-spin' : ''}`} />
                停止
              </button>
            )}
            <button
              type="button"
              onClick={() => void handleUndoNarrow()}
              disabled={Boolean(busyAction)}
              className="de-button-secondary inline-flex items-center gap-1.5 px-3 py-1.5 text-sm disabled:cursor-not-allowed disabled:opacity-60"
            >
              {busyAction === 'undoing' ? <Loader2 className="h-4 w-4 animate-spin" /> : <ArrowLeft className="h-4 w-4" />}
              回退上一轮
            </button>
          </div>
        </div>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto px-6 pb-28">
        <div className="sticky top-0 z-30 -mx-6 border-b border-[var(--de-rule)] bg-[var(--de-paper)] px-6 py-3">
          <section className="mx-auto max-w-7xl border border-[var(--de-rule)] bg-[var(--de-surface)] p-4">
            <div className="flex items-center justify-between gap-3">
              <h3 className="text-sm font-semibold text-[var(--de-ink)]">研究助理</h3>
              <button
                type="button"
                onClick={() => setChatCollapsed((value) => !value)}
                className="de-button-secondary inline-flex items-center gap-1 px-2.5 py-1 text-xs"
              >
                {chatCollapsed ? <ChevronDown className="h-3.5 w-3.5" /> : <ChevronUp className="h-3.5 w-3.5" />}
                {chatCollapsed ? '展开' : '收起'}
              </button>
            </div>

            {!chatCollapsed && (
              <div className="mt-3 space-y-3">
                {chatRuntime.status !== 'idle' && chatRuntime.message && (
                  <div
                    className={`border border-[var(--de-rule)] bg-[var(--de-surface-muted)] px-3 py-2 text-xs ${
                      chatRuntime.status === 'failed'
                        ? 'text-[var(--de-danger)]'
                        : chatRuntime.status === 'cancelled'
                          ? 'text-[var(--de-warning)]'
                          : 'text-[var(--de-ink)]'
                    }`}
                  >
                    <div className="flex items-center justify-between gap-2">
                      <span className="inline-flex items-center gap-1.5">
                        {(isChatThinking || chatRuntime.status === 'cancelling') && (
                          <Loader2 className="h-3.5 w-3.5 animate-spin" />
                        )}
                        {chatRuntime.message}
                      </span>
                      {!isChatThinking && <span>{chatRuntime.percent}%</span>}
                    </div>
                    {(chatRuntime.status === 'running' || chatRuntime.status === 'cancelling') && !isChatThinking && (
                      <>
                        <div className="mt-2 h-1.5 bg-[var(--de-surface-muted)]">
                          <div
                            className="h-1.5 bg-[var(--de-accent)] transition-[width] duration-150"
                            style={{ width: `${Math.max(0, Math.min(100, chatRuntime.percent))}%` }}
                          />
                        </div>
                        {chatRuntime.eta > 0 && <div className="mt-1 text-[11px] opacity-80">预计剩余 {chatRuntime.eta}s</div>}
                      </>
                    )}
                  </div>
                )}

                <div className="max-h-48 space-y-2 overflow-y-auto">
                  {chatMessages.map((message) => (
                    <div
                      key={message.id}
                      className={`px-3 py-2 text-xs leading-6 ${
                        message.role === 'assistant'
                          ? 'border-l-2 border-[var(--de-rule-strong)] text-[var(--de-ink)]'
                          : 'ml-4 bg-[var(--de-accent)] text-white'
                      }`}
                    >
                      {message.content}
                      {message.role === 'assistant' &&
                        message.id === firstAssistantMessageId &&
                        initialSuggestedQueries.length > 0 && (
                          <div className="mt-2 flex flex-wrap gap-2">
                            {initialSuggestedQueries.map((query) => (
                              <button
                                key={`${message.id}-${query}`}
                                onClick={() => {
                                  setReplyInput(query);
                                  replyInputRef.current?.focus();
                                }}
                                className="border-b border-[var(--de-rule)] px-1 py-1 text-[11px] text-[var(--de-ink-muted)] transition-colors hover:border-[var(--de-accent)] hover:text-[var(--de-accent)]"
                              >
                                {query}
                              </button>
                            ))}
                          </div>
                        )}
                    </div>
                  ))}
                </div>

                <div className="flex gap-2">
                  <textarea
                    ref={replyInputRef}
                    value={replyInput}
                    onChange={(event) => setReplyInput(event.target.value)}
                    onKeyDown={(event) => {
                      if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') {
                        event.preventDefault();
                        void handleReply(replyInput);
                      }
                    }}
                    placeholder="补充你的筛选偏好"
                    rows={3}
                    className="de-field min-h-[74px] min-w-0 flex-1 resize-y px-3 py-2 text-xs"
                  />
                  <button
                    onClick={() => void handleReply(replyInput)}
                    disabled={isReplyBlocked || !replyInput.trim()}
                    className="de-button-primary inline-flex h-fit items-center gap-1.5 px-3 py-2 text-xs disabled:opacity-60"
                  >
                    {busyAction === 'replying' ? (
                      <>
                        <Loader2 className="h-3.5 w-3.5 animate-spin" />
                        处理中
                      </>
                    ) : (
                      '发送'
                    )}
                  </button>
                  <div className="flex flex-col gap-2">
                    <div className="flex items-center gap-1">
                      <span className="text-[11px] text-slate-500 dark:text-slate-300">每源</span>
                      <input
                        type="text"
                        inputMode="numeric"
                        min={5}
                        max={100}
                        value={supplementPerSourceLimit}
                        onFocus={(event) => event.currentTarget.select()}
                        onChange={(event) => {
                          const digits = event.target.value.replace(/\D/g, '').slice(0, 3);
                          setSupplementPerSourceLimit(digits === '' ? 0 : Number(digits));
                        }}
                        onBlur={() => {
                          setSupplementPerSourceLimit((value) => Math.max(5, Math.min(100, Number(value) || 20)));
                        }}
                        className="de-field w-20 px-2 py-1 text-[11px]"
                      />
                    </div>
                    <button
                      onClick={() => void handleSupplementSearch(replyInput)}
                      disabled={isReplyBlocked || !replyInput.trim()}
                      className="de-button-secondary inline-flex h-fit items-center gap-1.5 px-3 py-2 text-xs disabled:opacity-60"
                    >
                      {busyAction === 'supplementing' ? (
                        <>
                          <Loader2 className="h-3.5 w-3.5 animate-spin" />
                          补检中
                        </>
                      ) : (
                        '发起补充检索'
                      )}
                    </button>
                  </div>
                </div>
                <p className="text-[11px] text-slate-500 dark:text-slate-300">Enter 换行，Ctrl/Cmd + Enter 发送</p>
              </div>
            )}
          </section>
        </div>

        <div className="mx-auto max-w-7xl pt-5">
          <div className="mt-5 grid gap-6 xl:grid-cols-[320px_1px_1fr]">
            <aside className="space-y-4">
              <section className="de-panel p-4">
                <h3 className="text-sm font-semibold text-[var(--de-ink)]">研究概览</h3>
                <p className="mt-2 text-sm leading-7 text-[var(--de-ink-muted)]">
                  {currentAnalysis?.overview || 'AI 正在构建这轮检索的分组策略。'}
                </p>
                {searchStats && (
                  <dl className="mt-3 border-t border-[var(--de-rule)] text-xs text-[var(--de-ink-muted)]">
                    {searchStats.originalQuery && (
                      <div className="border-b border-[var(--de-rule)] py-2">
                        <dt className="font-medium text-[var(--de-ink)]">原始问题</dt>
                        <dd className="mt-1 leading-5">{searchStats.originalQuery}</dd>
                      </div>
                    )}
                    {(searchStats.rewrittenQueries?.length ?? 0) > 0 && (
                      <div className="border-b border-[var(--de-rule)] py-2">
                        <dt className="font-medium text-[var(--de-ink)]">英文检索词</dt>
                        <dd className="mt-1 leading-5">{(searchStats.rewrittenQueries ?? []).join(' | ')}</dd>
                      </div>
                    )}
                    {searchStats.queryHits && Object.keys(searchStats.queryHits).length > 0 && (
                      <div className="border-b border-[var(--de-rule)] py-2">
                        <dt className="font-medium text-[var(--de-ink)]">重写命中</dt>
                        <dd className="mt-1 leading-5">
                        {Object.entries(searchStats.queryHits)
                          .map(([query, count]) => `${query}=${count}`)
                          .join('；')}
                        </dd>
                      </div>
                    )}
                    <div className="border-b border-[var(--de-rule)] py-2">
                      <dt className="font-medium text-[var(--de-ink)]">首轮检索基线</dt>
                      <dd className="mt-1">原始 {searchStats.rawCount} · 去重后 {searchStats.dedupCount} · 入池 {searchStats.finalCount}</dd>
                    </div>
                    <div className="border-b border-[var(--de-rule)] py-2">
                      <dt className="font-medium text-[var(--de-ink)]">当前候选池</dt>
                      <dd className="mt-1">{currentPoolCount} 篇</dd>
                    </div>
                    {(activeDeepStartSession.summary.totalPlannedCount ?? 0) > 0 && (
                      <div className="py-2">
                        <dt className="font-medium text-[var(--de-ink)]">处理状态</dt>
                        <dd className="mt-1 leading-5">
                          {activeDeepStartSession.summary.processingStatus === 'background_processing' ? '后台处理中' : '已完成'} · 首批就绪 {activeDeepStartSession.summary.initialReadyCount || currentPoolCount} / 总计划 {activeDeepStartSession.summary.totalPlannedCount} · 后台剩余 {activeDeepStartSession.summary.backgroundRemaining || 0}
                        </dd>
                      </div>
                    )}
                  </dl>
                )}
              </section>

              <section className="de-panel p-4">
                <h3 className="text-sm font-semibold text-[var(--de-ink)]">研究方向</h3>
                <div className="mt-3 space-y-2">
                  {groupedDirections.map((direction) => {
                    const selectedCount = direction.paperIds.filter((paperId) => selectedPaperIds.has(paperId)).length;
                    return (
                      <div key={direction.id} className="border-b border-[var(--de-rule)] py-3 last:border-b-0">
                        <div className="flex items-center justify-between">
                          <div className="text-sm font-medium">{direction.name}</div>
                          <span className="text-[11px] tabular-nums text-[var(--de-ink-muted)]">{direction.paperIds.length} 篇</span>
                        </div>
                        <p className="mt-1 text-xs leading-5 text-slate-500 dark:text-slate-300">{direction.summary}</p>
                        <div className="mt-2 flex gap-2">
                          <button
                            onClick={() => void toggleDirection(direction, true)}
                            disabled={isSelectionActionBlocked}
                            className="de-button-secondary px-2.5 py-1 text-[11px] disabled:opacity-60"
                          >
                            全选 {selectedCount}/{direction.paperIds.length}
                          </button>
                          <button
                            onClick={() => void toggleDirection(direction, false)}
                            disabled={isSelectionActionBlocked}
                            className="de-button-secondary px-2.5 py-1 text-[11px] disabled:opacity-60"
                          >
                            清空
                          </button>
                        </div>
                      </div>
                    );
                  })}
                </div>
              </section>

              <section className="de-panel p-4">
                <div className="flex items-center justify-between gap-2">
                  <h3 className="text-xs uppercase tracking-[0.18em] text-slate-500 dark:text-slate-300">本地存储速览</h3>
                  {loadingStorageOverview && <Loader2 className="h-3.5 w-3.5 animate-spin text-[var(--de-accent)]" />}
                </div>
                {storageTreeOverview ? (
                  <div className="mt-3 max-h-64 space-y-2 overflow-y-auto text-xs text-slate-600 dark:text-slate-200">
                    {storageTreeOverview.directories.map((node) => renderStorageNode(node))}
                  </div>
                ) : (
                  <p className="mt-3 text-xs text-slate-500 dark:text-slate-300">暂时无法读取本地目录状态。</p>
                )}
              </section>
            </aside>

            <div className="hidden border-l border-dashed border-slate-300 xl:block dark:border-slate-600/60" />

            <section className="space-y-4">
              {currentResults.length > 0 ? (
                groupedDirections.map((direction) => (
                  <section key={direction.id} className="border-t border-[var(--de-rule-strong)] pt-4 first:border-t-0 first:pt-0">
                    <div className="mb-3 flex items-start justify-between gap-3">
                      <div>
                        <h3 className="text-base font-semibold">{direction.name}</h3>
                        <p className="mt-1 text-sm text-slate-500 dark:text-slate-300">{direction.why}</p>
                      </div>
                      <span className="text-xs tabular-nums text-[var(--de-ink-muted)]">{direction.paperIds.length} 篇</span>
                    </div>
                    <div className="grid gap-3 sm:grid-cols-2">
                      {direction.paperIds
                        .map((paperId) => paperById.get(paperId))
                        .filter((paper): paper is SearchPaper => Boolean(paper))
                        .map((paper) => renderPaperCard(paper))}
                    </div>
                  </section>
                ))
              ) : (
                <div className="border-y border-[var(--de-rule)] py-10 text-center">
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
            className="fixed inset-0 z-30 bg-slate-950/35"
            aria-label="关闭详情抽屉"
          />
          <aside className="fixed right-0 top-0 z-40 flex h-full w-full max-w-2xl flex-col border-l border-[var(--de-rule)] bg-[var(--de-surface)] p-5 shadow-lg">
            <div className="flex items-start justify-between gap-4">
              <div>
                <p className="text-xs uppercase tracking-[0.16em] text-slate-500 dark:text-slate-400">Paper Detail</p>
                <h3 className="mt-2 text-lg font-semibold leading-7 text-slate-950 dark:text-slate-50">{activePaper.title}</h3>
                <p className="mt-1 text-sm text-slate-600 dark:text-slate-300">
                  {publicationLabel(activePaper)}
                </p>
                <p className="mt-1 text-xs text-slate-500 dark:text-slate-400">
                  来源：{sourceLabel(activePaper)}
                </p>
              </div>
              <button
                type="button"
                onClick={() => setActivePaperId(null)}
                className="de-button-secondary p-2 text-[var(--de-ink-muted)]"
              >
                <X className="h-4 w-4" />
              </button>
            </div>

            <div className="mt-4 flex flex-wrap gap-2">
              <button
                type="button"
                onClick={() => void toggleSelect(activePaper.id)}
                disabled={isSelectionActionBlocked}
                className={`rounded-[var(--de-radius)] px-3 py-1.5 text-xs font-medium ${
                  selectedPaperIds.has(activePaper.id)
                    ? 'bg-[var(--de-accent)] text-white'
                    : 'de-button-secondary'
                }`}
              >
                {selectedPaperIds.has(activePaper.id) ? '已选中' : '加入选中'}
              </button>
              {activePaper.url && (
                <a
                  href={activePaper.url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="de-button-secondary inline-flex items-center gap-1 px-3 py-1.5 text-xs"
                >
                  <ExternalLink className="h-3.5 w-3.5" />
                  打开链接
                </a>
              )}
            </div>

            <div className="mt-4 grid gap-3 overflow-y-auto pr-1">
              {activePaper.matchReason ? (
                <section className="border-y border-[var(--de-rule)] bg-[var(--de-surface-muted)] px-3 py-2.5">
                  <h4 className="text-xs font-semibold text-[var(--de-accent)]">检索匹配</h4>
                  <p className="mt-1 text-sm leading-6 text-slate-700 dark:text-slate-200">{activePaper.matchReason}</p>
                  {activePaper.matchedTerms?.length ? (
                    <p className="mt-1 text-xs text-slate-500 dark:text-slate-400">
                      命中词：{activePaper.matchedTerms.join(' · ')}
                    </p>
                  ) : null}
                </section>
              ) : null}
              <section className="border-b border-[var(--de-rule)] px-1 py-3">
                <h4 className="text-xs font-semibold text-[var(--de-ink-muted)]">机构 / 学校</h4>
                <p className="mt-2 text-sm leading-6 text-slate-700 dark:text-slate-200">{institutionLabel(activePaper)}</p>
                {activePaper.enrichmentNote && (
                  <p className="mt-2 text-xs leading-5 text-amber-700 dark:text-amber-300">{activePaper.enrichmentNote}</p>
                )}
              </section>

              <section className="border-b border-[var(--de-rule)] px-1 py-3">
                <h4 className="text-xs font-semibold text-[var(--de-ink-muted)]">关键词</h4>
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

              <section className="border-b border-[var(--de-rule)] px-1 py-3">
                <h4 className="text-xs font-semibold text-[var(--de-ink-muted)]">作者列表</h4>
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

              <section className="px-1 py-3">
                <div className="flex items-center justify-between gap-2">
                  <h4 className="text-xs font-semibold text-[var(--de-ink-muted)]">完整摘要</h4>
                  <button
                    type="button"
                    onClick={() => void handleCopyActiveAbstract()}
                    className="de-button-secondary px-2 py-1 text-xs"
                  >
                    复制摘要
                  </button>
                </div>
                {copyFeedback && (
                  <p className="mt-2 text-xs text-slate-500 dark:text-slate-300">{copyFeedback}</p>
                )}
                <p className="mt-2 whitespace-pre-wrap text-sm leading-7 text-slate-700 dark:text-slate-200">
                  {useLegacyAbstractHighlight
                    ? renderLegacyHighlightedText(activePaper.abstract || '', summaryHighlightTokens)
                    : renderInlineHighlightedAbstract(activePaper.abstract || '', summaryHighlightTokens)}
                </p>
              </section>
            </div>
          </aside>
        </>
      )}

      {isFolderModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/40 px-4">
          <div className="de-panel w-full max-w-md p-5 shadow-lg">
            <h3 className="text-base font-semibold">新建文件夹</h3>
            <p className="mt-1 text-sm text-slate-500 dark:text-slate-300">仅支持路径输入，按分段自动创建多级目录（全局根级）。</p>
            <input
              type="text"
              autoFocus
              value={newFolderPath}
              onChange={(event) => {
                const nextValue = event.target.value;
                setNewFolderPath(nextValue);
                const validationError = validateFolderPathInput(nextValue);
                if (validationError) {
                  setNewFolderError(validationError);
                } else if (newFolderError) {
                  setNewFolderError('');
                }
              }}
              onKeyDown={(event) => {
                if (event.key === 'Enter') {
                  event.preventDefault();
                }
              }}
              placeholder="输入路径，例如 Robotics/VLA/Benchmarks"
              className="de-field mt-3 w-full px-3 py-2 text-sm"
            />
            {newFolderError && <p className="mt-2 text-xs text-rose-600 dark:text-rose-300">{newFolderError}</p>}
            <div className="mt-4 flex justify-end gap-2">
              <button
                type="button"
                onClick={() => setIsFolderModalOpen(false)}
                className="de-button-secondary px-3 py-2 text-sm"
              >
                取消
              </button>
              <button
                type="button"
                onClick={() => void submitCreateFolder()}
                disabled={isCreatingFolder}
                className="de-button-primary inline-flex items-center gap-1 px-3 py-2 text-sm disabled:opacity-60"
              >
                {isCreatingFolder ? <Loader2 className="h-4 w-4 animate-spin" /> : <FolderPlus className="h-4 w-4" />}
                创建
              </button>
            </div>
          </div>
        </div>
      )}

      <div className="pointer-events-none fixed bottom-6 left-1/2 z-30 w-full max-w-7xl -translate-x-1/2 px-6">
        <div className="pointer-events-auto mx-auto max-w-xl rounded-[var(--de-radius)] border border-[var(--de-rule-strong)] bg-[var(--de-surface)] px-4 py-3 shadow-md">
          <div className="flex items-center justify-between gap-3">
            <div className="flex items-center gap-2 text-sm text-slate-600 dark:text-slate-200">
              <CheckCheck className="h-4 w-4 text-[var(--de-accent)]" />
              当前已选 <span className="font-semibold text-[var(--de-accent)]">{selectedCount}</span> 篇论文
            </div>
            <button
              onClick={() => void handleImportSelected()}
              disabled={Boolean(importDisabledReason)}
              title={importDisabledReason || '导入到目标文件夹并后台下载 PDF'}
              className="de-button-primary inline-flex items-center gap-1.5 px-4 py-2 text-sm font-medium disabled:cursor-not-allowed disabled:opacity-60"
            >
              {busyAction === 'importing' ? <Loader2 className="h-4 w-4 animate-spin" /> : <Plus className="h-4 w-4" />}
              {busyAction === 'importing' ? '导入中' : '导入选中'}
            </button>
          </div>
          {(importDisabledReason || importFeedback) && (
            <p className="mt-2 text-xs text-slate-500 dark:text-slate-300">{importFeedback || importDisabledReason}</p>
          )}
          {importFeedback && (
            <div className="mt-2 flex flex-wrap gap-2">
              <button
                type="button"
                onClick={() => navigate('/deepread')}
                className="de-button-secondary px-2.5 py-1 text-xs"
              >
                去 DeepRead
              </button>
              <button
                type="button"
                onClick={() => navigate('/history')}
                className="de-button-secondary px-2.5 py-1 text-xs"
              >
                去历史
              </button>
              <button
                type="button"
                onClick={() => navigate('/')}
                className="de-button-secondary px-2.5 py-1 text-xs"
              >
                回首页
              </button>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
