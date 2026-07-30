import { useEffect, useMemo, useState } from 'react';
import {
  ArrowRight,
  Check,
  Clock3,
  FileSearch,
  Loader2,
  Search,
  Square,
} from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import {
  cancelDeepStartTask,
  onDeepStartProgress,
  onSearchProgress,
  startDeepStartSession,
} from '../../lib/backend';
import { errorToUserMessage, isCancellationError } from '../../lib/errors';
import type { DeepStartProgressEvent, SearchProgressEvent } from '../../types';
import { useAppStore } from '../../stores/appStore';

const researchStarters = [
  '梳理近两年具身智能中 VLA 模型的关键路线与真实机器人评测',
  '寻找 LLM code agent 的代表论文、技术报告和主要 benchmark',
  '比较 RAG 与长上下文模型在复杂知识任务中的效果和成本',
];

const phaseLabels: Record<string, string> = {
  searching: '检索候选',
  enriching: '补全元信息',
  downloading: '获取全文',
  parsing: '解析论文',
  weak_extracting: '提取结构',
  initial_batch_ready: '首批结果可用',
  background_processing: '补全剩余结果',
  analyzing: '生成研究地图',
  persisting: '保存工作区',
  cancelling: '正在停止',
  cancelled: '已停止',
  completed: '完成',
};

function formatUpdatedAt(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '时间未知';
  return new Intl.DateTimeFormat('zh-CN', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  }).format(date);
}

