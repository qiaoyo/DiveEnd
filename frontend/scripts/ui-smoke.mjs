#!/usr/bin/env node
import { spawn } from 'node:child_process';
import { existsSync } from 'node:fs';
import { mkdir, mkdtemp, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import path from 'node:path';
import net from 'node:net';

const ROOT = path.resolve(new URL('..', import.meta.url).pathname);
const DEFAULT_VIEWPORT = { width: 1440, height: 1000, deviceScaleFactor: 1 };
const OUTPUT_ROOT = process.env.UI_SMOKE_OUTPUT || path.join(tmpdir(), `diveend-ui-smoke-${new Date().toISOString().replace(/[:.]/g, '-')}`);
const CHROME_PATHS = [
  process.env.UI_SMOKE_CHROME,
  '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
  '/Applications/Chromium.app/Contents/MacOS/Chromium',
  '/usr/bin/google-chrome-stable',
  '/usr/bin/google-chrome',
  '/usr/bin/chromium-browser',
  '/usr/bin/chromium',
].filter(Boolean);

if (typeof WebSocket !== 'function') {
  throw new Error('UI smoke requires Node with a native WebSocket client. Current project machine reports Node 24+ support.');
}

function log(message) {
  console.log(`[ui-smoke] ${message}`);
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function findChrome() {
  const chrome = CHROME_PATHS.find((candidate) => candidate && existsSync(candidate));
  if (!chrome) {
    throw new Error(`Chrome executable not found. Set UI_SMOKE_CHROME to override. Checked: ${CHROME_PATHS.join(', ')}`);
  }
  return chrome;
}

function getFreePort() {
  return new Promise((resolve, reject) => {
    const server = net.createServer();
    server.listen(0, '127.0.0.1', () => {
      const address = server.address();
      server.close(() => resolve(address.port));
    });
    server.on('error', reject);
  });
}

async function fetchJSON(url, timeoutMs = 1000) {
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), timeoutMs);
  try {
    const response = await fetch(url, { signal: controller.signal });
    if (!response.ok) {
      throw new Error(`${url} returned HTTP ${response.status}`);
    }
    return response.json();
  } finally {
    clearTimeout(timeout);
  }
}

async function waitForHTTP(url, timeoutMs = 30000) {
  const started = Date.now();
  let lastError;
  while (Date.now() - started < timeoutMs) {
    try {
      const response = await fetch(url, { method: 'GET' });
      if (response.ok) {
        return;
      }
      lastError = new Error(`${url} returned HTTP ${response.status}`);
    } catch (error) {
      lastError = error;
    }
    await sleep(250);
  }
  throw new Error(`Timed out waiting for ${url}: ${lastError?.message ?? 'unknown error'}`);
}

function startViteServer(port) {
  const child = spawn('npm', ['run', 'dev', '--', '--host', '127.0.0.1', '--port', String(port), '--strictPort'], {
    cwd: ROOT,
    stdio: ['ignore', 'pipe', 'pipe'],
    env: { ...process.env, BROWSER: 'none' },
  });
  child.stdout.on('data', (chunk) => process.stdout.write(`[vite] ${chunk}`));
  child.stderr.on('data', (chunk) => process.stderr.write(`[vite] ${chunk}`));
  return child;
}

class CDPClient {
  constructor(ws) {
    this.ws = ws;
    this.nextId = 1;
    this.pending = new Map();
    this.listeners = new Map();
    this.ws.addEventListener('message', (event) => this.handleMessage(event));
  }

  static async connect(wsURL) {
    const ws = new WebSocket(wsURL);
    await new Promise((resolve, reject) => {
      ws.addEventListener('open', resolve, { once: true });
      ws.addEventListener('error', reject, { once: true });
    });
    return new CDPClient(ws);
  }

  handleMessage(event) {
    const payload = JSON.parse(event.data);
    if (payload.id) {
      const entry = this.pending.get(payload.id);
      if (!entry) return;
      this.pending.delete(payload.id);
      if (payload.error) {
        entry.reject(new Error(payload.error.message || JSON.stringify(payload.error)));
      } else {
        entry.resolve(payload.result ?? {});
      }
      return;
    }

    const listeners = this.listeners.get(payload.method) ?? [];
    for (const listener of [...listeners]) {
      listener(payload);
    }
  }

  send(method, params = {}, sessionId = undefined) {
    const id = this.nextId++;
    const payload = { id, method, params };
    if (sessionId) payload.sessionId = sessionId;
    return new Promise((resolve, reject) => {
      this.pending.set(id, { resolve, reject });
      this.ws.send(JSON.stringify(payload));
    });
  }

  waitEvent(method, predicate = () => true, timeoutMs = 10000) {
    return new Promise((resolve, reject) => {
      const timeout = setTimeout(() => {
        cleanup();
        reject(new Error(`Timed out waiting for CDP event ${method}`));
      }, timeoutMs);
      const listener = (payload) => {
        if (!predicate(payload)) return;
        cleanup();
        resolve(payload);
      };
      const cleanup = () => {
        clearTimeout(timeout);
        const listeners = this.listeners.get(method) ?? [];
        this.listeners.set(method, listeners.filter((item) => item !== listener));
      };
      this.listeners.set(method, [...(this.listeners.get(method) ?? []), listener]);
    });
  }

