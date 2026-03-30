import { useEffect } from 'react';
import { AppLayout } from './components/layout/AppLayout';
import { useAppStore } from './stores/appStore';

function App() {
  const { theme } = useAppStore();

  useEffect(() => {
    // Apply theme to document
    if (theme === 'dark') {
      document.documentElement.classList.add('dark');
    } else {
      document.documentElement.classList.remove('dark');
    }
  }, [theme]);

  return <AppLayout />;
}

export default App;