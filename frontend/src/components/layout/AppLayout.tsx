import { lazy, Suspense, useEffect } from 'react';
import { AlertCircle, Loader2, X } from 'lucide-react';
import { useLocation } from 'react-router-dom';
import { useAppStore } from '../../stores/appStore';
import { GlobalNav } from './GlobalNav';

const DeepStartPanel = lazy(() =>
  import('../deepstart/DeepStartPanel').then((module) => ({ default: module.DeepStartPanel })),
);
const SessionDetailPanel = lazy(() =>
  import('../deepstart/SessionDetailPanel').then((module) => ({ default: module.SessionDetailPanel })),
);
const HistoryPanel = lazy(() =>
  import('../history/HistoryPanel').then((module) => ({ default: module.HistoryPanel })),
);
const DeepReadPanel = lazy(() =>
  import('../deepread/DeepReadPanel').then((module) => ({ default: module.DeepReadPanel })),
);
const Screening = lazy(() =>
  import('../../pages/Screening').then((module) => ({ default: module.Screening })),
);
const Sync = lazy(() => import('../../pages/Sync').then((module) => ({ default: module.Sync })));
const SettingsPanel = lazy(() =>
  import('../settings/SettingsPanel').then((module) => ({ default: module.SettingsPanel })),
);

function WorkspaceLoading() {
  return (
    <div
      className="flex h-full items-center justify-center gap-2 text-sm text-[var(--de-ink-muted)]"
      role="status"
    >
      <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
      正在打开研究工作区
    </div>
  );
}

function WorkspaceContent({ pathname }: { pathname: string }) {
  if (pathname === '/' || pathname === '/deepstart') return <DeepStartPanel />;
  if (pathname === '/history') return <HistoryPanel />;
  if (pathname.startsWith('/session/')) return <SessionDetailPanel />;
  if (pathname === '/deepread') return <DeepReadPanel />;
  if (pathname === '/screening') return <Screening />;
  if (pathname === '/sync') return <Sync />;
  if (pathname === '/settings') {
    return (
      <div className="mx-auto h-full w-full max-w-4xl overflow-y-auto border-x border-[var(--de-rule)] bg-[var(--de-surface)]">
        <SettingsPanel />
      </div>
    );
  }
  return <DeepStartPanel />;
}

export function AppLayout() {
  const { pathname } = useLocation();
  const { error, isHydrating, setError } = useAppStore();

  useEffect(() => {
    setError(null);
  }, [pathname, setError]);

  return (
    <div className="de-shell flex h-full min-h-0 flex-col">
      <GlobalNav />

      {error ? (
        <div
          role="alert"
          className="flex shrink-0 items-start gap-2 border-b border-[var(--de-rule)] bg-[var(--de-surface-muted)] px-4 py-2 text-sm text-[var(--de-ink)]"
        >
          <AlertCircle className="mt-0.5 h-4 w-4 shrink-0 text-[var(--de-danger)]" aria-hidden="true" />
          <span className="min-w-0 flex-1">{error}</span>
          <button
            type="button"
            onClick={() => setError(null)}
            className="rounded-[4px] p-0.5 text-[var(--de-ink-muted)] hover:bg-[var(--de-rule)] hover:text-[var(--de-ink)]"
            aria-label="关闭错误提示"
            title="关闭"
          >
            <X className="h-4 w-4" />
          </button>
        </div>
      ) : null}

      <main className="min-h-0 flex-1">
        {isHydrating ? (
          <WorkspaceLoading />
        ) : (
          <Suspense fallback={<WorkspaceLoading />}>
            <WorkspaceContent pathname={pathname} />
          </Suspense>
        )}
      </main>
    </div>
  );
}