  close() {
    this.ws.close();
  }
}

function smokeBridgeSource() {
  return `(() => {
    const now = () => new Date().toISOString();
    const listeners = new Map();
    const folders = [
      { id: 'folder-1', name: 'Cache', parentId: '', path: 'Cache', isSystem: true, createdAt: now(), updatedAt: now() },
      { id: 'folder-2', name: 'Subtopic', parentId: 'folder-1', path: 'Cache/Subtopic', isSystem: false, createdAt: now(), updatedAt: now() },
      { id: 'folder-3', name: 'Archive', parentId: '', path: 'Archive', isSystem: false, createdAt: now(), updatedAt: now() },
    ];
    let papers = [
      { id: 'paper-1', sourcePaperId: 'paper-1', title: 'Embodied Agent Study', authors: 'Alice, Bob', abstract: 'A smoke-test paper for DeepRead and library flows.', year: 2025, journal: 'ICRA', url: 'https://example.org/paper-1', pdfPath: '', downloadStatus: 'queued', downloadError: '', folderId: 'folder-1', category: 'robotics', tags: ['robotics'], addedAt: now(), updatedAt: now() },
      { id: 'paper-2', sourcePaperId: 'paper-2', title: 'Child Folder Paper', authors: 'Carol', abstract: 'A failed-download paper used to test retry UI.', year: 2024, journal: 'RSS', url: 'https://example.org/paper-2', pdfPath: '', downloadStatus: 'failed', downloadError: 'no downloadable pdf url', folderId: 'folder-2', category: 'benchmark', tags: ['benchmark'], addedAt: now(), updatedAt: now() },
    ];
    let syncSettings = { autoSync: false, syncOnStartup: false, syncBeforeExit: false, syncInterval: 30, conflictResolution: 'manual' };
    let syncProgress = { total: 0, completed: 0, currentFile: '', status: 'idle', message: '' };
    let screeningDetail = null;
    const folderTree = () => [{ folder: folders[0], children: [{ folder: folders[1], children: [] }] }, { folder: folders[2], children: [] }];
    const deepStartDetail = (prompt = 'VLA benchmark') => ({
      summary: { id: 'session-smoke', title: prompt, rootPrompt: prompt, currentQuery: prompt, targetFolderId: 'folder-1', processingStatus: 'completed', initialReadyCount: 2, totalPlannedCount: 2, backgroundRemaining: 0, createdAt: now(), updatedAt: now() },
      messages: [
        { id: 'msg-1', sessionId: 'session-smoke', role: 'user', content: prompt, createdAt: now() },
        { id: 'msg-2', sessionId: 'session-smoke', role: 'assistant', content: '我已经按 survey、benchmark 与真实机器人评测整理好了候选论文。', createdAt: now() },
      ],
      currentResults: [
        { id: 'search-1', title: 'Unified Embodied Agent Benchmark', authors: 'Alice, Bob', abstract: 'This embodied benchmark studies robot policy transfer in realistic scenarios.', year: 2025, journal: 'arXiv', publicationVenue: 'NeurIPS', publicationYear: 2025, citationCount: 156, url: 'https://example.org/search-1', category: 'benchmark', tags: ['robotics'], source: 'arxiv', institutions: ['CMU', 'OpenAI'], keywords: ['embodied', 'benchmark', 'robot policy'], sourceLabel: 'arXiv' },
        { id: 'search-2', title: 'Vision-Language-Action Survey', authors: 'Dana', abstract: 'A survey of VLA model families and evaluation patterns.', year: 2026, journal: 'arXiv', publicationVenue: 'arXiv', publicationYear: 2026, citationCount: 28, url: 'https://example.org/search-2', category: 'survey', tags: ['survey'], source: 'semantic_scholar', institutions: ['Stanford'], keywords: ['vla', 'survey'], sourceLabel: 'Semantic Scholar' },
      ],
      currentAnalysis: {
        overview: '这是 UI smoke 自动化生成的探索概览，用来检查 DeepStart 流程和卡片交互。',
        directions: [{ id: 'd-1', name: 'Benchmark', summary: '围绕具身智能评测', why: '先看 benchmark 更容易对齐任务边界。', paperIds: ['search-1'] }],
        paperNotes: [{ paperId: 'search-1', tier: 'core', reason: 'embodied benchmark 代表作', directionIds: ['d-1'] }],
        followUpQuestions: ['先保留 survey / overview 论文', '我更关心 benchmark 和评测范式'],
        suggestedQueries: ['embodied benchmark', 'VLA recent progress'],
        recommendedPaperIds: ['search-1'],
        retainedPaperIds: ['search-1'],
        searchStats: { query: prompt, rawCount: 2, dedupCount: 2, finalCount: 2 },
      },
      selectedPaperIds: [],
    });
    const emit = (eventName, payload) => {
      const callbacks = listeners.get(eventName) || [];
      callbacks.forEach((callback) => callback(payload));
    };
    window.__diveendSmoke = {
      emit,
      calls: [],
      getState: () => ({ syncSettings, syncProgress, papers, screeningDetail }),
    };
    window.runtime = {
      EventsOnMultiple(eventName, callback) {
        const callbacks = listeners.get(eventName) || [];
        callbacks.push(callback);
        listeners.set(eventName, callbacks);
        return () => listeners.set(eventName, (listeners.get(eventName) || []).filter((item) => item !== callback));
      },
      EventsOn(eventName, callback) { return this.EventsOnMultiple(eventName, callback, -1); },
      EventsOff(eventName) { listeners.delete(eventName); },
      EventsOffAll() { listeners.clear(); },
      EventsEmit(eventName, payload) { emit(eventName, payload); },
      CanResolveFilePaths() { return true; },
      ResolveFilePaths(files) { return Array.from(files || []).map((file) => '/mock/' + file.name); },
      BrowserOpenURL() {},
    };
    window.go = { main: { App: {
      async GetInitialState() { const activeSession = deepStartDetail('scientific reading assistant'); return { config: {}, folders, papers: papers.filter((paper) => paper.folderId === 'folder-1'), activeFolderId: 'folder-1', deepStartSessions: [activeSession.summary], activeDeepStartSession: activeSession }; }, 
      async GetSecretPrefill() { return { strongLLMApiKey: '', hasStrongLLMApiKey: false, weakLLMApiKey: '', hasWeakLLMAPIKey: false, baiduToken: '', hasBaiduToken: true }; },
      async SaveConfig(config) { return { config, restartRequired: false, message: '配置已保存' }; },
      async GetPDFServiceStatus() { return { enabled: true, url: 'http://127.0.0.1:50051', healthy: true, ready: true, checks: { pdf_parser: 'up' }, checkedAt: now(), message: 'PDF 服务 ready' }; },
      async StartDeepStartSession(prompt) {
        emit('deepstart-progress', { sessionId: 'session-smoke', phase: 'searching', message: '正在检索论文候选', elapsedSeconds: 1, estimatedRemainingSeconds: 2, total: 4, completed: 1, overallPercent: 35 });
        await new Promise((resolve) => setTimeout(resolve, 350));
        emit('deepstart-progress', { sessionId: 'session-smoke', phase: 'completed', message: '探索完成', elapsedSeconds: 2, estimatedRemainingSeconds: 0, total: 4, completed: 4, overallPercent: 100 });
        return deepStartDetail(prompt);
      },
      async CancelDeepStartTask() {},
      async ReplyDeepStartSession(sessionId, message) { const detail = deepStartDetail('VLA benchmark'); detail.messages.push({ id: 'msg-reply', sessionId, role: 'user', content: message, createdAt: now() }); detail.messages.push({ id: 'msg-reply-ai', sessionId, role: 'assistant', content: '已按你的偏好重新收束候选。', createdAt: now() }); return detail; },
      async RerunDeepStartSearch(sessionId, query) { return deepStartDetail(query); },
      async UndoDeepStartNarrow() { return deepStartDetail('VLA benchmark'); },
      async UpdateDeepStartSelections(sessionId, selectedPaperIds, targetFolderId) { const detail = deepStartDetail('VLA benchmark'); detail.selectedPaperIds = selectedPaperIds || []; detail.summary.targetFolderId = targetFolderId || 'folder-1'; return detail; },
      async ImportPapersWithAssets(folderId, selectedPapers) { const imported = (selectedPapers || []).map((paper, index) => ({ id: 'imported-' + index, sourcePaperId: paper.id, title: paper.title, authors: paper.authors, abstract: paper.abstract, year: paper.year || 2025, journal: paper.journal || '', url: paper.url || '', pdfPath: '', downloadStatus: 'queued', downloadError: '', folderId: folderId || 'folder-1', category: paper.category || '', tags: paper.tags || [], addedAt: now(), updatedAt: now() })); papers = [...imported, ...papers]; return { imported, skipped: [], queued: imported.length, message: '已导入并加入下载队列' }; },
      async GetFolders() { return folders; },
      async GetFolderTree() { return folderTree(); },
      async CreateFolder(name) { const folder = { id: 'folder-' + Date.now(), name, parentId: '', path: name, isSystem: false, createdAt: now(), updatedAt: now() }; folders.push(folder); return folder; },
      async CreateFolderNode(request) { const name = String(request?.path || request?.name || 'New Folder').split('/').pop(); const folder = { id: 'folder-' + Date.now(), name, parentId: request?.parentId || '', path: request?.path || name, isSystem: false, createdAt: now(), updatedAt: now() }; folders.push(folder); return folder; },
      async RenameFolderNode(request) { return folders.find((folder) => folder.id === request.folderId) || folders[0]; },
      async MoveFolderNode(request) { return folders.find((folder) => folder.id === request.folderId) || folders[0]; },
      async DeleteFolderNode() {},
      async GetPapers(folderId) { return papers.filter((paper) => !folderId || paper.folderId === folderId); },
      async MovePaperToFolder(paperId, targetFolderId) { papers = papers.map((paper) => paper.id === paperId ? { ...paper, folderId: targetFolderId, updatedAt: now() } : paper); return papers.find((paper) => paper.id === paperId); },
      async MovePapersToFolder(paperIds, targetFolderId) { papers = papers.map((paper) => (paperIds || []).includes(paper.id) ? { ...paper, folderId: targetFolderId, updatedAt: now() } : paper); return papers.filter((paper) => (paperIds || []).includes(paper.id)); },
      async DeletePaper(id) { papers = papers.filter((paper) => paper.id !== id); },
      async RetryPaperDownload() { return undefined; },
      async RetryPaperDownloadWithURL(paperId) { papers = papers.map((paper) => paper.id === paperId ? { ...paper, downloadStatus: 'queued', downloadError: '' } : paper); },
      async RetryFolderPendingDownloads() { return 1; },
      async SelectAndAttachPaperPDF(paperId) { const updated = papers.find((paper) => paper.id === paperId) || papers[0]; return { ...updated, pdfPath: '/mock/attached.pdf', downloadStatus: 'downloaded', downloadError: '' }; },
      async GetFolderStorageTreeOverview() { return { rootPath: '/mock/papers', directories: [{ folderId: 'folder-1', folderName: 'Cache', folderPath: '/mock/papers/Cache', paperCount: 1, queued: 1, downloading: 0, downloaded: 0, failed: 0, children: [] }], generatedAt: now() }; },
      async GetDeepReadState(paperId) { return { paperId, hasPdf: false, pdfPath: '', parseStatus: 'missing_pdf', parseError: '本地 PDF 不可用，请先等待下载完成或手动导入 PDF。', sections: [{ id: 'abstract', title: 'Abstract', content: 'Smoke abstract content.', index: 0 }], markdown: '# Abstract\\n\\nSmoke abstract content.', translations: [], notes: [], lastPreparedAt: now() }; },
      async PrepareDeepReadPaper(paperId) { return { paperId, hasPdf: false, pdfPath: '', parseStatus: 'parsed', parseError: '', sections: [{ id: 'abstract', title: 'Abstract', content: 'Smoke abstract content.', index: 0 }], markdown: '# Abstract\\n\\nSmoke abstract content.', translations: [], notes: [], lastPreparedAt: now() }; },
      async SaveDeepReadNote(paperId, section, content) { return { id: 'note-' + Date.now(), paperId, section, content, createdAt: now(), updatedAt: now() }; },
      async TranslatePaperSection(paperId, section, text) { return { id: 'translation-' + Date.now(), paperId, section, originalText: text, translatedText: '这是自动化 smoke 的翻译结果。', summary: '这是摘要。', createdAt: now(), updatedAt: now() }; },
      async GetTranslations() { return []; },
      async GetDeepReadPDFURL() { return ''; },
      async GetDeepReadPDFBytes() { return ''; },
      async SelectScreeningPDFs() { return ['/mock/survey.pdf', '/mock/benchmark.pdf']; },
      async CreateScreeningSession(title) { return { id: 'screening-smoke', title, status: 'upload', totalPapers: 0, createdAt: now(), updatedAt: now() }; },
      async UploadScreeningFiles(sessionId, filePaths) { screeningDetail = { session: { id: sessionId, title: 'Screening Smoke', status: 'extract', totalPapers: filePaths.length, createdAt: now(), updatedAt: now() }, papers: filePaths.map((filePath, index) => ({ id: 'screen-paper-' + index, sessionId, fileName: filePath.split('/').pop(), filePath, fileSize: 1024, status: 'pending', title: index === 0 ? 'Survey Candidate' : 'Benchmark Candidate', authors: 'Smoke Author', abstract: 'Smoke screening abstract', createdAt: now(), updatedAt: now() })), currentNode: null, pathHistory: [] }; return screeningDetail; },
      async ExtractPaperContent(sessionId) { emit('extract-progress', { sessionId, total: 2, completed: 1, currentFile: 'survey.pdf', status: 'processing', errorMessage: '' }); await new Promise((resolve) => setTimeout(resolve, 300)); emit('extract-progress', { sessionId, total: 2, completed: 2, currentFile: '', status: 'completed', errorMessage: '' }); return { sessionId, total: 2, completed: 2, currentFile: '', status: 'completed', errorMessage: '' }; },
      async GetExtractProgress(sessionId) { return { sessionId, total: 2, completed: 2, currentFile: '', status: 'completed', errorMessage: '' }; },
      async AnalyzePapers() { return { id: 'node-1', nodeType: 'branch', message: '你最想先保留哪一类论文？', dimension: '研究方向', options: [{ key: 'survey', label: '综述与综览', paperIds: ['screen-paper-0'], count: 1 }, { key: 'benchmark', label: '基准与实验', paperIds: ['screen-paper-1'], count: 1 }], allowMultiSelect: true, allowSkip: false, remainingPaperIds: ['screen-paper-0', 'screen-paper-1'] }; },
      async ApplyScreeningChoice() { if (screeningDetail) screeningDetail.pathHistory = [{ dimension: '研究方向', choice: '综述与综览' }]; return { id: 'node-complete', nodeType: 'complete', message: '筛选完成，请确认导入剩余论文。', dimension: '结果确认', options: [], allowMultiSelect: false, allowSkip: false, remainingPaperIds: ['screen-paper-0'] }; },
      async CompleteScreening() { const imported = [{ id: 'screen-import-1', sourcePaperId: 'screen-paper-0', title: 'Survey Candidate', authors: 'Smoke Author', abstract: 'Smoke screening abstract', year: 2026, journal: '', url: '', pdfPath: '/mock/survey.pdf', downloadStatus: 'downloaded', downloadError: '', folderId: 'folder-1', category: 'survey', tags: ['survey'], addedAt: now(), updatedAt: now() }]; papers = [...imported, ...papers]; return imported; },
      async ListScreeningSessions() { return []; },
      async GetScreeningSession(sessionId) { return screeningDetail || { session: { id: sessionId, title: 'Screening Smoke', status: 'screen', totalPapers: 2, createdAt: now(), updatedAt: now() }, papers: [], currentNode: null, pathHistory: [] }; },
      async CancelScreening() {},
      async GetSyncStatus() { return { enabled: true, provider: 'baidu_cloud', lastSync: null, syncInProgress: syncProgress.status === 'uploading', pendingFiles: 2, conflicts: 1, totalSynced: 3, totalFailed: 0 }; },
      async GetSyncPreview() { return { enabled: true, dataPath: '/mock/DiveEndData', remoteRoot: '/apps/pcstest_oauth/diveend-v1', tokenFile: 'baiduyun_token.json', totalFiles: 3, totalBytes: 4096, databaseBytes: 2048, paperPdfCount: 1, paperPdfBytes: 1024, otherFiles: 1, files: [{ key: 'data/diveend.db', fileName: 'diveend.db', kind: 'database', size: 2048, remotePath: '/apps/pcstest_oauth/diveend-v1/data/diveend.db' }, { key: 'papers/smoke/paper.pdf', fileName: 'paper.pdf', kind: 'paper_pdf', size: 1024, remotePath: '/apps/pcstest_oauth/diveend-v1/papers/smoke/paper.pdf' }], checkedAt: now(), warning: '' }; },
      async RefreshBaiduToken() { return { enabled: true, tokenFile: 'baiduyun_token.json', hasAccessToken: true, hasRefreshToken: true, hasClientId: true, hasClientSecret: true, refreshed: true, checkedAt: now(), message: '百度 access token 已通过 refresh token 更新' }; },
      async TriggerSync() { syncProgress = { total: 3, completed: 1, currentFile: 'data/diveend.db', status: 'uploading', message: '上传数据库快照' }; emit('sync-progress', syncProgress); return syncProgress; },
      async GetSyncProgress() { return syncProgress; },
      async GetSyncConflicts() { return [{ id: 'conflict-1', fileName: 'diveend.db', fileKind: 'database', localPath: '/mock/diveend.db', localSize: 2048, localTime: now(), remotePath: '/apps/DiveEnd/data/diveend.db', remoteSize: 4096, remoteTime: now(), newerSide: 'remote', resolution: '', createdAt: now() }]; },
      async GetSyncRecords() { return [{ id: 'record-1', type: 'upload', fileName: 'diveend.db', fileSize: 2048, remotePath: '/apps/DiveEnd/data/diveend.db', localPath: '/mock/diveend.db', status: 'success', message: 'Uploaded database snapshot', createdAt: now(), completedAt: now() }]; },
      async ResolveSyncConflict() {},
      async GetSyncSettings() { return syncSettings; },
      async SaveSyncSettings(settings) { syncSettings = { ...syncSettings, ...settings }; return syncSettings; },
      async GetPendingDatabaseRestore() { return { pending: false, applied: false, stagedPath: '', backupPath: '', remotePath: '', message: '' }; },
      async ApplyPendingDatabaseRestore() { return { pending: false, applied: true, stagedPath: '', backupPath: '', remotePath: '', message: '已应用' }; },
      async CancelPendingDatabaseRestore() {},
    } } };
  })();`;
}

