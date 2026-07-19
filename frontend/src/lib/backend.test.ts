import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  canResolveFilePaths,
  createScreeningSession,
  getInitialState,
  getSecretPrefill,
  getSyncRecords,
  getSyncStatus,
  onDeepStartProgress,
  onExtractProgress,
  onSearchProgress,
  onSyncProgress,
  resolveFilePaths,
  saveConfig,
} from './backend';
import { defaultConfig } from '../types';

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
    const unsubscribeSpy = vi.fn(() => listeners.delete('extract-progress'));

    (window as any).go = { main: { App: {} } };
    (window as any).runtime = {
      CanResolveFilePaths: vi.fn(() => true),
      ResolveFilePaths: vi.fn(() => ['/tmp/example.pdf']),
      EventsOnMultiple: vi.fn((eventName: string, callback: (...args: any[]) => void) => {
        listeners.set(eventName, callback);
        return unsubscribeSpy;
      }),
      EventsOff: vi.fn(),
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
    expect(unsubscribeSpy).toHaveBeenCalledTimes(1);
    expect(window.runtime?.EventsOff).not.toHaveBeenCalled();
  });

  it('subscribes to search progress events in Wails runtime', () => {
    const listeners = new Map<string, (...args: any[]) => void>();
    const unsubscribeSpy = vi.fn(() => listeners.delete('search-progress'));

    (window as any).go = { main: { App: {} } };
    (window as any).runtime = {
      CanResolveFilePaths: vi.fn(() => true),
      ResolveFilePaths: vi.fn(() => ['/tmp/example.pdf']),
      EventsOnMultiple: vi.fn((eventName: string, callback: (...args: any[]) => void) => {
        listeners.set(eventName, callback);
        return unsubscribeSpy;
      }),
      EventsOff: vi.fn(),
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
    expect(unsubscribeSpy).toHaveBeenCalledTimes(1);
    expect(window.runtime?.EventsOff).not.toHaveBeenCalled();
  });

  it('subscribes to deepstart progress events in Wails runtime', () => {
    const listeners = new Map<string, (...args: any[]) => void>();
    const unsubscribeSpy = vi.fn(() => listeners.delete('deepstart-progress'));

    (window as any).go = { main: { App: {} } };
    (window as any).runtime = {
      CanResolveFilePaths: vi.fn(() => true),
      ResolveFilePaths: vi.fn(() => []),
      EventsOnMultiple: vi.fn((eventName: string, callback: (...args: any[]) => void) => {
        listeners.set(eventName, callback);
        return unsubscribeSpy;
      }),
      EventsOff: vi.fn(),
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
      initialBatchTotal: 0,
      initialBatchCompleted: 0,
      backgroundCompleted: 0,
      stats: {
        query: 'embodied',
        originalQuery: '',
        rewrittenQueries: [],
        queryHits: {},
        rawCount: 220,
        dedupCount: 200,
        finalCount: 200,
      },
    });

    unsubscribe();
    expect(unsubscribeSpy).toHaveBeenCalledTimes(1);
    expect(window.runtime?.EventsOff).not.toHaveBeenCalled();
  });

  it('subscribes to sync progress events in Wails runtime', () => {
    const listeners = new Map<string, (...args: any[]) => void>();
    const unsubscribeSpy = vi.fn(() => listeners.delete('sync-progress'));

    (window as any).go = { main: { App: {} } };
    (window as any).runtime = {
      CanResolveFilePaths: vi.fn(() => true),
      ResolveFilePaths: vi.fn(() => []),
      EventsOnMultiple: vi.fn((eventName: string, callback: (...args: any[]) => void) => {
        listeners.set(eventName, callback);
        return unsubscribeSpy;
      }),
      EventsOff: vi.fn(),
    };

    const callback = vi.fn();
    const unsubscribe = onSyncProgress(callback);

    listeners.get('sync-progress')?.({
      total: 3,
      completed: 1,
      currentFile: 'data/diveend.db',
      status: 'uploading',
      message: '上传中',
    });

    expect(callback).toHaveBeenCalledWith({
      total: 3,
      completed: 1,
      currentFile: 'data/diveend.db',
      status: 'uploading',
      message: '上传中',
    });

    unsubscribe();
    expect(unsubscribeSpy).toHaveBeenCalledTimes(1);
    expect(window.runtime?.EventsOff).not.toHaveBeenCalled();
  });

  it('redacts secrets from runtime progress events and sync history errors', async () => {
    const listeners = new Map<string, (...args: any[]) => void>();
    const unsubscribeSpy = vi.fn(() => listeners.delete('sync-progress'));

    (window as any).go = {
      main: {
        App: {
          GetSyncRecords: vi.fn(async () => [
            {
              id: 'record-1',
              type: 'upload',
              fileName: 'paper.pdf',
              fileSize: 100,
              remotePath: '/apps/DiveEnd/paper.pdf',
              localPath: '/tmp/paper.pdf',
              status: 'failed',
              errorMessage: 'upload failed with api_key=sk-sync-history-secret and token=baidu-history-secret',
              createdAt: '2026-06-13T00:00:00Z',
            },
          ]),
        },
      },
    };
    (window as any).runtime = {
      CanResolveFilePaths: vi.fn(() => true),
      ResolveFilePaths: vi.fn(() => []),
      EventsOnMultiple: vi.fn((eventName: string, callback: (...args: any[]) => void) => {
        listeners.set(eventName, callback);
        return unsubscribeSpy;
      }),
      EventsOff: vi.fn(),
    };

    const callback = vi.fn();
    const unsubscribe = onSyncProgress(callback);

    listeners.get('sync-progress')?.({
      total: 1,
      completed: 0,
      currentFile: 'paper.pdf',
      status: 'uploading',
      message: 'upload failed Authorization: Bearer sk-sync-event-secret-12345',
    });
    const records = await getSyncRecords(1);

    expect(callback).toHaveBeenCalledWith({
      total: 1,
      completed: 0,
      currentFile: 'paper.pdf',
      status: 'uploading',
      message: 'upload failed Authorization: Bearer [redacted]',
    });
    expect(records[0].errorMessage).toContain('[redacted]');
    expect(records[0].errorMessage).not.toContain('sk-sync-history-secret');
    expect(records[0].errorMessage).not.toContain('baidu-history-secret');

    unsubscribe();
  });

  it('unsubscribes only the listener returned by Wails runtime', () => {
    const listeners: Array<(...args: any[]) => void> = [];
    const unsubscribeSpies: Array<ReturnType<typeof vi.fn>> = [];

    (window as any).go = { main: { App: {} } };
    (window as any).runtime = {
      CanResolveFilePaths: vi.fn(() => true),
      ResolveFilePaths: vi.fn(() => []),
      EventsOnMultiple: vi.fn((eventName: string, callback: (...args: any[]) => void) => {
        listeners.push(callback);
        const index = listeners.length - 1;
        const unsubscribe = vi.fn(() => {
          listeners[index] = () => undefined;
        });
        unsubscribeSpies.push(unsubscribe);
        return unsubscribe;
      }),
      EventsOff: vi.fn(() => {
        listeners.length = 0;
      }),
    };

    const first = vi.fn();
    const second = vi.fn();
    const unsubscribeFirst = onSyncProgress(first);
    const unsubscribeSecond = onSyncProgress(second);

    unsubscribeFirst();
    listeners.forEach((listener) => listener({ status: 'uploading', total: 1, completed: 1 }));

    expect(first).not.toHaveBeenCalled();
    expect(second).toHaveBeenCalledWith({
      total: 1,
      completed: 1,
      currentFile: '',
      status: 'uploading',
      message: '',
    });
    expect(unsubscribeSpies[0]).toHaveBeenCalledTimes(1);
    expect(unsubscribeSpies[1]).not.toHaveBeenCalled();
    expect(window.runtime?.EventsOff).not.toHaveBeenCalled();

    unsubscribeSecond();
  });

  it('does not fall back to mock APIs when a Wails App binding is partially missing', async () => {
    (window as any).go = { main: { App: {} } };
    (window as any).runtime = {
      CanResolveFilePaths: vi.fn(() => true),
      ResolveFilePaths: vi.fn(() => []),
      EventsOnMultiple: vi.fn(),
      EventsOff: vi.fn(),
    };

    await expect(getSyncStatus()).rejects.toThrow(/GetSyncStatus/);
    await expect(createScreeningSession('missing binding')).rejects.toThrow(/CreateScreeningSession/);
  });

  it('redacts Baidu refresh and client credentials in mock saveConfig responses', async () => {
    const result = await saveConfig({
      ...defaultConfig,
      baiduCloud: {
        ...defaultConfig.baiduCloud,
        token: 'access-token',
        refreshToken: 'refresh-token',
        clientId: 'client-id',
        clientSecret: 'client-secret',
      },
    });

    expect(result.config.baiduCloud.token).toBe('');
    expect(result.config.baiduCloud.refreshToken).toBe('');
    expect(result.config.baiduCloud.clientId).toBe('');
    expect(result.config.baiduCloud.clientSecret).toBe('');
    expect(result.config.baiduCloud.hasToken).toBe(true);
  });

  it('drops secret prefill values returned by runtime and keeps only presence flags', async () => {
    (window as any).go = {
      main: {
        App: {
          GetSecretPrefill: vi.fn(async () => ({
            strongLLMApiKey: 'strong-secret',
            hasStrongLLMApiKey: true,
            weakLLMApiKey: 'weak-secret',
            hasWeakLLMApiKey: true,
            baiduToken: 'baidu-secret',
            hasBaiduToken: true,
          })),
        },
      },
    };
    (window as any).runtime = {
      CanResolveFilePaths: vi.fn(() => true),
      ResolveFilePaths: vi.fn(() => []),
      EventsOnMultiple: vi.fn(),
      EventsOff: vi.fn(),
    };

    const prefill = await getSecretPrefill();

    expect(prefill.strongLLMApiKey).toBe('');
    expect(prefill.weakLLMApiKey).toBe('');
    expect(prefill.baiduToken).toBe('');
    expect(prefill.hasStrongLLMApiKey).toBe(true);
    expect(prefill.hasWeakLLMApiKey).toBe(true);
    expect(prefill.hasBaiduToken).toBe(true);
  });

  it('does not fetch local secret seed files in mock runtime mode', async () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch').mockRejectedValue(new Error('mock runtime should not fetch local secrets'));

    const initialState = await getInitialState();
    const prefill = await getSecretPrefill();

    expect(fetchSpy).not.toHaveBeenCalled();
    expect(initialState.config.llm.apiKey).toBe('');
    expect(initialState.config.weakLLM.apiKey).toBe('');
    expect(initialState.config.baiduCloud.token).toBe('');
    expect(prefill).toEqual({
      strongLLMApiKey: '',
      hasStrongLLMApiKey: false,
      weakLLMApiKey: '',
      hasWeakLLMApiKey: false,
      baiduToken: '',
      hasBaiduToken: false,
    });

    fetchSpy.mockRestore();
  });
});
