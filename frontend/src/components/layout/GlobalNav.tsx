import { NavLink } from 'react-router-dom';

const navItems = [
  { to: '/', label: 'Home' },
  { to: '/history', label: '历史' },
  { to: '/deepstart', label: 'DeepStart' },
  { to: '/deepread', label: 'DeepRead' },
];

export function GlobalNav() {
  return (
    <div className="border-b border-slate-200/80 bg-white/80 px-4 py-2 backdrop-blur-md dark:border-slate-700/60 dark:bg-slate-900/75">
      <div className="mx-auto flex max-w-7xl items-center justify-between gap-3">
        <p className="text-xs font-semibold uppercase tracking-[0.18em] text-slate-500 dark:text-slate-300">
          DiveEnd
        </p>
        <nav className="flex items-center gap-1.5">
          {navItems.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              className={({ isActive }) =>
                `rounded-lg px-3 py-1.5 text-xs transition ${
                  isActive
                    ? 'bg-indigo-600 text-white'
                    : 'text-slate-600 hover:bg-slate-100 hover:text-indigo-700 dark:text-slate-200 dark:hover:bg-slate-800 dark:hover:text-indigo-300'
                }`
              }
            >
              {item.label}
            </NavLink>
          ))}
        </nav>
      </div>
    </div>
  );
}