async function launchChrome() {
  const chromePath = findChrome();
  const debugPort = await getFreePort();
  const userDataDir = await mkdtemp(path.join(tmpdir(), 'diveend-ui-smoke-chrome-'));
  const chrome = spawn(chromePath, [
    '--headless=new',
    '--disable-gpu',
    '--disable-background-networking',
    '--disable-component-update',
    '--disable-crash-reporter',
    '--disable-breakpad',
    '--disable-sync',
    '--disable-extensions',
    '--disable-default-apps',
    '--no-first-run',
    '--no-default-browser-check',
    `--remote-debugging-port=${debugPort}`,
    `--user-data-dir=${userDataDir}`,
    'about:blank',
  ], { stdio: ['ignore', 'pipe', 'pipe'] });

  chrome.stdout.on('data', (chunk) => process.stdout.write(`[chrome] ${chunk}`));
  chrome.stderr.on('data', (chunk) => process.stderr.write(`[chrome] ${chunk}`));

  const started = Date.now();
  let version;
  while (Date.now() - started < 30000) {
    try {
      version = await fetchJSON(`http://127.0.0.1:${debugPort}/json/version`);
      break;
    } catch {
      await sleep(200);
    }
  }
  if (!version?.webSocketDebuggerUrl) {
    throw new Error('Timed out waiting for Chrome DevTools endpoint');
  }

  return { chrome, debugPort, userDataDir, wsURL: version.webSocketDebuggerUrl };
}

