import { HashRouter, Routes, Route, Navigate } from 'react-router-dom';
import { AppLayout } from './AppLayout';

export function AppRouter() {
  return (
    <HashRouter>
      <Routes>
        <Route path="/" element={<AppLayout />} />
        <Route path="/history" element={<AppLayout />} />
        <Route path="/deepstart" element={<AppLayout />} />
        <Route path="/session/:sessionId" element={<AppLayout />} />
        <Route path="/deepread" element={<AppLayout />} />
        <Route path="/screening" element={<AppLayout />} />
        <Route path="/sync" element={<AppLayout />} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </HashRouter>
  );
}
