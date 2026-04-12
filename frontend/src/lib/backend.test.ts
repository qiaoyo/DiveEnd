import { afterEach, describe, expect, it, vi } from 'vitest';
import { canResolveFilePaths, onDeepStartProgress, onExtractProgress, onSearchProgress, resolveFilePaths } from './backend';

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

  it('subscribes to deepstart progress events in Wails runtime', () => {
    const listeners = new Map<string, (...args: any[]) => void>();
    const offSpy = vi.fn();

    (window as any).go = { main: { App: {} } };
    (window as any).runtime = {
      CanResolveFilePaths: vi.fn(() => true),
      ResolveFilePaths: vi.fn(() => []),
      EventsOnMultiple: vi.fn((eventName: string, callback: (...args: any[]) => void) => {
        listeners.set(eventName, callback);
        return vi.fn(() => listeners.delete(eventName));
      }),
      EventsOff: offSpy,
    };

    const callback = vi.fn();
    const unsubscribe = onDeepStartProgress(callback);

    listeners.get('deepstart-progress')?.({
      sessionId: 'deepstart-1',
      phase: 'enriching',
      message: '正在导入 200 篇论文',
      elapsedSeconds: 12,
      estimatedRemainingSeconds: 8,
      total: 200,
      completed: 40,
      overallPercent: 32,
      stats: {
        query: 'embodied',
        rawCount: 220,
        dedupCount: 200,
        finalCount: 200,
      },
    });

    expect(callback).toHaveBeenCalledWith({
      sessionId: 'deepstart-1',
      phase: 'enriching',
      message: '正在导入 200 篇论文',
      elapsedSeconds: 12,
      estimatedRemainingSeconds: 8,
      total: 200,
      completed: 40,
      overallPercent: 32,
      successCount: 0,
      failedCount: 0,
      noPdfUrlCount: 0,
      downloadedCount: 0,
      parsedCount: 0,
      extractedCount: 0,
      stats: {
        query: 'embodied',
        rawCount: 220,
        dedupCount: 200,
        finalCount: 200,
      },
    });

    unsubscribe();
    expect(offSpy).toHaveBeenCalledWith('deepstart-progress');
  });
});
