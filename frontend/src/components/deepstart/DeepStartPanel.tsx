import { useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { BrainCircuit, Loader2, Search, Sparkles } from 'lucide-react';
import { onSearchProgress, startDeepStartSession } from '../../lib/backend';
import type { SearchProgressEvent } from '../../types';
import { useAppStore } from '../../stores/appStore';

export function DeepStartPanel() {
  const navigate = useNavigate();
  const { activeFolderId, folders, setActiveDeepStartSession, setError } = useAppStore();

  const [isStarting, setIsStarting] = useState(false);
  const [newPrompt, setNewPrompt] = useState('');
  const [searchProgress, setSearchProgress] = useState<SearchProgressEvent | null>(null);

  const examplePrompts = [
    '我想梳理这两年具身智能领域的论文',
    'LLM code agent 的代表性论文和 tech report',
    'VLA (Vision-Language-Action) 的最近进展',
  ];

  const previewNodes = [
    { name: 'Long Context', count: 23, depth: 0 },
    { name: 'VLA', count: 14, depth: 1 },
    { name: 'Skill Learning', count: 11, depth: 2 },
    { name: 'Sim2Real', count: 9, depth: 3 },
    { name: 'Benchmark', count: 17, depth: 1 },
  ];

  const previewCards = [
    { title: 'Transformers for Infinite Context Windows', venue: 'ICLR 2024', summary: '针对长上下文窗口的架构改进，强调记忆机制和推理稳定性。' },
    { title: 'VLA Survey: Foundations and Trends', venue: 'arXiv 2025', summary: '系统归纳视觉-语言-动作模型的训练范式与数据瓶颈。' },
    { title: 'Embodied Agents in Open-World Settings', venue: 'NeurIPS 2025', summary: '聚焦真实环境评测与安全约束，覆盖操作和导航任务。' },
  ];

  useEffect(() => {
    return onSearchProgress((progress) => {
      setSearchProgress(progress);
    });
  }, []);

  const progressPercent = useMemo(() => {
    if (!searchProgress) {
      return 0;
    }
    if (searchProgress.phase === 'completed') {
      return 100;
    }
    if (searchProgress.totalSeconds <= 0) {
      return 0;
    }
    return Math.min(100, Math.round((searchProgress.elapsedSeconds / searchProgress.totalSeconds) * 100));
  }, [searchProgress]);

  const handleStartSession = async () => {
    const prompt = newPrompt.trim();
    if (!prompt) {
      setError('请先输入研究方向或问题');
      return;
    }

    setIsStarting(true);
    setSearchProgress({
      query: prompt,
      elapsedSeconds: 0,
      totalSeconds: 60,
      completedSources: 0,
      totalSources: 2,
      sources: [
        {
          name: 'Semantic Scholar',
          attempt: 0,
          maxAttempts: 60,
          status: 'pending',
          success: false,
          done: false,
          resultCount: 0,
          error: '',
        },
        {
          name: 'arXiv',
          attempt: 0,
          maxAttempts: 60,
          status: 'pending',
          success: false,
          done: false,
          resultCount: 0,
          error: '',
        },
      ],
      phase: 'searching',
      message: '',
    });
    try {
      const detail = await startDeepStartSession(
        prompt,
        activeFolderId || folders[0]?.id || ''
      );
      setActiveDeepStartSession(detail);
      // 创建成功后跳转到会话详情页面
      navigate(`/session/${detail.summary.id}`);
    } catch (error) {
      setError(error instanceof Error ? error.message : '创建探索会话失败');
    } finally {
      setIsStarting(false);
    }
  };

  return (
    <div className="flex h-full min-h-0 flex-col overflow-hidden bg-slate-50 text-slate-900 dark:bg-slate-950 dark:text-slate-100">
      <div className="relative min-h-0 flex-1 overflow-y-auto px-6 pb-24 pt-8 lg:px-10">
        <div className="pointer-events-none absolute left-[-120px] top-[-160px] h-[340px] w-[340px] rounded-full bg-indigo-500/25 blur-3xl" />
        <div className="pointer-events-none absolute bottom-[-160px] right-[-120px] h-[340px] w-[340px] rounded-full bg-violet-500/20 blur-3xl" />

        <div className="relative mx-auto max-w-6xl">
          <div className="mb-8 text-center">
            <p className="inline-flex items-center gap-2 rounded-full border border-indigo-200 bg-white/70 px-4 py-1.5 text-xs font-semibold uppercase tracking-[0.22em] text-indigo-600 dark:border-indigo-500/30 dark:bg-slate-900/70 dark:text-indigo-300">
              <Sparkles className="h-3.5 w-3.5" />
              DeepStart Workspace
            </p>
            <h1 className="mt-5 text-3xl font-semibold leading-tight lg:text-4xl">
              让检索从一个问题，直接进入可执行的论文地图
            </h1>
            <p className="mx-auto mt-3 max-w-2xl text-sm leading-7 text-slate-600 dark:text-slate-300">
              用自然语言描述方向，DiveEnd 会自动建立分类支线、推荐阅读顺序，并把你选中的论文导入工作区。
            </p>
          </div>

          <div className="de-glass rounded-[28px] p-5 shadow-[0_24px_60px_-30px_rgba(15,23,42,0.45)]">
            <div className="flex flex-wrap items-center gap-3 rounded-2xl border border-white/70 bg-white/80 px-4 py-3 dark:border-slate-500/30 dark:bg-slate-900/75">
              <Search className="h-5 w-5 text-slate-400" />
              <input
                value={newPrompt}
                onChange={(event) => setNewPrompt(event.target.value)}
                onKeyDown={(event) => event.key === 'Enter' && !event.shiftKey && void handleStartSession()}
                placeholder="例如：寻找最近两年 VLA 的综述、基础模型与真实机器人评测论文"
                className="min-w-[200px] flex-1 border-none bg-transparent text-sm outline-none placeholder:text-slate-400"
              />
              <button
                type="button"
                onClick={() => void handleStartSession()}
                disabled={isStarting || !newPrompt.trim()}
                className="inline-flex items-center gap-2 rounded-xl bg-indigo-600 px-4 py-2 text-sm font-medium text-white transition hover:bg-indigo-500 disabled:cursor-not-allowed disabled:opacity-60"
              >
                {isStarting ? <Loader2 className="h-4 w-4 animate-spin" /> : <BrainCircuit className="h-4 w-4" />}
                {isStarting ? '正在创建探索会话...' : '开始探索'}
              </button>
              <span className="inline-flex h-9 w-9 items-center justify-center rounded-xl bg-violet-100 text-violet-600 dark:bg-violet-500/20 dark:text-violet-300">
                <Sparkles className="h-4 w-4" />
              </span>
            </div>

            <div className="mt-4 flex flex-wrap gap-2">
              {examplePrompts.map((example) => (
                <button
                  key={example}
                  onClick={() => setNewPrompt(example)}
                  className="rounded-full border border-slate-200 bg-white/70 px-3 py-1.5 text-xs text-slate-600 transition hover:border-indigo-300 hover:text-indigo-600 dark:border-slate-600/60 dark:bg-slate-900/70 dark:text-slate-300 dark:hover:border-indigo-400/60 dark:hover:text-indigo-300"
                >
                  {example}
                </button>
              ))}
            </div>

            {isStarting && searchProgress && (
              <div className="mt-4 rounded-2xl border border-indigo-200 bg-indigo-50/70 p-4 dark:border-indigo-500/35 dark:bg-indigo-500/10">
                <div className="flex items-center justify-between text-xs text-indigo-700 dark:text-indigo-200">
                  <span>正在搜索并重试（最多 1 分钟）</span>
                  <span>{searchProgress.elapsedSeconds}s / {searchProgress.totalSeconds}s</span>
                </div>
                <div className="mt-2 h-2 rounded-full bg-indigo-100 dark:bg-indigo-500/20">
                  <div className="h-2 rounded-full bg-indigo-600 transition-all" style={{ width: `${progressPercent}%` }} />
                </div>
                <div className="mt-3 grid gap-2 sm:grid-cols-2">
                  {searchProgress.sources.map((source) => (
                    <div key={source.name} className="rounded-xl border border-indigo-200 bg-white/70 px-3 py-2 text-xs dark:border-indigo-500/30 dark:bg-slate-900/60">
                      <div className="flex items-center justify-between">
                        <span className="font-semibold">{source.name}</span>
                        <span className={`rounded-full px-2 py-0.5 ${
                          source.status === 'success'
                            ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/20 dark:text-emerald-200'
                            : source.status === 'failed'
                              ? 'bg-rose-100 text-rose-700 dark:bg-rose-500/20 dark:text-rose-200'
                              : 'bg-indigo-100 text-indigo-700 dark:bg-indigo-500/20 dark:text-indigo-200'
                        }`}
                        >
                          {source.status}
                        </span>
                      </div>
                      <div className="mt-1 text-slate-600 dark:text-slate-300">
                        attempt {source.attempt}/{source.maxAttempts}
                        {source.success ? ` · ${source.resultCount} papers` : ''}
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>

          <div className="mt-7 grid gap-6 lg:grid-cols-[0.95fr_1.35fr]">
            <section className="de-glass rounded-[28px] p-5">
              <div className="flex items-center justify-between">
                <h2 className="text-sm font-semibold uppercase tracking-[0.2em] text-slate-500 dark:text-slate-300">Category Tree</h2>
                <span className="rounded-full bg-indigo-100 px-2.5 py-1 text-xs text-indigo-700 dark:bg-indigo-500/25 dark:text-indigo-200">Preview</span>
              </div>

              <div className="mt-4 space-y-3">
                {previewNodes.map((node, idx) => (
                  <div key={node.name} className="relative" style={{ marginLeft: `${node.depth * 18}px` }}>
                    {idx > 0 && <div className="absolute -left-4 top-0 h-3 w-3 border-b border-l border-indigo-300 dark:border-indigo-500/40" />}
                    <div className="flex items-center justify-between rounded-xl border border-slate-200 bg-white/80 px-3 py-2 text-sm dark:border-slate-700 dark:bg-slate-900/75">
                      <span>{node.name}</span>
                      <span className="rounded-full bg-violet-100 px-2 py-0.5 text-xs font-semibold text-violet-700 dark:bg-violet-500/20 dark:text-violet-300">
                        {node.count}
                      </span>
                    </div>
                  </div>
                ))}
              </div>
            </section>

            <section className="de-glass rounded-[28px] p-5">
              <h2 className="text-sm font-semibold uppercase tracking-[0.2em] text-slate-500 dark:text-slate-300">Main Contention</h2>
              <div className="mt-4 grid gap-3 sm:grid-cols-2">
                {previewCards.map((card) => (
                  <article key={card.title} className="group rounded-2xl border border-slate-200 bg-white/80 p-4 transition hover:-translate-y-0.5 hover:border-indigo-300 hover:shadow-lg dark:border-slate-700 dark:bg-slate-900/70 dark:hover:border-indigo-500/50">
                    <div className="text-sm font-semibold leading-6">{card.title}</div>
                    <div className="mt-1 text-xs uppercase tracking-[0.16em] text-slate-500 dark:text-slate-400">{card.venue}</div>
                    <p className="mt-3 text-sm leading-6 text-slate-600 dark:text-slate-300">{card.summary}</p>
                  </article>
                ))}
              </div>
            </section>
          </div>

          <div className="pointer-events-none fixed bottom-8 left-1/2 z-20 w-full max-w-[520px] -translate-x-1/2 px-6">
            <div className="pointer-events-auto de-glass flex items-center justify-between rounded-2xl px-4 py-3 shadow-xl">
              <div className="text-sm text-slate-600 dark:text-slate-300">
                <span className="font-semibold text-indigo-600 dark:text-indigo-300">{newPrompt.trim() ? '1' : '0'}</span> 条探索意图已准备
              </div>
              <button
                type="button"
                onClick={() => void handleStartSession()}
                disabled={isStarting || !newPrompt.trim()}
                className="inline-flex items-center gap-2 rounded-xl bg-violet-600 px-4 py-2 text-sm font-medium text-white transition hover:bg-violet-500 disabled:cursor-not-allowed disabled:opacity-60"
              >
                {isStarting ? <Loader2 className="h-4 w-4 animate-spin" /> : <Sparkles className="h-4 w-4" />}
                导入探索工作区
              </button>
            </div>
          </div>

          <div className="mt-6 text-center">
            <button
              onClick={() => navigate('/history')}
              className="text-sm text-slate-500 transition hover:text-indigo-600 dark:text-slate-400 dark:hover:text-indigo-300"
            >
              查看探索历史 →
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