async function createPage(cdp, baseURL) {
  const { targetId } = await cdp.send('Target.createTarget', { url: 'about:blank' });
  const { sessionId } = await cdp.send('Target.attachToTarget', { targetId, flatten: true });
  await cdp.send('Page.enable', {}, sessionId);
  await cdp.send('Runtime.enable', {}, sessionId);
  await cdp.send('Log.enable', {}, sessionId);
  await cdp.send('Emulation.setDeviceMetricsOverride', { ...DEFAULT_VIEWPORT, mobile: false }, sessionId);
  await cdp.send('Page.addScriptToEvaluateOnNewDocument', { source: smokeBridgeSource() }, sessionId);
  return new SmokePage(cdp, sessionId, baseURL);
}

class SmokePage {
  constructor(cdp, sessionId, baseURL) {
    this.cdp = cdp;
    this.sessionId = sessionId;
    this.baseURL = baseURL;
    this.consoleErrors = [];
    this.cdp.listeners.set('Runtime.consoleAPICalled', [
      (payload) => {
        if (payload.sessionId !== this.sessionId) return;
        const { type, args = [] } = payload.params ?? {};
        if (type === 'error' || type === 'assert') {
          this.consoleErrors.push(args.map((arg) => arg.value ?? arg.description ?? '').join(' '));
        }
      },
    ]);
    this.cdp.listeners.set('Runtime.exceptionThrown', [
      (payload) => {
        if (payload.sessionId !== this.sessionId) return;
        this.consoleErrors.push(payload.params?.exceptionDetails?.text ?? 'Runtime exception');
      },
    ]);
  }

