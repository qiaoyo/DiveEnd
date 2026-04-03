import { useState } from 'react';
import { BrowserRouter as Router, Routes, Route, Navigate } from 'react-router-dom';
import { Toaster } from 'sonner';
import './App.css';

// Pages
import { DeepRead } from './pages/DeepRead';

// Components
import { Button } from './components/ui/button';
import { BookOpen, Search, Settings, Library } from 'lucide-react';

// Placeholder components for other routes
const Home = () => (
  <div className="flex flex-col items-center justify-center h-full p-8">
    <div className="w-24 h-24 bg-primary/10 rounded-full flex items-center justify-center mb-6">
      <BookOpen className="w-12 h-12 text-primary" />
    </div>
    <h1 className="text-3xl font-bold mb-2">Welcome to DiveEnd</h1>
    <p className="text-muted-foreground text-center max-w-md mb-8">
      Your immersive paper collection and AI translation reading tool.
      Start by searching for papers or opening your library.
    </p>
    <div className="flex gap-4">
      <Button size="lg" className="gap-2">
        <Search className="w-4 h-4" />
        DeepStart
      </Button>
      <Button size="lg" variant="outline" className="gap-2">
        <Library className="w-4 h-4" />
        My Library
      </Button>
    </div>
  </div>
);

const DeepStart = () => (
  <div className="flex flex-col items-center justify-center h-full p-8">
    <h1 className="text-2xl font-bold mb-4">DeepStart - Coming Soon</h1>
    <p className="text-muted-foreground">Domain exploration and paper discovery</p>
  </div>
);

const Library = () => (
  <div className="flex flex-col items-center justify-center h-full p-8">
    <h1 className="text-2xl font-bold mb-4">Library - Coming Soon</h1>
    <p className="text-muted-foreground">Manage your paper collection</p>
  </div>
);

const SettingsPage = () => (
  <div className="flex flex-col items-center justify-center h-full p-8">
    <h1 className="text-2xl font-bold mb-4">Settings - Coming Soon</h1>
    <p className="text-muted-foreground">Configure your preferences</p>
  </div>
);

// Layout component with sidebar
const Layout: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [sidebarOpen, setSidebarOpen] = useState(true);

  return (
    <div className="flex h-screen bg-background">
      {/* Sidebar */}
      <aside
        className={`${sidebarOpen ? 'w-64' : 'w-16'}
          border-r bg-muted/30 flex flex-col transition-all duration-300`}
      >
        {/* Logo */}
        <div className="h-14 flex items-center px-4 border-b">
          <BookOpen className="w-6 h-6 text-primary" />
          {sidebarOpen && (
            <span className="ml-3 font-bold text-lg">DiveEnd</span>
          )}
        </div>

        {/* Navigation */}
        <nav className="flex-1 py-4 px-2 space-y-1">
          <NavItem
            to="/"
            icon={BookOpen}
            label="Home"
            sidebarOpen={sidebarOpen}
          />
          <NavItem
            to="/deepstart"
            icon={Search}
            label="DeepStart"
            sidebarOpen={sidebarOpen}
          />
          <NavItem
            to="/library"
            icon={Library}
            label="Library"
            sidebarOpen={sidebarOpen}
          />
          <NavItem
            to="/settings"
            icon={Settings}
            label="Settings"
            sidebarOpen={sidebarOpen}
          />
        </nav>

        {/* Toggle button */}
        <button
          onClick={() => setSidebarOpen(!sidebarOpen)}
          className="h-10 border-t flex items-center justify-center hover:bg-muted/50 transition-colors"
        >
          {sidebarOpen ? '←' : '→'}
        </button>
      </aside>

      {/* Main content */}
      <main className="flex-1 overflow-hidden">
        {children}
      </main>
    </div>
  );
};

// Nav item component
const NavItem: React.FC<{
  to: string;
  icon: React.ComponentType<{ className?: string }>;
  label: string;
  sidebarOpen: boolean;
}> = ({ to, icon: Icon, label, sidebarOpen }) => {
  const location = window.location;
  const isActive = location.pathname === to || location.pathname.startsWith(`${to}/`);

  return (
    <a
      href={to}
      className={`flex items-center px-3 py-2 rounded-md transition-colors ${
        isActive
          ? 'bg-primary text-primary-foreground'
          : 'text-muted-foreground hover:bg-muted hover:text-foreground'
      }`}
    >
      <Icon className="w-5 h-5 flex-shrink-0" />
      {sidebarOpen && <span className="ml-3">{label}</span>}
    </a>
  );
};

// Main App component
function App() {
  return (
    <Router>
      <Layout>
        <Routes>
          <Route path="/" element={<Home />} />
          <Route path="/deepstart" element={<DeepStart />} />
          <Route path="/library" element={<Library />} />
          <Route path="/settings" element={<SettingsPage />} />
          <Route path="/deepread/:paperId" element={<DeepRead />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </Layout>
      <Toaster position="top-right" />
    </Router>
  );
}

export default App;
