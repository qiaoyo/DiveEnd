import { useNavigate } from 'react-router-dom';
import { BookOpen, Compass, History } from 'lucide-react';
import { useAppStore } from '../../stores/appStore';

export function MainPanel() {
  const navigate = useNavigate();
  const { deepStartSessions } = useAppStore();

  const hasHistory = deepStartSessions.length > 0;

  return (
    <div className="flex h-full flex-col bg-slate-50 dark:bg-slate-950">
      <div className="flex flex-1 flex-col items-center justify-center px-8 py-10">
        <div className="w-full max-w-5xl">
          <div className="text-center">
            <p className="inline-flex rounded-full border border-indigo-200 bg-white/70 px-4 py-1 text-xs font-semibold uppercase tracking-[0.2em] text-indigo-600 dark:border-indigo-500/40 dark:bg-slate-900/70 dark:text-indigo-300">
              DiveEnd Workspace
            </p>
            <h1 className="mt-4 text-4xl font-semibold text-slate-900 dark:text-slate-100">AI-Driven Research</h1>
            <p className="mx-auto mt-3 max-w-2xl text-sm leading-7 text-slate-500 dark:text-slate-300">
              从探索、筛选到沉浸阅读，构建一条完整的论文工作流。
            </p>
          </div>

          <div className="mt-10 grid grid-cols-1 gap-5 md:grid-cols-3">
            <button
              onClick={() => navigate('/history')}
              disabled={!hasHistory}
              className="de-glass group rounded-2xl p-6 text-left transition hover:-translate-y-0.5 hover:shadow-xl disabled:cursor-not-allowed disabled:opacity-60"
            >
              <History className="h-7 w-7 text-indigo-500" />
              <h3 className="mt-4 text-lg font-semibold">探索历史</h3>
              <p className="mt-2 text-sm text-slate-500 dark:text-slate-300">
                {hasHistory ? `${deepStartSessions.length} 个会话记录` : '暂无历史记录'}
              </p>
            </button>

            <button
              onClick={() => navigate('/deepstart')}
              className="de-glass group rounded-2xl p-6 text-left transition hover:-translate-y-0.5 hover:shadow-xl"
            >
              <Compass className="h-7 w-7 text-violet-500" />
              <h3 className="mt-4 text-lg font-semibold">DeepStart</h3>
              <p className="mt-2 text-sm text-slate-500 dark:text-slate-300">开始新的论文探索</p>
            </button>

            <button
              onClick={() => navigate('/deepread')}
              className="de-glass group rounded-2xl p-6 text-left transition hover:-translate-y-0.5 hover:shadow-xl"
            >
              <BookOpen className="h-7 w-7 text-indigo-500" />
              <h3 className="mt-4 text-lg font-semibold">DeepRead</h3>
              <p className="mt-2 text-sm text-slate-500 dark:text-slate-300">沉浸式论文阅读</p>
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
