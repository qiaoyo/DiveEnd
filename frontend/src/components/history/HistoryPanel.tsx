import { ArrowRight, FileSearch, Plus } from 'lucide-react';
import { formatDistanceToNow } from 'date-fns';
import { zhCN } from 'date-fns/locale';
import { useNavigate } from 'react-router-dom';
import { useAppStore } from '../../stores/appStore';

export function HistoryPanel() {
  const navigate = useNavigate();
  const { deepStartSessions } = useAppStore();

  return (
    <div className="h-full overflow-y-auto bg-[var(--de-paper)]">
      <div className="mx-auto w-full max-w-5xl px-6 py-8 lg:px-10 lg:py-10">
        <header className="flex flex-wrap items-end justify-between gap-4 border-b border-[var(--de-rule)] pb-5">
          <div>
            <p className="text-sm font-medium text-[var(--de-accent)]">研究记录</p>
            <h1 className="de-display mt-2 text-3xl font-semibold text-[var(--de-ink)]">已保存的检索</h1>
            <p className="mt-2 text-sm text-[var(--de-ink-muted)]">
              {deepStartSessions.length ? `共 ${deepStartSessions.length} 个研究工作区` : '还没有研究工作区'}
            </p>
          </div>
          <button
            type="button"
            onClick={() => navigate('/discover')}
            className="de-button-primary inline-flex h-9 items-center gap-2 px-3 text-sm font-medium"
          >
            <Plus className="h-4 w-4" />
            新建检索
          </button>
        </header>

        {deepStartSessions.length === 0 ? (
          <div className="flex min-h-[420px] flex-col items-center justify-center text-center">
            <FileSearch className="h-8 w-8 text-[var(--de-ink-muted)]" />
            <h2 className="de-display mt-5 text-xl font-semibold text-[var(--de-ink)]">从一个研究问题开始</h2>
            <p className="mt-2 max-w-md text-sm leading-6 text-[var(--de-ink-muted)]">
              检索完成后，论文分组、AI 分析与筛选状态会保存在这里。
            </p>
            <button
              type="button"
            onClick={() => navigate('/discover')}
              className="de-button-secondary mt-6 inline-flex h-9 items-center gap-2 px-4 text-sm"
            >
              前往发现
              <ArrowRight className="h-4 w-4" />
            </button>
          </div>
        ) : (
          <div className="divide-y divide-[var(--de-rule)]">
            {deepStartSessions.map((session) => (
              <button
                key={session.id}
                type="button"
                onClick={() => navigate(`/session/${session.id}`)}
                className="group grid w-full grid-cols-1 gap-3 px-1 py-5 text-left transition-colors hover:bg-[var(--de-surface-muted)] sm:grid-cols-[minmax(0,1fr)_180px_24px] sm:items-center"
              >
                <div className="min-w-0">
                  <h2 className="truncate text-base font-semibold text-[var(--de-ink)]">
                    {session.title || session.rootPrompt}
                  </h2>
                  <p className="mt-1 line-clamp-2 text-sm leading-6 text-[var(--de-ink-muted)]">
                    {session.rootPrompt}
                  </p>
                </div>
                <div className="text-xs text-[var(--de-ink-muted)]">
                  <p>{session.processingStatus === 'completed' ? '分析完成' : '处理中'}</p>
                  <p className="mt-1">
                    {formatDistanceToNow(new Date(session.updatedAt), { addSuffix: true, locale: zhCN })}
                  </p>
                </div>
                <ArrowRight className="hidden h-4 w-4 text-[var(--de-ink-muted)] transition-colors group-hover:text-[var(--de-accent)] sm:block" />
              </button>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
