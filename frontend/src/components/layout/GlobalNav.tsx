import {
  BookOpen,
  Cloud,
  Compass,
  Filter,
  History,
  Moon,
  Settings,
  Sun,
  Loader2,
} from 'lucide-react';
import type { LucideIcon } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import { NavLink, useLocation, useNavigate } from 'react-router-dom';
import { saveConfig } from '../../lib/backend';
import { errorToUserMessage } from '../../lib/errors';
import { useAppStore } from '../../stores/appStore';

type NavItem = {
  to: string;
  label: string;
  icon: LucideIcon;
  end?: boolean;
};

const primaryItems: NavItem[] = [
  { to: '/discover', label: '发现', icon: Compass, end: true },
  { to: '/deepread', label: '阅读', icon: BookOpen },
  { to: '/screening', label: '分析', icon: Filter },
];

const utilityItems: NavItem[] = [
  { to: '/history', label: '研究记录', icon: History },
  { to: '/sync', label: '同步', icon: Cloud },
  { to: '/settings', label: '设置', icon: Settings },
];

function PrimaryLink({ item }: { item: NavItem }) {
  const Icon = item.icon;
  return (
    <NavLink
      to={item.to}
      end={item.end}
      className={({ isActive }) =>
        `flex h-10 items-center gap-2 border-b-2 px-3 text-sm font-medium transition-colors ${
          isActive
            ? 'border-[var(--de-accent)] text-[var(--de-ink)]'
            : 'border-transparent text-[var(--de-ink-muted)] hover:text-[var(--de-ink)]'
        }`
      }
    >
      <Icon className="h-4 w-4" aria-hidden="true" />
      <span>{item.label}</span>
    </NavLink>
  );
}

export function GlobalNav() {
  const location = useLocation();
  const { pathname } = location;
  const navigate = useNavigate();
  const { config, theme, setConfig, setError } = useAppStore();
  const [switchingTheme, setSwitchingTheme] = useState(false);
  const utilityPath = utilityItems.some((item) => item.to === pathname);
  const navigationState = location.state as { returnTo?: string } | null;
  const previousWorkspacePath = useRef(
    utilityPath ? navigationState?.returnTo || '/home' : pathname || '/home',
  );

  useEffect(() => {
    if (!utilityPath) {
      previousWorkspacePath.current = pathname || '/home';
    }
  }, [pathname, utilityPath]);

  const handleUtilityClick = (path: string) => {
    if (pathname === path) {
      navigate(previousWorkspacePath.current || '/home', { replace: true });
      return;
    }
    navigate(path, {
      state: { returnTo: utilityPath ? previousWorkspacePath.current : pathname || '/home' },
    });
  };

  const handleToggleTheme = async () => {
    if (switchingTheme) return;
    setSwitchingTheme(true);
    try {
      const result = await saveConfig({
        ...config,
        theme: theme === 'dark' ? 'light' : 'dark',
      });
      setConfig(result.config);
    } catch (error) {
      setError(errorToUserMessage(error, '切换主题失败'));
    } finally {
      setSwitchingTheme(false);
    }
  };

  return (
    <header className="z-40 h-14 shrink-0 border-b border-[var(--de-rule)] bg-[var(--de-surface)]">
      <div className="flex h-full items-center px-4">
        <NavLink to="/home" className="mr-7 flex min-w-0 items-center gap-2" aria-label="DiveEnd 首页">
          <span className="flex h-7 w-7 items-center justify-center rounded-[4px] bg-[var(--de-accent)] text-sm font-semibold text-white">
            D
          </span>
          <span className="de-display text-base font-semibold text-[var(--de-ink)]">DiveEnd</span>
        </NavLink>

        <nav className="flex h-full items-center" aria-label="主要功能">
          {primaryItems.map((item) => (
            <PrimaryLink key={item.to} item={item} />
          ))}
        </nav>

        <nav className="ml-auto flex items-center gap-1" aria-label="工作区工具">
          {utilityItems.map((item) => {
            const Icon = item.icon;
            const isActive = pathname === item.to;
            return (
              <button
                key={item.to}
                type="button"
                onClick={() => handleUtilityClick(item.to)}
                title={item.label}
                aria-label={item.label}
                aria-pressed={isActive}
                className={`flex h-9 w-9 items-center justify-center rounded-[5px] transition-colors ${
                  isActive
                    ? 'bg-[var(--de-accent-soft)] text-[var(--de-accent)]'
                    : 'text-[var(--de-ink-muted)] hover:bg-[var(--de-surface-muted)] hover:text-[var(--de-ink)]'
                }`}
              >
                <Icon className="h-[18px] w-[18px]" aria-hidden="true" />
              </button>
            );
          })}
          <span className="mx-2 h-5 w-px bg-[var(--de-rule-strong)]" aria-hidden="true" />
          <button
            type="button"
            onClick={() => void handleToggleTheme()}
            disabled={switchingTheme}
            title={`切换到${theme === 'dark' ? '浅色' : '深色'}模式`}
            aria-label={`切换到${theme === 'dark' ? '浅色' : '深色'}模式`}
            className="flex h-9 w-9 items-center justify-center rounded-[5px] text-[var(--de-ink-muted)] transition-colors hover:bg-[var(--de-surface-muted)] hover:text-[var(--de-ink)] disabled:opacity-60"
          >
            {switchingTheme ? (
              <Loader2 className="h-[17px] w-[17px] animate-spin" aria-hidden="true" />
            ) : theme === 'dark' ? (
              <Sun className="h-[17px] w-[17px]" aria-hidden="true" />
            ) : (
              <Moon className="h-[17px] w-[17px]" aria-hidden="true" />
            )}
          </button>
        </nav>
      </div>
    </header>
  );
}