export function DeepStartPanel() {
  const navigate = useNavigate();
  const {
    activeFolderId,
    deepStartSessions,
    folders,
    setActiveDeepStartSession,
    setError,
  } = useAppStore();

  const [prompt, setPrompt] = useState('');
  const [targetFolderId, setTargetFolderId] = useState(activeFolderId || folders[0]?.id || '');
  const [isStarting, setIsStarting] = useState(false);
  const [isCancelling, setIsCancelling] = useState(false);
  const [searchProgress, setSearchProgress] = useState<SearchProgressEvent | null>(null);
  const [deepStartProgress, setDeepStartProgress] = useState<DeepStartProgressEvent | null>(null);

  useEffect(() => {
    if (!targetFolderId && (activeFolderId || folders[0]?.id)) {
      setTargetFolderId(activeFolderId || folders[0]?.id || '');
    }
  }, [activeFolderId, folders, targetFolderId]);

  useEffect(() => onSearchProgress(setSearchProgress), []);
  useEffect(() => onDeepStartProgress(setDeepStartProgress), []);

  const progressPercent = useMemo(() => {
    if (deepStartProgress) return Math.max(0, Math.min(100, deepStartProgress.overallPercent || 0));
    if (!searchProgress) return 0;
    if (searchProgress.phase === 'completed') return 100;
    return searchProgress.totalSeconds > 0
      ? Math.min(95, Math.round((searchProgress.elapsedSeconds / searchProgress.totalSeconds) * 100))
      : 4;
  }, [deepStartProgress, searchProgress]);

  const runningSessionId = deepStartProgress?.sessionId?.trim() || '';
  const currentPhase = deepStartProgress?.phase || (isStarting ? 'searching' : '');
  const recentSessions = deepStartSessions.slice(0, 6);

  const handleStart = async () => {
    const researchQuestion = prompt.trim();
    if (!researchQuestion) {
      setError('请输入要研究的问题、方法或领域');
      return;
    }

    setError(null);
    setIsStarting(true);
    setIsCancelling(false);
    setSearchProgress(null);
    setDeepStartProgress({
      sessionId: '',
      phase: 'searching',
      message: '正在准备多源检索',
      elapsedSeconds: 0,
      estimatedRemainingSeconds: 60,
      total: 0,
      completed: 0,
      overallPercent: 2,
    });

    try {
      const detail = await startDeepStartSession(researchQuestion, targetFolderId);
      setActiveDeepStartSession(detail);
      navigate(`/session/${detail.summary.id}`);
    } catch (error) {
      if (isCancellationError(error)) {
        setDeepStartProgress((previous) => ({
          sessionId: previous?.sessionId || '',
          phase: 'cancelled',
          message: '本次检索已停止，没有写入未完成结果',
          elapsedSeconds: previous?.elapsedSeconds || 0,
          estimatedRemainingSeconds: 0,
          total: previous?.total || 0,
          completed: previous?.completed || 0,
          overallPercent: previous?.overallPercent || 0,
          stats: previous?.stats,
        }));
      } else {
        setError(errorToUserMessage(error, '无法完成论文检索'));
      }
    } finally {
      setIsStarting(false);
      setIsCancelling(false);
    }
  };

  const handleCancel = async () => {
    if (!runningSessionId) return;
    setIsCancelling(true);
    try {
      await cancelDeepStartTask(runningSessionId);
    } catch (error) {
      setError(errorToUserMessage(error, '停止检索失败'));
      setIsCancelling(false);
    }
  };

  return (
    <div className="h-full overflow-y-auto bg-[var(--de-paper)]">
      <div className="mx-auto w-full max-w-6xl px-6 py-8 lg:px-10 lg:py-10">
        <header className="max-w-3xl">
          <p className="text-sm font-medium text-[var(--de-accent)]">论文发现</p>
          <h1 className="de-display mt-2 text-3xl font-semibold leading-tight text-[var(--de-ink)]">
            从研究问题开始检索
          </h1>
          <p className="mt-3 max-w-2xl text-sm leading-6 text-[var(--de-ink-muted)]">
            描述你要理解的问题。系统会扩展检索词、合并多个来源、去重论文，并生成可继续追问的研究地图。
          </p>
        </header>

        <section className="mt-7 border-y border-[var(--de-rule)] bg-[var(--de-surface)]">
          <div className="grid min-h-[188px] grid-cols-1 lg:grid-cols-[1fr_220px]">
            <div className="border-b border-[var(--de-rule)] p-5 lg:border-b-0 lg:border-r">
              <label htmlFor="research-question" className="text-xs font-semibold text-[var(--de-ink)]">
                研究问题
              </label>
              <textarea
                id="research-question"
                value={prompt}
                onChange={(event) => setPrompt(event.target.value)}
                onKeyDown={(event) => {
                  if ((event.metaKey || event.ctrlKey) && event.key === 'Enter') {
                    event.preventDefault();
                    void handleStart();
                  }
                }}
                placeholder="例如：哪些方法真正提升了代码智能体在长任务中的可靠性？"
                className="mt-2 min-h-[108px] w-full resize-none border-0 bg-transparent p-0 text-lg leading-8 text-[var(--de-ink)] outline-none placeholder:text-[var(--de-ink-muted)]"
                disabled={isStarting}
              />
            </div>

            <div className="flex flex-col justify-between p-5">
              <div>
                <label htmlFor="target-folder" className="text-xs font-semibold text-[var(--de-ink)]">
                  保存到
                </label>
                <select
                  id="target-folder"
                  value={targetFolderId}
                  onChange={(event) => setTargetFolderId(event.target.value)}
                  className="de-field mt-2 w-full px-3 py-2 text-sm"
                  disabled={isStarting}
                >
                  {folders.map((folder) => (
                    <option key={folder.id} value={folder.id}>
                      {folder.path || folder.name}
                    </option>
                  ))}
                </select>
              </div>

              <button
                type="button"
                onClick={() => void handleStart()}
                disabled={isStarting || !prompt.trim()}
                className="de-button-primary mt-5 inline-flex h-10 items-center justify-center gap-2 px-4 text-sm font-medium"
              >
                {isStarting ? <Loader2 className="h-4 w-4 animate-spin" /> : <Search className="h-4 w-4" />}
                {isStarting ? '检索中' : '开始检索'}
              </button>
            </div>
          </div>
        </section>

        {!isStarting ? (
          <div className="mt-3 flex flex-wrap items-center gap-x-4 gap-y-2">
            <span className="text-xs text-[var(--de-ink-muted)]">可从这些问题开始</span>
            {researchStarters.map((starter) => (
              <button
                key={starter}
                type="button"
                onClick={() => setPrompt(starter)}
                className="border-b border-transparent text-left text-xs text-[var(--de-ink-muted)] transition-colors hover:border-[var(--de-rule-strong)] hover:text-[var(--de-ink)]"
              >
                {starter}
              </button>
            ))}
          </div>
        ) : null}

        {(isStarting || currentPhase === 'cancelled') && (
          <section className="mt-6 border border-[var(--de-rule)] bg-[var(--de-surface)] p-5">
            <div className="flex flex-wrap items-start justify-between gap-4">
              <div>
                <div className="flex items-center gap-2 text-sm font-semibold text-[var(--de-ink)]">
                  {currentPhase === 'cancelled' ? (
                    <Square className="h-4 w-4 text-[var(--de-danger)]" />
                  ) : (
                    <Loader2 className="h-4 w-4 animate-spin text-[var(--de-accent)]" />
                  )}
                  {phaseLabels[currentPhase] || '处理中'}
                </div>
                <p className="mt-1 text-xs leading-5 text-[var(--de-ink-muted)]">
                  {deepStartProgress?.message || '正在从 Semantic Scholar 与 arXiv 获取候选论文'}
                </p>
              </div>
              {isStarting ? (
                <button
                  type="button"
                  onClick={() => void handleCancel()}
                  disabled={isCancelling || !runningSessionId}
                  className="de-button-secondary inline-flex h-8 items-center gap-1.5 px-3 text-xs disabled:opacity-50"
                  title={runningSessionId ? '停止当前检索' : '后端任务建立后即可停止'}
                >
                  {isCancelling ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Square className="h-3 w-3" />}
                  停止
                </button>
              ) : null}
            </div>

            <div className="mt-4 h-1.5 overflow-hidden bg-[var(--de-surface-muted)]">
              <div
                className="h-full bg-[var(--de-accent)] transition-[width] duration-150"
                style={{ width: `${progressPercent}%` }}
              />
            </div>
            <div className="mt-2 flex justify-between text-xs text-[var(--de-ink-muted)]">
              <span>{progressPercent}%</span>
              {deepStartProgress?.estimatedRemainingSeconds ? (
                <span>预计约 {deepStartProgress.estimatedRemainingSeconds} 秒</span>
              ) : null}
            </div>

            {searchProgress?.sources?.length ? (
              <div className="mt-5 divide-y divide-[var(--de-rule)] border-y border-[var(--de-rule)]">
                {searchProgress.sources.map((source) => (
                  <div key={source.name} className="flex items-center gap-3 py-2.5 text-xs">
                    {source.success ? (
                      <Check className="h-4 w-4 text-[var(--de-accent)]" />
                    ) : (
                      <Clock3 className="h-4 w-4 text-[var(--de-ink-muted)]" />
                    )}
                    <span className="font-medium text-[var(--de-ink)]">{source.name}</span>
                    <span className="ml-auto text-[var(--de-ink-muted)]">
                      {source.success
                        ? `${source.resultCount} 篇`
                        : source.status === 'failed'
                          ? '本来源暂不可用'
                          : `第 ${Math.max(source.attempt, 1)} 次尝试`}
                    </span>
                  </div>
                ))}
              </div>
            ) : null}

            {deepStartProgress?.stats ? (
              <p className="mt-4 text-xs leading-5 text-[var(--de-ink-muted)]">
                已获取 {deepStartProgress.stats.rawCount} 条记录，去重后 {deepStartProgress.stats.dedupCount} 篇，
                当前纳入 {deepStartProgress.stats.finalCount} 篇。
              </p>
            ) : null}
          </section>
        )}

        <section className="mt-10">
          <div className="flex items-end justify-between border-b border-[var(--de-rule)] pb-3">
            <div>
              <h2 className="de-display text-xl font-semibold text-[var(--de-ink)]">最近研究</h2>
              <p className="mt-1 text-xs text-[var(--de-ink-muted)]">继续已有检索，或查看 AI 已整理的论文结构。</p>
            </div>
            {deepStartSessions.length > recentSessions.length ? (
              <button
                type="button"
                onClick={() => navigate('/history')}
                className="inline-flex items-center gap-1 text-xs font-medium text-[var(--de-accent)] hover:underline"
              >
                全部记录 <ArrowRight className="h-3.5 w-3.5" />
              </button>
            ) : null}
          </div>

          {recentSessions.length ? (
            <div className="divide-y divide-[var(--de-rule)]">
              {recentSessions.map((session) => (
                <button
                  key={session.id}
                  type="button"
                  onClick={() => navigate(`/session/${session.id}`)}
                  className="group grid w-full grid-cols-[minmax(0,1fr)_auto] items-center gap-6 px-1 py-4 text-left transition-colors hover:bg-[var(--de-surface-muted)]"
                >
                  <div className="min-w-0">
                    <h3 className="truncate text-sm font-semibold text-[var(--de-ink)]">
                      {session.title || session.rootPrompt}
                    </h3>
                    <p className="mt-1 line-clamp-1 text-xs text-[var(--de-ink-muted)]">
                      {session.currentQuery || session.rootPrompt}
                    </p>
                  </div>
                  <div className="flex items-center gap-4">
                    <span className="hidden text-xs text-[var(--de-ink-muted)] sm:block">
                      {formatUpdatedAt(session.updatedAt)}
                    </span>
                    <ArrowRight className="h-4 w-4 text-[var(--de-ink-muted)] transition-colors group-hover:text-[var(--de-accent)]" />
                  </div>
                </button>
              ))}
            </div>
          ) : (
            <div className="flex items-center gap-3 py-10 text-sm text-[var(--de-ink-muted)]">
              <FileSearch className="h-5 w-5" />
              完成第一次检索后，研究记录会出现在这里。
            </div>
          )}
        </section>
      </div>
    </div>
  );
}
