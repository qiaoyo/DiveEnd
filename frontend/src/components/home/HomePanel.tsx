import { useState } from 'react';
import {
  ArrowRight,
  BookOpen,
  Brain,
  Compass,
  History,
} from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { useAppStore } from '../../stores/appStore';

type WorkflowStep = {
  label: string;
  title: string;
  description: string;
  action: string;
  path: string;
  icon: typeof Compass;
  accent: string;
};

const workflowSteps: WorkflowStep[] = [
  {
    label: '发现',
    title: '从一个问题开始',
    description: '多源检索、合并版本，先看清研究地图。',
    action: '开始检索',
    path: '/discover',
    icon: Compass,
    accent: 'text-[var(--de-accent)]',
  },
  {
    label: '阅读',
    title: 'AI 辅助筛选阅读',
    description: '自然阅读，结构化解析关键信息。',
    action: '打开阅读',
    path: '/deepread',
    icon: BookOpen,
    accent: 'text-[var(--de-warning)]',
  },
  {
    label: '分析',
    title: '留下可复用判断',
    description: '筛选候选、整理结论，让下一次检索更快。',
    action: '进入分析',
    path: '/screening',
    icon: Brain,
    accent: 'text-[#496b8a] dark:text-[#8eb9dc]',
  },
];

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

