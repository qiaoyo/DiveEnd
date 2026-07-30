import {
  BookOpen,
  Cloud,
  Compass,
  Filter,
  History,
  Settings,
} from 'lucide-react';
import type { LucideIcon } from 'lucide-react';
import { NavLink } from 'react-router-dom';

type NavItem = {
  to: string;
  label: string;
  icon: LucideIcon;
  end?: boolean;
};

const primaryItems: NavItem[] = [
  { to: '/', label: '发现', icon: Compass, end: true },
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
  return (
    <header className="z-40 h-14 shrink-0 border-b border-[var(--de-rule)] bg-[var(--de-surface)]">
      <div className="flex h-full items-center px-4">
        <NavLink to="/" className="mr-7 flex min-w-0 items-center gap-2" aria-label="DiveEnd 发现">
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
            return (
              <NavLink
                key={item.to}
                to={item.to}
                title={item.label}
                aria-label={item.label}
                className={({ isActive }) =>
                  `flex h-9 w-9 items-center justify-center rounded-[5px] transition-colors ${
                    isActive
                      ? 'bg-[var(--de-accent-soft)] text-[var(--de-accent)]'
                      : 'text-[var(--de-ink-muted)] hover:bg-[var(--de-surface-muted)] hover:text-[var(--de-ink)]'
                  }`
                }
              >
                <Icon className="h-[18px] w-[18px]" aria-hidden="true" />
              </NavLink>
            );
          })}
        </nav>
      </div>
    </header>
  );
}
