import { afterEach, describe, expect, it, vi } from 'vitest';
import { canResolveFilePaths, onExtractProgress, onSearchProgress, resolveFilePaths } from './backend';

describe('backend runtime helpers', () => {
  afterEach(() => {
    delete window.runtime;
    delete window.go;
  });

  it('returns false for file path resolution outside Wails runtime', () => {
    expect(canResolveFilePaths()).toBe(false);
    expect(resolveFilePaths([])).toEqual([]);
  });

  it('resolves file paths and subscribes to extract progress inside Wails runtime', () => {
    const listeners = new Map<string, (...args: any[]) => void>();
    const offSpy = vi.fn();

    (window as any).go = { main: { App: {} } };
    (window as any).runtime = {
      CanResolveFilePaths: vi.fn(() => true),
      ResolveFilePaths: vi.fn(() => ['/tmp/example.pdf']),
      EventsOnMultiple: vi.fn((eventName: string, callback: (...args: any[]) => void) => {
        listeners.set(eventName, callback);
        return vi.fn(() => listeners.delete(eventName));
      }),
      EventsOff: offSpy,
    };

    expect(canResolveFilePaths()).toBe(true);
    expect(resolveFilePaths([new File(['pdf'], 'example.pdf', { type: 'application/pdf' })])).toEqual([
      '/tmp/example.pdf',
    ]);

    const callback = vi.fn();
    const unsubscribe = onExtractProgress(callback);

    listeners.get('extract-progress')?.({
      sessionId: 'session-1',
      total: 2,
      completed: 1,
      currentFile: 'example.pdf',
      status: 'processing',
    });

    expect(callback).toHaveBeenCalledWith({
      sessionId: 'session-1',
      total: 2,
      completed: 1,
      currentFile: 'example.pdf',
      status: 'processing',
      errorMessage: '',
    });

    unsubscribe();
    expect(offSpy).toHaveBeenCalledWith('extract-progress');
  });

  it('subscribes to search progress events in Wails runtime', () => {
    const listeners = new Map<string, (...args: any[]) => void>();
    const offSpy = vi.fn();

    (window as any).go = { main: { App: {} } };
    (window as any).runtime = {
      CanResolveFilePaths: vi.fn(() => true),
      ResolveFilePaths: vi.fn(() => ['/tmp/example.pdf']),
      EventsOnMultiple: vi.fn((eventName: string, callback: (...args: any[]) => void) => {
        listeners.set(eventName, callback);
        return vi.fn(() => listeners.delete(eventName));
      }),
      EventsOff: offSpy,
    };

    const callback = vi.fn();
    const unsubscribe = onSearchProgress(callback);

    listeners.get('search-progress')?.({
      query: 'vla',
      elapsedSeconds: 10,
      totalSeconds: 60,
      completedSources: 1,
      totalSources: 2,
      sources: [
        {
          name: 'Semantic Scholar',
          attempt: 3,
          maxAttempts: 60,
          status: 'retrying',
          success: false,
          done: false,
          resultCount: 0,
          error: '429',
        },
      ],
      phase: 'searching',
    });

    expect(callback).toHaveBeenCalledWith({
      query: 'vla',
      elapsedSeconds: 10,
      totalSeconds: 60,
      completedSources: 1,
      totalSources: 2,
      sources: [
        {
          name: 'Semantic Scholar',
          attempt: 3,
          maxAttempts: 60,
          status: 'retrying',
          success: false,
          done: false,
          resultCount: 0,
          error: '429',
        },
      ],
      phase: 'searching',
      message: '',
    });

    unsubscribe();
    expect(offSpy).toHaveBeenCalledWith('search-progress');
  });
});