  async eval(fn, ...args) {
    const expression = `(${fn.toString()})(...${JSON.stringify(args)})`;
    const result = await this.cdp.send('Runtime.evaluate', {
      expression,
      awaitPromise: true,
      returnByValue: true,
      userGesture: true,
    }, this.sessionId);
    if (result.exceptionDetails) {
      throw new Error(result.exceptionDetails.text || 'Runtime.evaluate failed');
    }
    return result.result?.value;
  }

  async navigate(route) {
    const normalizedRoute = route.replace(/^#?\/?/, '/');
    const url = `${this.baseURL}/#${normalizedRoute}`;
    const currentURL = await this.eval(() => location.href).catch(() => '');
    if (currentURL.startsWith(this.baseURL)) {
      await this.eval((nextHash) => {
        location.hash = nextHash;
        window.dispatchEvent(new HashChangeEvent('hashchange'));
      }, `#${normalizedRoute}`);
    } else {
      await this.cdp.send('Page.navigate', { url }, this.sessionId);
    }
    await this.waitFor(() => document.readyState === 'complete');
    await this.waitFor(() => document.body && document.body.textContent.length > 0);
    await this.waitFor(() => !document.body.textContent.includes('正在加载 DiveEnd 工作区'));
    await sleep(500);
  }

  async waitFor(fn, timeoutMs = 10000, intervalMs = 200, ...args) {
    const started = Date.now();
    let lastValue;
    while (Date.now() - started < timeoutMs) {
      lastValue = await this.eval(fn).catch((error) => ({ error: error.message }));
      if (lastValue === true || (lastValue && !lastValue.error)) {
        return lastValue;
      }
      await sleep(intervalMs);
    }
    throw new Error(`Timed out waiting for condition. Last value: ${JSON.stringify(lastValue)}`);
  }

  async waitForText(text, timeoutMs = 10000) {
    return this.waitFor(new Function(`return document.body.textContent.includes(${JSON.stringify(text)})`), timeoutMs, 200);
  }

  async screenshot(filePath) {
    const result = await this.cdp.send('Page.captureScreenshot', { format: 'png', captureBeyondViewport: true }, this.sessionId);
    await writeFile(filePath, Buffer.from(result.data, 'base64'));
  }

  async clickText(text, options = {}) {
    const clicked = await this.eval((wanted, selector) => {
      const candidates = Array.from(document.querySelectorAll(selector || 'button,a,label,[role="button"],summary,h1,h2,h3,h4,p,span')).sort((a, b) => a.textContent.trim().length - b.textContent.trim().length);
      const visible = (element) => {
        const rect = element.getBoundingClientRect();
        const style = getComputedStyle(element);
        return rect.width > 0 && rect.height > 0 && style.visibility !== 'hidden' && style.display !== 'none';
      };
      const element = candidates.find((candidate) => visible(candidate) && candidate.textContent.trim().includes(wanted));
      if (!element) return false;
      element.scrollIntoView({ block: 'center', inline: 'center' });
      element.click();
      return true;
    }, text, options.selector || 'button,a,label,[role="button"],summary,h1,h2,h3,h4,p,span');
    if (!clicked) {
      throw new Error(`Could not click visible text: ${text}`);
    }
    await sleep(options.afterMs ?? 250);
  }

  async fillPlaceholder(placeholder, value) {
    const filled = await this.eval((wanted, nextValue) => {
      const inputs = Array.from(document.querySelectorAll('input,textarea'));
      const input = inputs.find((candidate) => (candidate.getAttribute('placeholder') || '').includes(wanted));
      if (!input) return false;
      input.focus();
      const setter = Object.getOwnPropertyDescriptor(Object.getPrototypeOf(input), 'value')?.set;
      setter ? setter.call(input, nextValue) : (input.value = nextValue);
      input.dispatchEvent(new Event('input', { bubbles: true }));
      input.dispatchEvent(new Event('change', { bubbles: true }));
      return true;
    }, placeholder, value);
    if (!filled) {
      throw new Error(`Could not fill placeholder: ${placeholder}`);
    }
    await sleep(200);
  }

  async setFileInput(fileName = 'survey.pdf') {
    const changed = await this.eval((name) => {
      const input = document.querySelector('input[type="file"]');
      if (!input) return false;
      const file = new File(['%PDF-1.4 smoke'], name, { type: 'application/pdf' });
      const dataTransfer = new DataTransfer();
      dataTransfer.items.add(file);
      input.files = dataTransfer.files;
      input.dispatchEvent(new Event('change', { bubbles: true }));
      return true;
    }, fileName);
    if (!changed) {
      throw new Error('Could not set file input');
    }
    await sleep(300);
  }

  async assertText(text) {
    const exists = await this.eval((wanted) => document.body.textContent.includes(wanted), text);
    if (!exists) {
      throw new Error(`Expected text not found: ${text}`);
    }
  }

  async assertStartButtonDisabled() {
    const disabled = await this.eval(() => {
      const buttons = Array.from(document.querySelectorAll('button'));
      const start = buttons.find((button) => button.textContent.includes('开始检索'));
      return Boolean(start?.disabled);
    });
    if (!disabled) {
      throw new Error('Expected DeepStart start button to be disabled before input');
    }
  }

  async emitRuntimeEvent(eventName, payload) {
    await this.eval((name, data) => window.__diveendSmoke?.emit?.(name, data), eventName, payload);
    await sleep(250);
  }
}

async function main() {
  await mkdir(OUTPUT_ROOT, { recursive: true });
  const report = [];
  const errors = [];
  const serverPort = process.env.UI_SMOKE_PORT ? Number(process.env.UI_SMOKE_PORT) : await getFreePort();
  const baseURL = process.env.UI_SMOKE_BASE_URL || `http://127.0.0.1:${serverPort}`;
  const server = process.env.UI_SMOKE_BASE_URL ? null : startViteServer(serverPort);
  const chromeHandle = await launchChrome();
  const cdp = await CDPClient.connect(chromeHandle.wsURL);
  const page = await createPage(cdp, baseURL);

  const shot = async (name, description) => {
    const file = path.join(OUTPUT_ROOT, `${String(report.length + 1).padStart(2, '0')}-${name}.png`);
    await page.screenshot(file);
    report.push({ name, description, file });
    log(`${name}: ${file}`);
  };

  try {
    await waitForHTTP(baseURL);

    await page.navigate('/');
    log('bridge injected: ' + await page.eval(() => Boolean(window.__diveendSmoke)));
    await page.assertText('DiveEnd');
    await shot('home', 'Home 初始工作台与全局导航');

    await page.navigate('/deepstart');
    await page.assertStartButtonDisabled();
    await shot('deepstart-empty-disabled', 'DeepStart 空输入时开始按钮禁用态');
    await page.fillPlaceholder('哪些方法真正提升了代码智能体', 'scientific reading assistant');
    await shot('deepstart-ready', 'DeepStart 输入后可执行状态');
    await page.clickText('开始检索');
    await shot('deepstart-loading', 'DeepStart 检索/分析 loading 状态');
    await page.navigate('/session/session-smoke');
    await page.waitForText('Unified Embodied Agent Benchmark');
    await shot('deepstart-session', 'DeepStart 会话详情、AI Chat、候选论文卡片');
    await page.clickText('embodied benchmark');
    await shot('deepstart-suggested-query', 'DeepStart 建议 query 只填入输入框，不误触发重搜');
    await page.clickText('Unified Embodied Agent Benchmark');
    await page.waitForText('Paper Detail');
    await shot('deepstart-paper-detail', 'DeepStart 论文详情抽屉');

    await page.navigate('/deepread');
    await page.waitForText('论文阅读工作台');
    await shot('deepread-library', 'DeepRead 文库与缺失 PDF 状态');
    await page.clickText('论文库');
    await page.clickText('Cache/Subtopic');
    await page.waitForText('Child Folder Paper');
    await page.waitForText('no downloadable pdf url');
    await shot('deepread-download-error', 'DeepRead 下载失败、重试入口');
    await page.clickText('论文库');
    await page.clickText('手动链接');
    await page.waitForText('填写可访问的 http/https PDF 链接');
    await page.fillPlaceholder('https://...', 'https://example.org/manual-paper.pdf');
    await shot('deepread-manual-url', 'DeepRead 手动 PDF URL 重试表单');
    await page.clickText('使用该链接重试');
    await shot('deepread-retry-submitted', 'DeepRead 手动重试提交后的状态');

    await page.navigate('/screening');
    await page.waitForText('导入待分析论文');
    await shot('screening-upload', 'Screening 上传入口和步骤条');
    await page.clickText('选择 PDF 文件');
    await shot('screening-extracting', 'Screening 文件上传后提取阶段/loading 状态');
    await page.waitForText('你最想先保留哪一类论文？');
    await shot('screening-decision', 'Screening AI 决策树选项');
    await page.clickText('综述与综览');
    await shot('screening-option-selected', 'Screening 选项选择状态');
    await page.clickText('继续下一步');
    await page.waitForText('筛选完成');
    await shot('screening-complete', 'Screening 结果确认状态');
    await page.clickText('导入到文库');
    await page.waitForText('已成功导入');
    await shot('screening-imported', 'Screening 导入完成反馈');

    await page.navigate('/sync');
    await page.waitForText('已连接');
    await shot('sync-dashboard', 'Sync 仪表盘、状态卡片、手动同步入口');
    await page.clickText('更新凭证');
    await page.waitForText('百度 access token 已通过 refresh token 更新');
    await shot('sync-refresh-token', 'Sync 手动刷新百度 access token');
    await page.clickText('检查并同步');
    await page.waitForText('确认本次百度云同步');
    await shot('sync-preflight', 'Sync 手动同步前预检确认');
    await page.clickText('确认开始同步');
    await page.waitForText('上传数据库快照');
    await shot('sync-progress', 'Sync 手动同步进行中状态');
    await page.emitRuntimeEvent('sync-progress', { total: 3, completed: 3, currentFile: '', status: 'complete', message: '同步完成' });
    await sleep(500);
    await shot('sync-progress-complete', 'Sync runtime event 完成态刷新');
    await page.clickText('冲突处理');
    await page.waitForText('SQLite 数据库');
    await shot('sync-conflict-diff', 'Sync 冲突 diff 元数据和覆盖前保留提示');
    await page.clickText('退出前同步', { selector: 'label,span,button' });
    await shot('sync-settings-toggle', 'Sync 设置切换、退出前同步独立开关');

    const markdown = [
      '# DiveEnd UI Smoke Report',
      '',
      `Generated: ${new Date().toISOString()}`,
      `Base URL: ${baseURL}`,
      '',
      '## Screenshots',
      '',
      ...report.map((item, index) => `${index + 1}. **${item.name}** — ${item.description}\n   - ${item.file}`),
      '',
      '## Console errors',
      '',
      page.consoleErrors.length === 0 ? '- None' : page.consoleErrors.map((item) => `- ${item}`),
    ].flat().join('\n');
    await writeFile(path.join(OUTPUT_ROOT, 'report.md'), markdown);
    await writeFile(path.join(OUTPUT_ROOT, 'report.json'), JSON.stringify({ baseURL, screenshots: report, consoleErrors: page.consoleErrors }, null, 2));

    if (page.consoleErrors.length > 0) {
      throw new Error(`UI smoke finished with console errors: ${page.consoleErrors.join(' | ')}`);
    }

    log(`Report: ${path.join(OUTPUT_ROOT, 'report.md')}`);
  } catch (error) {
    errors.push(error);
    const bodyText = await page.eval(() => document.body.textContent.slice(0, 5000)).catch(() => '');
    await writeFile(path.join(OUTPUT_ROOT, 'error.json'), JSON.stringify({ message: error.message, stack: error.stack, bodyText, screenshots: report, consoleErrors: page.consoleErrors }, null, 2));
    throw error;
  } finally {
    cdp.close();
    chromeHandle.chrome.kill('SIGTERM');
    if (server) server.kill('SIGTERM');
    if (!process.env.UI_SMOKE_KEEP_PROFILE) {
      await rm(chromeHandle.userDataDir, { recursive: true, force: true }).catch(() => undefined);
    }
  }
}

main().catch((error) => {
  console.error(`[ui-smoke] FAILED: ${error.stack || error.message}`);
  process.exit(1);
});
