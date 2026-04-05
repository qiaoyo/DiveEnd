import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { AppLayout } from './AppLayout';

export function AppRouter() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<Navigate to="/deepstart" replace />} />
        <Route path="/deepstart" element={<AppLayout />} />
        <Route path="/deepread" element={<AppLayout />} />
        <Route path="/screening" element={<AppLayout />} />
        <Route path="/sync" element={<AppLayout />} />
        <Route path="*" element={<Navigate to="/deepstart" replace />} />
      </Routes>
    </BrowserRouter>
  );
}