export function HomePanel() {
  const navigate = useNavigate();
  const { deepStartSessions, folders, papers } = useAppStore();
  const [question, setQuestion] = useState('');
  const latestSession = deepStartSessions[0];

  const startResearch = () => {
    const initialPrompt = question.trim();
    if (initialPrompt) {
      navigate('/discover', { state: { initialPrompt } });
      return;
    }
    navigate('/discover');
  };

  return (
    <div className="h-full overflow-y-auto bg-[var(--de-paper)]">
      <div className="mx-auto w-full max-w-6xl px-6 py-8 lg:px-10 lg:py-10">
        <header className="border-b border-[var(--de-rule)] pb-7">
          <div className="flex flex-col justify-between gap-7 lg:flex-row lg:items-end">
            <div className="max-w-3xl">
              <p className="text-xs font-semibold text-[var(--de-accent)]">个人研究工作台</p>
              <h1 className="de-display mt-3 max-w-2xl text-4xl font-semibold leading-[1.08] text-[var(--de-ink)] sm:text-5xl">
                从问题到结论
              </h1>
              <p className="mt-4 max-w-2xl text-sm leading-6 text-[var(--de-ink-muted)]">
                把检索、阅读和分析放在同一个个人研究工作台里。
              </p>
            </div>

            <div className="grid grid-cols-2 gap-x-5 gap-y-3 border-l border-[var(--de-rule)] pl-4 text-xs text-[var(--de-ink-muted)] sm:grid-cols-4 lg:min-w-[430px]">
              <div>
                <p>论文</p>
                <p className="mt-1 text-lg font-semibold text-[var(--de-ink)]">{papers.length}</p>
                <p className="mt-0.5">已保存</p>
              </div>
              <div>
                <p>文件夹</p>
                <p className="mt-1 text-lg font-semibold text-[var(--de-ink)]">{folders.length}</p>
                <p className="mt-0.5">可用目录</p>
              </div>
              <div>
                <p>研究空间</p>
                <p className="mt-1 text-lg font-semibold text-[var(--de-ink)]">1</p>
                <p className="mt-0.5">当前工作台</p>
              </div>
              <div>
                <p>研究记录</p>
                <p className="mt-1 text-lg font-semibold text-[var(--de-ink)]">{deepStartSessions.length}</p>
                <p className="mt-0.5">已保存</p>
              </div>
            </div>
          </div>
        </header>

        <section className="grid border-b border-[var(--de-rule)] lg:grid-cols-[minmax(0,1fr)_300px]">
          <div className="py-7 lg:pr-10">
            <h2 className="de-display text-2xl font-semibold text-[var(--de-ink)]">写下你想弄清的事</h2>
            <div className="mt-5 flex flex-col gap-3 sm:flex-row sm:items-end">
              <label className="min-w-0 flex-1">
                <span className="sr-only">研究问题</span>
                <textarea
                  value={question}
                  onChange={(event) => setQuestion(event.target.value)}
                  onKeyDown={(event) => {
                    if ((event.metaKey || event.ctrlKey) && event.key === 'Enter') {
                      event.preventDefault();
                      startResearch();
                    }
                  }}
                  placeholder="例如：最近的视觉语言动作模型，在哪些真实机器人任务上更可靠？"
                  className="de-field min-h-[82px] w-full resize-none px-3 py-2.5 text-sm leading-6 placeholder:text-[var(--de-ink-muted)]"
                />
              </label>
              <button
                type="button"
                onClick={startResearch}
                className="de-button-primary inline-flex h-10 shrink-0 items-center justify-center gap-2 px-4 text-sm font-medium"
              >
                {question.trim() ? '带着问题去发现' : '打开发现'}
                <ArrowRight className="h-4 w-4" aria-hidden="true" />
              </button>
            </div>
          </div>

          <div className="border-t border-[var(--de-rule)] py-7 lg:border-l lg:border-t-0 lg:pl-7">
            <div className="flex items-center justify-between gap-4">
              <div className="flex items-center gap-2 text-xs font-semibold text-[var(--de-ink-muted)]">
                <History className="h-4 w-4" aria-hidden="true" />
                <span>继续最近研究</span>
              </div>
              {deepStartSessions.length > 3 ? (
                <button
                  type="button"
                  onClick={() => navigate('/history')}
                  className="inline-flex items-center gap-1 text-xs font-medium text-[var(--de-accent)] hover:underline"
                >
                  全部记录 <ArrowRight className="h-3.5 w-3.5" aria-hidden="true" />
                </button>
              ) : null}
            </div>
            {latestSession ? (
              <button
                type="button"
                onClick={() => navigate(`/session/${latestSession.id}`)}
                className="group mt-4 block w-full border-t border-[var(--de-rule)] pt-4 text-left"
              >
                <p className="line-clamp-2 text-sm font-semibold leading-6 text-[var(--de-ink)]">
                  {latestSession.title || latestSession.rootPrompt}
                </p>
                <div className="mt-3 flex items-center justify-between gap-3 text-xs text-[var(--de-ink-muted)]">
                  <span>{latestSession.processingStatus === 'completed' ? '分析完成' : '正在整理'}</span>
                  <span>{formatUpdatedAt(latestSession.updatedAt)}</span>
                </div>
                <span className="mt-4 inline-flex items-center gap-1 text-xs font-medium text-[var(--de-accent)]">
                  回到这项研究
                  <ArrowRight className="h-3.5 w-3.5 transition-transform group-hover:translate-x-0.5" aria-hidden="true" />
                </span>
              </button>
            ) : (
              <div className="mt-4 border-t border-[var(--de-rule)] pt-4">
                <p className="text-sm font-medium text-[var(--de-ink)]">还没有研究记录</p>
                <p className="mt-2 text-xs leading-5 text-[var(--de-ink-muted)]">
                  从左侧写下一个问题，第一次检索完成后，它会出现在这里。
                </p>
              </div>
            )}
          </div>
        </section>

        <section className="py-8">
          <p className="border-b border-[var(--de-rule)] pb-3 text-xs font-semibold text-[var(--de-ink-muted)]">跳转工作区</p>

          <div className="grid border-b border-[var(--de-rule)] md:grid-cols-3 md:divide-x md:divide-[var(--de-rule)]">
            {workflowSteps.map((step) => {
              const Icon = step.icon;
              return (
                <button
                  key={step.path}
                  type="button"
                  onClick={() => navigate(step.path)}
                  className="group border-t border-[var(--de-rule)] py-6 text-left transition-colors hover:bg-[var(--de-surface-muted)] md:border-t-0 md:px-6 md:first:pl-0 md:last:pr-0"
                >
                  <div className="flex items-center justify-between gap-4">
                    <Icon className={`h-5 w-5 ${step.accent}`} aria-hidden="true" />
                  </div>
                  <p className="mt-8 text-xs font-semibold text-[var(--de-ink-muted)]">{step.label}</p>
                  <h3 className="de-display mt-2 text-xl font-semibold text-[var(--de-ink)]">{step.title}</h3>
                  <p className="mt-2 min-h-[48px] text-sm leading-6 text-[var(--de-ink-muted)]">{step.description}</p>
                  <span className="mt-5 inline-flex items-center gap-1 text-xs font-medium text-[var(--de-ink)]">
                    {step.action}
                    <ArrowRight className="h-3.5 w-3.5 transition-transform group-hover:translate-x-0.5" aria-hidden="true" />
                  </span>
                </button>
              );
            })}
          </div>
        </section>
      </div>
    </div>
  );
}
