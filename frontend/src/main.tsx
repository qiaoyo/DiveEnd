import { useEffect } from 'react';
import { AppRouter } from './components/layout/Router';
import { getInitialState } from './lib/backend';
import { useAppStore } from './stores/appStore';

function App() {
  const { theme, hydrate, setError, setHydrating } = useAppStore();

  useEffect(() => {
    let cancelled = false;

    const load = async () => {
      setHydrating(true);
      try {
        const initialState = await getInitialState();
        if (!cancelled) {
          hydrate(initialState);
        }
      } catch (error) {
        if (!cancelled) {
          setHydrating(false);
          setError(error instanceof Error ? error.message : '初始化失败');
        }
      }
    };

    void load();

    return () => {
      cancelled = true;
    };
  }, [hydrate, setError, setHydrating]);

  useEffect(() => {
    if (theme === 'dark') {
      document.documentElement.classList.add('dark');
    } else {
      document.documentElement.classList.remove('dark');
    }
  }, [theme]);

  return <AppRouter />;
}

export default App;
