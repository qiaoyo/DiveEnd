import { useEffect, useMemo, useRef, useState } from 'react';
import {
  BookOpen,
  BookText,
  ExternalLink,
  FileText,
  Folder,
  Languages,
  Loader2,
  Minus,
  Plus,
  RefreshCw,
  Save,
  SunMoon,
} from 'lucide-react';
import { Document, Page, pdfjs } from 'react-pdf';
import pdfWorkerSrc from 'pdfjs-dist/build/pdf.worker.min.mjs?url';
import {
  getDeepReadPDFBytes,
  getDeepReadPDFURL,
  getDeepReadState,
  getPDFServiceStatus,
  getFolderTree,
  getPapers,
  prepareDeepReadPaper,
  retryFolderPendingDownloads,
  retryPaperDownload,
  retryPaperDownloadWithURL,
  saveConfig,
  saveDeepReadNote,
  selectAndAttachPaperPDF,
  translatePaperSection,
} from '../../lib/backend';
import { errorToUserMessage } from '../../lib/errors';
import type { DeepReadState, Folder as FolderRecord, FolderNode, Paper, PDFServiceStatus } from '../../types';
import { useAppStore } from '../../stores/appStore';

pdfjs.GlobalWorkerOptions.workerSrc = pdfWorkerSrc;

type LeftTab = 'directory' | 'thumbnails';

function deepReadSectionLabel(section?: DeepReadState['sections'][number]): string {
  if (!section) {
    return 'Untitled Section';
  }
  return section.title?.trim() || 'Untitled Section';
}

function fileURLFromPath(path: string): string {
  const trimmed = path.trim();
  if (!trimmed) {
    return '';
  }
  if (trimmed.startsWith('http://') || trimmed.startsWith('https://') || trimmed.startsWith('file://')) {
    return trimmed;
  }
  return `file://${encodeURI(trimmed)}`;
}

function decodeBase64PDFBytes(base64: string): Uint8Array {
  const normalized = (base64 || '').trim();
  if (!normalized) {
    return new Uint8Array();
  }
  const binary = window.atob(normalized);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i += 1) {
    bytes[i] = binary.charCodeAt(i);
  }
  return bytes;
}

async function loadDeepReadPDFSource(paperId: string): Promise<{ resourceURL: string; bytes: Uint8Array | null }> {
  try {
    const resourceURL = (await getDeepReadPDFURL(paperId)).trim();
    if (resourceURL) {
      return { resourceURL, bytes: null };
    }
  } catch {
    // Fall through to byte fallback for older bridges or temporary asset-server failures.
  }

  try {
    const bytes = decodeBase64PDFBytes(await getDeepReadPDFBytes(paperId));
    return { resourceURL: '', bytes: bytes.length > 0 ? bytes : null };
  } catch {
    return { resourceURL: '', bytes: null };
  }
}

function flattenFolderNodes(nodes: FolderNode[]): FolderRecord[] {
  const items: FolderRecord[] = [];
  const walk = (list: FolderNode[]) => {
    for (const node of list) {
      items.push(node.folder);
      walk(node.children);
    }
  };
  walk(nodes);
  return items;
}

function statusLabel(status: string): string {
  switch ((status || '').toLowerCase()) {
    case 'queued':
      return '排队中';
    case 'downloading':
      return '下载中';
    case 'downloaded':
      return '已下载';
    case 'failed':
      return '下载失败';
    default:
      return status || '未知状态';
  }
}

function statusStyle(status: string): string {
  switch ((status || '').toLowerCase()) {
    case 'downloaded':
      return 'border-emerald-300 bg-emerald-100/80 text-emerald-700 dark:border-emerald-500/40 dark:bg-emerald-500/15 dark:text-emerald-200';
    case 'failed':
      return 'border-rose-300 bg-rose-100/80 text-rose-700 dark:border-rose-500/40 dark:bg-rose-500/15 dark:text-rose-200';
    case 'downloading':
      return 'border-indigo-300 bg-indigo-100/80 text-indigo-700 dark:border-indigo-500/40 dark:bg-indigo-500/15 dark:text-indigo-200';
    default:
      return 'border-amber-300 bg-amber-100/80 text-amber-700 dark:border-amber-500/40 dark:bg-amber-500/15 dark:text-amber-200';
  }
}

function isNoDownloadablePDFError(errorMessage: string): boolean {
  return errorMessage.toLowerCase().includes('no downloadable pdf url');
}

function isHttpURL(raw: string | undefined): boolean {
  return /^https?:\/\//i.test((raw || '').trim());
}

function queryTokens(query: string): string[] {
  return query
    .toLowerCase()
    .split(/[\s,;，；、|/]+/)
    .map((token) => token.trim())
    .filter(Boolean);
}

export function DeepReadPanel() {
  const {
    activeFolderId,
    config,
    folders,
    isTranslating,
    papers,
    prependTranslation,
    selectedPaper,
    setActiveFolderId,
    setConfig,
    setError,
    setFolders,
    setIsTranslating,
    setPapers,
    setSelectedPaper,
    theme,
  } = useAppStore();

  const [folderTree, setFolderTree] = useState<FolderNode[]>([]);
  const [libraryLoading, setLibraryLoading] = useState(false);
  const [libraryQuery, setLibraryQuery] = useState('');
  const [retryingPaperId, setRetryingPaperId] = useState('');
  const [retryingFolder, setRetryingFolder] = useState(false);
  const [attachingPaperId, setAttachingPaperId] = useState('');
  const [leftTab, setLeftTab] = useState<LeftTab>('directory');
  const [deepReadState, setDeepReadState] = useState<DeepReadState | null>(null);
  const [loadingState, setLoadingState] = useState(false);
  const [preparing, setPreparing] = useState(false);
  const [selectedSectionId, setSelectedSectionId] = useState('');
  const [originalText, setOriginalText] = useState('');
  const [noteDraft, setNoteDraft] = useState('');
  const [savingNote, setSavingNote] = useState(false);
  const [switchingTheme, setSwitchingTheme] = useState(false);
  const [pageNumber, setPageNumber] = useState(1);
  const [numPages, setNumPages] = useState(0);
  const [pdfZoom, setPDFZoom] = useState(0.9);
  const [pdfViewportWidth, setPDFViewportWidth] = useState(780);
  const [pdfLoadError, setPDFLoadError] = useState('');
  const [pdfResourceURL, setPDFResourceURL] = useState('');
  const [pdfBytes, setPDFBytes] = useState<Uint8Array | null>(null);
  const [translationError, setTranslationError] = useState('');
  const [manualURLDrafts, setManualURLDrafts] = useState<Record<string, string>>({});
  const [manualURLPanelPaperId, setManualURLPanelPaperId] = useState<string | null>(null);
  const [submittingManualURLPaperId, setSubmittingManualURLPaperId] = useState('');
  const [pdfServiceStatus, setPDFServiceStatus] = useState<PDFServiceStatus | null>(null);
  const manualURLInputRef = useRef<HTMLInputElement | null>(null);
  const pdfViewportRef = useRef<HTMLDivElement | null>(null);
  const pdfPageRefs = useRef<Record<number, HTMLDivElement | null>>({});

  const sections = deepReadState?.sections ?? [];
  const selectedSection = useMemo(
    () => sections.find((section) => section.id === selectedSectionId) || sections[0],
    [sections, selectedSectionId]
  );
  const pdfSrc = useMemo(() => fileURLFromPath(deepReadState?.pdfPath ?? ''), [deepReadState?.pdfPath]);
  const pdfFileInput = useMemo(() => {
    if (pdfResourceURL) {
      return pdfResourceURL;
    }
    if (pdfBytes && pdfBytes.length > 0) {
      const data = pdfBytes.buffer.slice(pdfBytes.byteOffset, pdfBytes.byteOffset + pdfBytes.byteLength);
      return { data };
    }
    if (pdfSrc) {
      return pdfSrc;
    }
    return '';
  }, [pdfBytes, pdfResourceURL, pdfSrc]);
  const translationHistory = deepReadState?.translations ?? [];
  const noteHistory = deepReadState?.notes ?? [];
  const latestTranslation = translationHistory[0];
  const selectedPaperId = selectedPaper?.id ?? '';
  const loadingDownloads = papers.some((paper) => {
    const status = (paper.downloadStatus || '').toLowerCase();
    return status === 'queued' || status === 'downloading';
  });

  const filteredPapers = useMemo(() => {
    const tokens = queryTokens(libraryQuery);
    if (tokens.length === 0) {
      return papers;
    }
    return papers.filter((paper) => {
      const haystack = `${paper.title} ${paper.authors} ${paper.journal} ${paper.abstract} ${paper.tags.join(' ')}`.toLowerCase();
      return tokens.every((token) => haystack.includes(token));
    });
  }, [libraryQuery, papers]);

  const readerPageWidth = useMemo(() => {
    const available = Math.max(320, Math.floor(pdfViewportWidth - 24));
    const base = Math.min(760, available);
    const scaled = Math.round(base * pdfZoom);
    const maxAllowed = Math.round(available * 1.35);
    return Math.max(280, Math.min(maxAllowed, scaled));
  }, [pdfViewportWidth, pdfZoom]);
  const estimatedPDFPageHeight = Math.round(readerPageWidth * 1.36 + 28);
  const renderedPDFPages = useMemo(() => {
    if (numPages <= 0) {
      return [] as number[];
    }
    const buffer = 2;
    const start = Math.max(1, pageNumber - buffer);
    const end = Math.min(numPages, pageNumber + buffer);
    return Array.from({ length: end - start + 1 }, (_, index) => start + index);
  }, [numPages, pageNumber]);
  const firstRenderedPDFPage = renderedPDFPages[0] ?? 1;
  const lastRenderedPDFPage = renderedPDFPages[renderedPDFPages.length - 1] ?? 0;
  const pdfTopSpacerHeight = Math.max(0, firstRenderedPDFPage - 1) * estimatedPDFPageHeight;
  const pdfBottomSpacerHeight = Math.max(0, numPages - lastRenderedPDFPage) * estimatedPDFPageHeight;

  const setPDFPageRef = (page: number, element: HTMLDivElement | null) => {
    if (element) {
      pdfPageRefs.current[page] = element;
      return;
    }
    delete pdfPageRefs.current[page];
  };

  const jumpToPage = (page: number, behavior: ScrollBehavior = 'smooth') => {
    if (numPages <= 0) {
      return;
    }
    const normalized = Math.min(Math.max(page, 1), numPages);
    setPageNumber(normalized);
    window.setTimeout(() => {
      const viewport = pdfViewportRef.current;
      const target = pdfPageRefs.current[normalized];
      if (!viewport) {
        return;
      }
      viewport.scrollTo({
        top: target ? Math.max(target.offsetTop - 8, 0) : Math.max((normalized - 1) * estimatedPDFPageHeight, 0),
        behavior,
      });
    }, 0);
  };

  const handlePDFViewportScroll = () => {
    const viewport = pdfViewportRef.current;
    if (!viewport || numPages <= 0) {
      return;
    }
    const estimatedPage = Math.floor((viewport.scrollTop + viewport.clientHeight * 0.18) / estimatedPDFPageHeight) + 1;
    const nearestPage = Math.min(Math.max(estimatedPage, 1), numPages);
    if (nearestPage !== pageNumber) {
      setPageNumber(nearestPage);
    }
  };

  const loadFolderPapers = async (folderId: string, preserveSelection = true): Promise<Paper[]> => {
    const nextPapers = await getPapers(folderId);
    setActiveFolderId(folderId);
    setPapers(nextPapers);

    if (nextPapers.length === 0) {
      setSelectedPaper(null);
      return nextPapers;
    }

    if (preserveSelection && selectedPaper) {
      const matched = nextPapers.find((item) => item.id === selectedPaper.id);
      if (matched) {
        setSelectedPaper(matched);
        return nextPapers;
      }
    }

    setSelectedPaper(nextPapers[0]);
    return nextPapers;
  };

  useEffect(() => {
    let cancelled = false;
    getPDFServiceStatus()
      .then((status) => {
        if (!cancelled) {
          setPDFServiceStatus(status);
        }
      })
      .catch((error) => {
        if (!cancelled) {
          setPDFServiceStatus({
            enabled: false,
            url: '',
            healthy: false,
            ready: false,
            checkedAt: new Date().toISOString(),
            message: errorToUserMessage(error, '读取 PDF 服务状态失败'),
          });
        }
      });
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    if (manualURLPanelPaperId) {
      manualURLInputRef.current?.focus();
    }
  }, [manualURLPanelPaperId]);

  useEffect(() => {
    let cancelled = false;
    const loadLibrary = async () => {
      setLibraryLoading(true);
      try {
        const tree = await getFolderTree();
        if (cancelled) {
          return;
        }
        setFolderTree(tree);
        const flattened = flattenFolderNodes(tree);
        setFolders(flattened);

        const preferred =
          (activeFolderId && flattened.some((folder) => folder.id === activeFolderId) && activeFolderId) ||
          flattened[0]?.id ||
          '';
        if (preferred) {
          await loadFolderPapers(preferred, true);
        } else {
          setPapers([]);
          setSelectedPaper(null);
        }
      } catch (error) {
        if (!cancelled) {
          setError(errorToUserMessage(error, '加载目录失败'));
        }
      } finally {
        if (!cancelled) {
          setLibraryLoading(false);
        }
      }
    };

    void loadLibrary();
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    if (!activeFolderId || !loadingDownloads) {
      return;
    }

    const timer = window.setInterval(() => {
      void getPapers(activeFolderId)
        .then((nextPapers) => {
          setPapers(nextPapers);
          if (!selectedPaper) {
            return;
          }
          const matched = nextPapers.find((paper) => paper.id === selectedPaper.id);
          if (matched) {
            setSelectedPaper(matched);
          }
        })
        .catch(() => {});
    }, 4000);

    return () => {
      window.clearInterval(timer);
    };
  }, [activeFolderId, loadingDownloads, selectedPaper, setPapers, setSelectedPaper]);

  useEffect(() => {
    if (!selectedPaperId) {
      setDeepReadState(null);
      setSelectedSectionId('');
      setOriginalText('');
      setPageNumber(1);
      setNumPages(0);
      setPDFLoadError('');
      setPDFResourceURL('');
      setPDFBytes(null);
      return;
    }

    let cancelled = false;
    setLoadingState(true);
    setPDFLoadError('');
    setPDFResourceURL('');
    setPDFBytes(null);
    void getDeepReadState(selectedPaperId)
      .then(async (state) => {
        if (cancelled) {
          return;
        }
        setDeepReadState(state);
        const nextSection = state.sections[0];
        setSelectedSectionId(nextSection?.id ?? '');
        setOriginalText((nextSection?.content || selectedPaper?.abstract || '').trim());
        setPageNumber(1);
        setNumPages(0);

        // 在 Wails 环境优先走同源 asset URL，避免 file:// 被 WebView 拦截；
        // asset URL 不可用时再回退到后端字节流。
        if (state.hasPdf && state.pdfPath.trim()) {
          const source = await loadDeepReadPDFSource(selectedPaperId);
          if (cancelled) {
            return;
          }
          setPDFResourceURL(source.resourceURL);
          setPDFBytes(source.bytes);
        }
      })
      .catch((error) => {
        if (!cancelled) {
          setError(errorToUserMessage(error, '读取 DeepRead 状态失败'));
          setDeepReadState(null);
          setPDFResourceURL('');
          setPDFBytes(null);
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoadingState(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [selectedPaperId, setError]);

  useEffect(() => {
    pdfPageRefs.current = {};
  }, [selectedPaperId, pdfFileInput]);

  useEffect(() => {
    const viewport = pdfViewportRef.current;
    if (!viewport || typeof ResizeObserver === 'undefined') {
      return;
    }
    const observer = new ResizeObserver((entries) => {
      const nextWidth = entries[0]?.contentRect?.width ?? 0;
      if (nextWidth > 0) {
        setPDFViewportWidth(nextWidth);
      }
    });
    observer.observe(viewport);
    return () => observer.disconnect();
  }, [selectedPaperId, deepReadState?.hasPdf]);

  useEffect(() => {
    if (selectedSection?.content && selectedSection.content !== originalText) {
      setOriginalText(selectedSection.content);
    } else if (!selectedSection && selectedPaper?.abstract && !originalText.trim()) {
      setOriginalText(selectedPaper.abstract);
    }
  }, [selectedSection, selectedPaper?.abstract]); // eslint-disable-line react-hooks/exhaustive-deps

  const handlePrepare = async () => {
    if (!selectedPaper) {
      return;
    }
    setPreparing(true);
    setPDFLoadError('');
    setPDFResourceURL('');
    setPDFBytes(null);
    try {
      const state = await prepareDeepReadPaper(selectedPaper.id);
      setDeepReadState(state);
      if (!selectedSectionId && state.sections.length > 0) {
        setSelectedSectionId(state.sections[0].id);
      }
      if (state.hasPdf && state.pdfPath.trim()) {
        const source = await loadDeepReadPDFSource(selectedPaper.id);
        setPDFResourceURL(source.resourceURL);
        setPDFBytes(source.bytes);
      }
    } catch (error) {
      setError(errorToUserMessage(error, '准备 DeepRead 内容失败'));
    } finally {
      setPreparing(false);
    }
  };

  const handleTranslate = async () => {
    if (!selectedPaper) {
      return;
    }
    if (!originalText.trim()) {
      setError('请先选择章节或输入需要翻译的文本');
      return;
    }

    setIsTranslating(true);
    setTranslationError('');
    try {
      const record = await translatePaperSection(
        selectedPaper.id,
        deepReadSectionLabel(selectedSection),
        originalText.trim()
      );
      prependTranslation(record);
      setDeepReadState((prev) => {
        if (!prev) {
          return prev;
        }
        return {
          ...prev,
          translations: [record, ...prev.translations.filter((item) => item.id !== record.id)],
        };
      });
    } catch (error) {
      const message = errorToUserMessage(error, '翻译失败');
      setTranslationError(message);
      setError(message);
    } finally {
      setIsTranslating(false);
    }
  };

  const handleSaveNote = async () => {
    if (!selectedPaper) {
      return;
    }
    if (!noteDraft.trim()) {
      setError('请先输入笔记内容');
      return;
    }

    setSavingNote(true);
    try {
      const note = await saveDeepReadNote(selectedPaper.id, deepReadSectionLabel(selectedSection), noteDraft.trim());
      setDeepReadState((prev) => {
        if (!prev) {
          return prev;
        }
        return {
          ...prev,
          notes: [note, ...prev.notes.filter((item) => item.id !== note.id)],
        };
      });
      setNoteDraft('');
    } catch (error) {
      setError(errorToUserMessage(error, '保存笔记失败'));
    } finally {
      setSavingNote(false);
    }
  };

  const handleToggleTheme = async () => {
    if (switchingTheme) {
      return;
    }
    const nextTheme = theme === 'dark' ? 'light' : 'dark';
    setSwitchingTheme(true);
    try {
      const result = await saveConfig({
        ...config,
        theme: nextTheme,
      });
      setConfig(result.config);
    } catch (error) {
      setError(errorToUserMessage(error, '切换主题失败'));
    } finally {
      setSwitchingTheme(false);
    }
  };

  const handleRetryDownload = async (paper: Paper) => {
    if (retryingPaperId) {
      return;
    }
    setRetryingPaperId(paper.id);
    try {
      await retryPaperDownload(paper.id);
      let refreshedPapers: Paper[] = papers;
      if (activeFolderId) {
        refreshedPapers = await loadFolderPapers(activeFolderId, true);
      }
      const latest = refreshedPapers.find((item) => item.id === paper.id);
      if (
        latest &&
        latest.downloadStatus.toLowerCase() === 'failed' &&
        isNoDownloadablePDFError(latest.downloadError || '')
      ) {
        setManualURLPanelPaperId(paper.id);
        setManualURLDrafts((prev) => ({
          ...prev,
          [paper.id]: (prev[paper.id] || latest.url || '').trim(),
        }));
      } else if (manualURLPanelPaperId === paper.id) {
        setManualURLPanelPaperId(null);
      }
    } catch (error) {
      setError(errorToUserMessage(error, '重试下载失败'));
    } finally {
      setRetryingPaperId('');
    }
  };

  const handleRetryFolderDownloads = async () => {
    if (!activeFolderId || retryingFolder) {
      return;
    }
    setRetryingFolder(true);
    try {
      const queued = await retryFolderPendingDownloads(activeFolderId);
      await loadFolderPapers(activeFolderId, true);
      if (queued === 0) {
        setError('当前文件夹没有需要重新下载的论文');
      }
    } catch (error) {
      setError(errorToUserMessage(error, '批量重新下载失败'));
    } finally {
      setRetryingFolder(false);
    }
  };

  const handleAttachLocalPDF = async (paper: Paper) => {
    if (attachingPaperId) {
      return;
    }
    setAttachingPaperId(paper.id);
    setPDFLoadError('');
    try {
      const updated = await selectAndAttachPaperPDF(paper.id);
      if (activeFolderId) {
        await loadFolderPapers(activeFolderId, true);
      }
      if (selectedPaper?.id === paper.id) {
        setSelectedPaper(updated);
        const state = await getDeepReadState(paper.id);
        setDeepReadState(state);
        const source = await loadDeepReadPDFSource(paper.id);
        setPDFResourceURL(source.resourceURL);
        setPDFBytes(source.bytes);
      }
      setManualURLPanelPaperId(null);
    } catch (error) {
      setError(errorToUserMessage(error, '导入本地 PDF 失败'));
    } finally {
      setAttachingPaperId('');
    }
  };

  const handleManualURLDraftChange = (paperId: string, value: string) => {
    setManualURLDrafts((prev) => ({
      ...prev,
      [paperId]: value,
    }));
  };

  const handleRetryWithManualURL = async (paper: Paper) => {
    const draft = (manualURLDrafts[paper.id] || '').trim();
    if (!draft) {
      setError('请先填写可访问的 http/https PDF 链接');
      return;
    }

    setSubmittingManualURLPaperId(paper.id);
    try {
      await retryPaperDownloadWithURL(paper.id, draft);
      if (activeFolderId) {
        await loadFolderPapers(activeFolderId, true);
      }
      setManualURLPanelPaperId(null);
    } catch (error) {
      setError(errorToUserMessage(error, '手动链接重试失败'));
    } finally {
      setSubmittingManualURLPaperId('');
    }
  };

  const renderFolderNode = (node: FolderNode, depth = 0) => {
    const isActive = activeFolderId === node.folder.id;
    return (
      <div key={node.folder.id} className="space-y-1">
        <button
          type="button"
          onClick={() => void loadFolderPapers(node.folder.id, true)}
          style={{ paddingLeft: 10 + depth * 12 }}
          className={`flex w-full items-center gap-2 rounded-lg py-1.5 pr-2 text-left text-xs transition ${
            isActive
              ? 'bg-indigo-100 text-indigo-700 dark:bg-indigo-500/20 dark:text-indigo-100'
              : 'text-slate-600 hover:bg-slate-100 hover:text-slate-900 dark:text-slate-300 dark:hover:bg-slate-800/70 dark:hover:text-white'
          }`}
        >
          <Folder className="h-3.5 w-3.5 shrink-0" />
          <span className="truncate">{node.folder.path || node.folder.name}</span>
        </button>
        {node.children.map((child) => renderFolderNode(child, depth + 1))}
      </div>
    );
  };

  const currentFolder = folders.find((folder) => folder.id === activeFolderId);
  const parseStatus = deepReadState?.parseStatus || 'idle';
  const isMissingPDF = parseStatus === 'missing_pdf';
  const selectedPaperURL = selectedPaper?.url || '';

  return (
    <div className="flex h-full min-h-0 flex-col overflow-hidden bg-slate-50 text-slate-900 dark:bg-slate-950 dark:text-slate-100">
      <div className="border-b border-slate-200/80 bg-white/80 px-4 py-2.5 backdrop-blur-md dark:border-slate-700/80 dark:bg-slate-900/80">
        <div className="flex items-center justify-between gap-3">
          <div className="min-w-0">
            <p className="text-xs uppercase tracking-[0.2em] text-slate-500 dark:text-slate-400">DeepRead Workspace</p>
            <h2 className="truncate text-sm font-semibold">{selectedPaper?.title || '选择右侧论文开始阅读'}</h2>
            {pdfServiceStatus && (
              <p className={`mt-1 text-xs ${pdfServiceStatus.ready ? 'text-emerald-600 dark:text-emerald-300' : 'text-amber-600 dark:text-amber-300'}`}>
                PDF 服务：{pdfServiceStatus.ready ? 'ready' : '需要检查'}{pdfServiceStatus.message ? ` · ${pdfServiceStatus.message}` : ''}
              </p>
            )}
          </div>
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={() => void handlePrepare()}
              disabled={preparing || !selectedPaper}
              className="inline-flex items-center gap-1.5 rounded-xl border border-indigo-300/70 bg-indigo-100 px-3 py-1.5 text-xs text-indigo-700 transition hover:bg-indigo-200 disabled:opacity-60 dark:border-indigo-400/50 dark:bg-indigo-500/20 dark:text-indigo-100 dark:hover:bg-indigo-500/30"
            >
              {preparing ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <RefreshCw className="h-3.5 w-3.5" />}
              {preparing ? '处理中' : '准备阅读内容'}
            </button>
            {isHttpURL(selectedPaperURL) && (
              <a
                href={selectedPaperURL}
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-1.5 rounded-xl border border-slate-300 bg-white px-3 py-1.5 text-xs text-slate-700 transition hover:border-indigo-400 hover:text-indigo-700 dark:border-slate-600 dark:bg-slate-800/80 dark:text-slate-200 dark:hover:text-indigo-100"
              >
                打开网页
                <ExternalLink className="h-3.5 w-3.5" />
              </a>
            )}
          </div>
        </div>
      </div>

      <div className="grid min-h-0 flex-1 gap-3 p-3 xl:grid-cols-[280px_1fr_360px]">
        <aside className="min-h-0 rounded-2xl border border-slate-200 bg-white/85 p-3 dark:border-slate-700/80 dark:bg-slate-900/80">
          <div className="rounded-xl border border-slate-200 bg-white/85 p-3 dark:border-slate-700/70 dark:bg-slate-900/75">
            <h3 className="text-xs uppercase tracking-[0.16em] text-slate-500 dark:text-slate-400">Outline</h3>
            <div className="mt-3 max-h-44 space-y-2 overflow-y-auto">
              {sections.length > 0 ? (
                sections.map((item) => (
                  <button
                    key={item.id}
                    type="button"
                    onClick={() => setSelectedSectionId(item.id)}
                    className={`w-full rounded-lg border px-2.5 py-1.5 text-left text-xs transition ${
                      selectedSection?.id === item.id
                        ? 'border-indigo-300 bg-indigo-100 text-indigo-700 dark:border-indigo-400/60 dark:bg-indigo-500/20 dark:text-indigo-100'
                        : 'border-slate-200 bg-white text-slate-600 hover:border-indigo-300 hover:text-slate-900 dark:border-slate-700 dark:bg-slate-800/80 dark:text-slate-300 dark:hover:border-indigo-500/40 dark:hover:text-slate-100'
                    }`}
                  >
                    {item.title}
                  </button>
                ))
              ) : (
                <p className="text-xs text-slate-500 dark:text-slate-400">{loadingState ? '正在读取章节...' : '暂无章节，先准备阅读内容。'}</p>
              )}
            </div>
          </div>

          <div className="mt-3 min-h-0 rounded-xl border border-slate-200 bg-white/85 p-3 dark:border-slate-700/70 dark:bg-slate-900/75">
            <div className="flex items-center gap-1">
              <button
                type="button"
                onClick={() => setLeftTab('directory')}
                className={`rounded-lg px-2.5 py-1 text-[11px] ${
                  leftTab === 'directory'
                    ? 'bg-indigo-100 text-indigo-700 dark:bg-indigo-500/25 dark:text-indigo-100'
                    : 'text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800'
                }`}
              >
                目录结构
              </button>
              <button
                type="button"
                onClick={() => setLeftTab('thumbnails')}
                className={`rounded-lg px-2.5 py-1 text-[11px] ${
                  leftTab === 'thumbnails'
                    ? 'bg-indigo-100 text-indigo-700 dark:bg-indigo-500/25 dark:text-indigo-100'
                    : 'text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800'
                }`}
              >
                页面缩略图
              </button>
            </div>

            {leftTab === 'directory' ? (
              <div className="mt-3 max-h-52 space-y-1 overflow-y-auto text-xs text-slate-600 dark:text-slate-300">
                {sections.length > 0 ? (
                  sections.map((section) => (
                    <button
                      key={`dir-${section.id}`}
                      type="button"
                      onClick={() => setSelectedSectionId(section.id)}
                      className="block w-full truncate rounded-md px-2 py-1 text-left hover:bg-slate-100 dark:hover:bg-slate-800"
                    >
                      {section.index + 1}. {section.title}
                    </button>
                  ))
                ) : (
                  <p className="text-slate-500 dark:text-slate-400">暂无目录结构</p>
                )}
              </div>
            ) : (
              <div className="mt-3 max-h-52 overflow-y-auto rounded-lg border border-slate-200 bg-slate-50 p-2 dark:border-slate-700 dark:bg-slate-900/60">
                {pdfFileInput && deepReadState?.hasPdf ? (
                  <Document
                    file={pdfFileInput}
                    loading={<div className="py-4 text-center text-xs text-slate-500 dark:text-slate-400">加载缩略图中...</div>}
                    onLoadSuccess={({ numPages: totalPages }) => {
                      setNumPages(totalPages || 0);
                      setPDFLoadError('');
                    }}
                    onLoadError={(error) => setPDFLoadError(errorToUserMessage(error, '加载失败'))}
                  >
                    <div className="grid grid-cols-3 gap-2">
                      {Array.from({ length: Math.min(numPages || 0, 18) }, (_, index) => {
                        const page = index + 1;
                        const isCurrent = page === pageNumber;
                        return (
                          <button
                            key={`thumb-${page}`}
                            type="button"
                            onClick={() => jumpToPage(page)}
                            className={`rounded border p-1 text-[10px] ${
                              isCurrent
                                ? 'border-indigo-300 bg-indigo-100 dark:border-indigo-400 dark:bg-indigo-500/20'
                                : 'border-slate-200 dark:border-slate-700'
                            }`}
                          >
                            <Page pageNumber={page} width={68} renderAnnotationLayer={false} renderTextLayer={false} />
                            <div className="mt-1">P{page}</div>
                          </button>
                        );
                      })}
                    </div>
                  </Document>
                ) : (
                  <p className="px-1 py-2 text-xs text-slate-500 dark:text-slate-400">PDF 就绪后可预览页面缩略图。</p>
                )}
              </div>
            )}
          </div>

          <div className="mt-3 rounded-xl border border-slate-200 bg-white/85 p-3 text-xs text-slate-600 dark:border-slate-700/70 dark:bg-slate-900/75 dark:text-slate-300">
            <div className="flex items-center justify-between gap-2">
              <button
                type="button"
                onClick={() => jumpToPage(pageNumber - 1)}
                  disabled={pageNumber <= 1}
                  className="rounded-lg border border-slate-300 px-2 py-1 disabled:opacity-50 dark:border-slate-700"
                >
                上一页
              </button>
              <span>
                页码 {pageNumber}
                {numPages > 0 ? ` / ${numPages}` : ''}
              </span>
                <button
                  type="button"
                  onClick={() => jumpToPage(pageNumber + 1)}
                  disabled={numPages > 0 && pageNumber >= numPages}
                  className="rounded-lg border border-slate-300 px-2 py-1 disabled:opacity-50 dark:border-slate-700"
              >
                下一页
              </button>
            </div>
            <div className="mt-2 flex items-center justify-between gap-2">
              <span className="text-[11px] text-slate-500 dark:text-slate-400">缩放 {Math.round(pdfZoom * 100)}%</span>
              <div className="flex items-center gap-1">
                <button
                  type="button"
                  onClick={() => setPDFZoom((value) => Math.max(0.65, Number((value - 0.1).toFixed(2))))}
                  className="rounded-md border border-slate-300 p-1 text-slate-600 hover:border-indigo-400 dark:border-slate-700 dark:text-slate-300"
                  title="缩小"
                >
                  <Minus className="h-3.5 w-3.5" />
                </button>
                <button
                  type="button"
                  onClick={() => setPDFZoom(0.9)}
                  className="rounded-md border border-slate-300 px-2 py-1 text-[11px] text-slate-600 hover:border-indigo-400 dark:border-slate-700 dark:text-slate-300"
                  title="重置缩放"
                >
                  重置
                </button>
                <button
                  type="button"
                  onClick={() => setPDFZoom((value) => Math.min(1.2, Number((value + 0.1).toFixed(2))))}
                  className="rounded-md border border-slate-300 p-1 text-slate-600 hover:border-indigo-400 dark:border-slate-700 dark:text-slate-300"
                  title="放大"
                >
                  <Plus className="h-3.5 w-3.5" />
                </button>
              </div>
            </div>
            <button
              type="button"
              onClick={() => void handleToggleTheme()}
              disabled={switchingTheme}
              className="mt-3 inline-flex w-full items-center justify-center gap-1.5 rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-xs text-slate-700 transition hover:border-indigo-400 disabled:opacity-60 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200"
            >
              {switchingTheme ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <SunMoon className="h-3.5 w-3.5" />}
              切换全局到{theme === 'dark' ? '浅色' : '深色'}模式
            </button>
          </div>
        </aside>

        <section className="grid min-h-0 gap-3 xl:grid-cols-[1.45fr_1fr]">
          <div className="min-h-0 rounded-2xl border border-slate-200 bg-white/85 p-3 dark:border-slate-700/80 dark:bg-slate-900/80">
            <h3 className="text-xs uppercase tracking-[0.16em] text-slate-500 dark:text-slate-400">PDF Original Text</h3>
            <div className="mt-3 min-h-0 rounded-xl border border-slate-200 bg-white/85 p-3 dark:border-slate-700 dark:bg-slate-900/70">
              {!selectedPaper && (
                <div className="flex min-h-[420px] flex-col items-center justify-center text-center">
                  <BookOpen className="mb-3 h-10 w-10 text-slate-400 dark:text-slate-500" />
                  <p className="text-sm text-slate-600 dark:text-slate-300">从右侧选择一篇论文开始阅读</p>
                </div>
              )}

              {selectedPaper && (loadingState || preparing) && (
                <div className="flex min-h-[420px] items-center justify-center text-sm text-slate-600 dark:text-slate-300">
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  正在准备阅读内容...
                </div>
              )}

              {selectedPaper && !loadingState && !preparing && isMissingPDF && (
                <div className="flex min-h-[420px] flex-col items-center justify-center text-center text-sm text-slate-600 dark:text-slate-300">
                  <p>{deepReadState?.parseError || '本地 PDF 不可用。'}</p>
                  <p className="mt-2 text-xs text-slate-500 dark:text-slate-400">
                    当前下载状态：{statusLabel(selectedPaper.downloadStatus)}
                    {selectedPaper.downloadError ? `（${selectedPaper.downloadError}）` : ''}
                  </p>
                  <button
                    type="button"
                    onClick={() => void handlePrepare()}
                    className="mt-3 inline-flex items-center gap-1.5 rounded-lg border border-slate-300 px-3 py-1.5 text-xs hover:border-indigo-400 dark:border-slate-600"
                  >
                    <RefreshCw className="h-3.5 w-3.5" />
                    刷新状态
                  </button>
                </div>
              )}

              {selectedPaper && !loadingState && !preparing && parseStatus === 'failed' && (
                <div className="flex min-h-[420px] flex-col items-center justify-center text-center text-sm text-rose-700 dark:text-rose-300">
                  <p>{deepReadState?.parseError || 'PDF 解析失败'}</p>
                  <button
                    type="button"
                    onClick={() => void handlePrepare()}
                    className="mt-3 inline-flex items-center gap-1.5 rounded-lg border border-rose-300 px-3 py-1.5 text-xs hover:bg-rose-50 dark:border-rose-500/40 dark:hover:bg-rose-500/10"
                  >
                    <RefreshCw className="h-3.5 w-3.5" />
                    重试解析
                  </button>
                </div>
              )}

              {selectedPaper && !loadingState && !preparing && deepReadState?.hasPdf && !isMissingPDF && parseStatus !== 'failed' && (
                <div className="space-y-2">
                  {pdfLoadError ? (
                    <div className="rounded-lg border border-rose-300 bg-rose-50 px-3 py-2 text-xs text-rose-700 dark:border-rose-400/50 dark:bg-rose-500/10 dark:text-rose-200">
                      PDF 渲染失败：{pdfLoadError}
                    </div>
                  ) : null}
                  <div
                    ref={pdfViewportRef}
                    onScroll={handlePDFViewportScroll}
                    className="max-h-[72vh] overflow-auto rounded-lg border border-slate-200 bg-slate-50 p-2 dark:border-slate-700 dark:bg-slate-950/70"
                  >
                    <Document
                      file={pdfFileInput}
                      loading={<div className="py-8 text-center text-xs text-slate-500 dark:text-slate-400">加载 PDF 中...</div>}
                      onLoadSuccess={({ numPages: totalPages }) => {
                        setNumPages(totalPages || 0);
                        setPageNumber((value) => Math.min(Math.max(value, 1), totalPages || 1));
                        setPDFLoadError('');
                      }}
                      onLoadError={(error) => setPDFLoadError(errorToUserMessage(error, '加载失败'))}
                    >
                      <div className="space-y-3">
                        {pdfTopSpacerHeight > 0 ? <div style={{ height: pdfTopSpacerHeight }} aria-hidden="true" /> : null}
                        {renderedPDFPages.map((page) => {
                          const isCurrent = page === pageNumber;
                          return (
                            <div
                              key={`pdf-page-${page}`}
                              ref={(element) => setPDFPageRef(page, element)}
                              className={`rounded-md border p-1 ${
                                isCurrent
                                  ? 'border-indigo-300 bg-indigo-50 dark:border-indigo-400 dark:bg-indigo-500/10'
                                  : 'border-transparent'
                              }`}
                            >
                              <div className="flex justify-center">
                                <Page
                                  pageNumber={page}
                                  width={readerPageWidth}
                                  renderAnnotationLayer={false}
                                  renderTextLayer={false}
                                />
                              </div>
                            </div>
                          );
                        })}
                        {pdfBottomSpacerHeight > 0 ? <div style={{ height: pdfBottomSpacerHeight }} aria-hidden="true" /> : null}
                      </div>
                    </Document>
                  </div>
                </div>
              )}
            </div>
          </div>

          <aside className="min-h-0 rounded-2xl border border-slate-200 bg-white/85 p-3 dark:border-slate-700/80 dark:bg-slate-900/80">
            <div className="space-y-3">
              <div className="rounded-xl border border-indigo-300/70 bg-indigo-50 p-3 dark:border-indigo-400/40 dark:bg-indigo-500/10">
                <h3 className="flex items-center gap-2 text-xs uppercase tracking-[0.16em] text-indigo-700 dark:text-indigo-200">
                  <Languages className="h-3.5 w-3.5" />
                  AI Translation
                </h3>
                <textarea
                  value={originalText}
                  onChange={(event) => setOriginalText(event.target.value)}
                  placeholder="选择章节后可编辑待翻译内容"
                  className="mt-2 min-h-[120px] w-full rounded-lg border border-slate-300 bg-white px-2.5 py-2 text-xs leading-6 outline-none dark:border-slate-700 dark:bg-slate-950/60"
                />
                <button
                  type="button"
                  onClick={() => void handleTranslate()}
                  disabled={isTranslating || !selectedPaper}
                  className="mt-2 inline-flex items-center gap-1.5 rounded-lg bg-indigo-600 px-3 py-1.5 text-xs font-medium text-white transition hover:bg-indigo-500 disabled:opacity-60"
                >
                  {isTranslating ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Languages className="h-3.5 w-3.5" />}
                  {isTranslating ? '处理中' : '生成翻译与摘要'}
                </button>
                {translationError ? (
                  <div className="mt-2 rounded-lg border border-rose-200 bg-rose-50 px-2.5 py-2 text-xs leading-5 text-rose-700 dark:border-rose-500/40 dark:bg-rose-500/10 dark:text-rose-200">
                    翻译失败：{translationError}
                    <br />
                    请检查弱模型 API Key、base URL、网络连通性，或缩短待翻译文本后重试。
                  </div>
                ) : null}
                <p className="mt-2 text-xs leading-6 text-slate-700 dark:text-slate-200">
                  {latestTranslation?.translatedText || '选中章节并执行翻译后，这里会显示最新译文。'}
                </p>
              </div>

              <div className="rounded-xl border border-violet-300/70 bg-violet-50 p-3 dark:border-violet-400/40 dark:bg-violet-500/10">
                <h3 className="flex items-center gap-2 text-xs uppercase tracking-[0.16em] text-violet-700 dark:text-violet-200">
                  <BookText className="h-3.5 w-3.5" />
                  Notes
                </h3>
                <textarea
                  value={noteDraft}
                  onChange={(event) => setNoteDraft(event.target.value)}
                  placeholder="记录你的阅读笔记"
                  className="mt-2 min-h-[90px] w-full rounded-lg border border-slate-300 bg-white px-2.5 py-2 text-xs leading-6 outline-none dark:border-slate-700 dark:bg-slate-950/60"
                />
                <button
                  type="button"
                  onClick={() => void handleSaveNote()}
                  disabled={savingNote || !selectedPaper}
                  className="mt-2 inline-flex items-center gap-1.5 rounded-lg border border-violet-400/70 px-3 py-1.5 text-xs text-violet-700 transition hover:bg-violet-100 disabled:opacity-60 dark:border-violet-500/60 dark:text-violet-100 dark:hover:bg-violet-500/10"
                >
                  {savingNote ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Save className="h-3.5 w-3.5" />}
                  {savingNote ? '保存中' : '保存笔记'}
                </button>
              </div>

              <div className="rounded-xl border border-slate-200 bg-white/85 p-3 dark:border-slate-700 dark:bg-slate-900/70">
                <h3 className="text-xs uppercase tracking-[0.16em] text-slate-500 dark:text-slate-400">History</h3>
                <div className="mt-2 max-h-36 space-y-2 overflow-y-auto">
                  {translationHistory.length > 0 ? (
                    translationHistory.map((record) => (
                      <div key={record.id} className="rounded-lg border border-slate-200 bg-slate-50 p-2 text-xs dark:border-slate-700 dark:bg-slate-950/70">
                        <p className="font-medium text-slate-700 dark:text-slate-200">{record.section}</p>
                        <p className="mt-1 line-clamp-3 text-slate-600 dark:text-slate-300">{record.summary || record.translatedText}</p>
                      </div>
                    ))
                  ) : (
                    <p className="text-xs text-slate-500 dark:text-slate-400">暂无翻译历史</p>
                  )}
                </div>
                <div className="mt-3 border-t border-slate-200 pt-2 dark:border-slate-700">
                  <p className="text-[11px] uppercase tracking-[0.16em] text-slate-500 dark:text-slate-400">Notes History</p>
                  <div className="mt-2 max-h-28 space-y-2 overflow-y-auto">
                    {noteHistory.length > 0 ? (
                      noteHistory.map((note) => (
                        <div key={note.id} className="rounded-lg border border-slate-200 bg-slate-50 p-2 text-xs dark:border-slate-700 dark:bg-slate-950/70">
                          <p className="font-medium text-slate-700 dark:text-slate-200">{note.section}</p>
                          <p className="mt-1 whitespace-pre-wrap leading-5 text-slate-600 dark:text-slate-300">{note.content}</p>
                        </div>
                      ))
                    ) : (
                      <p className="text-xs text-slate-500 dark:text-slate-400">暂无笔记</p>
                    )}
                  </div>
                </div>
              </div>
            </div>
          </aside>
        </section>

        <aside className="min-h-0 rounded-2xl border border-slate-200 bg-white/85 p-3 dark:border-slate-700/80 dark:bg-slate-900/80">
          <div className="rounded-xl border border-slate-200 bg-white/85 p-3 dark:border-slate-700/70 dark:bg-slate-900/75">
            <h3 className="text-xs uppercase tracking-[0.16em] text-slate-500 dark:text-slate-400">文件夹目录</h3>
            <p className="mt-1 truncate text-[11px] text-slate-500 dark:text-slate-400">{currentFolder?.path || '未选择目录'}</p>
            <div className="mt-3 max-h-44 overflow-y-auto">
              {folderTree.length > 0 ? (
                folderTree.map((node) => renderFolderNode(node))
              ) : (
                <p className="text-xs text-slate-500 dark:text-slate-400">{libraryLoading ? '加载目录中...' : '暂无目录'}</p>
              )}
            </div>
          </div>

          <div className="mt-3 min-h-0 rounded-xl border border-slate-200 bg-white/85 p-3 dark:border-slate-700/70 dark:bg-slate-900/75">
            <div className="flex items-center justify-between gap-2">
              <h3 className="flex items-center gap-2 text-xs uppercase tracking-[0.16em] text-slate-500 dark:text-slate-400">
                <FileText className="h-3.5 w-3.5" />
                论文卡片集
              </h3>
              <span className="text-[11px] text-slate-500 dark:text-slate-400">{filteredPapers.length}</span>
            </div>
            <button
              type="button"
              onClick={() => void handleRetryFolderDownloads()}
              disabled={!activeFolderId || retryingFolder}
              className="mt-2 inline-flex w-full items-center justify-center gap-1.5 rounded-lg border border-amber-300 bg-amber-50 px-2.5 py-1.5 text-[11px] text-amber-800 transition hover:bg-amber-100 disabled:opacity-60 dark:border-amber-500/40 dark:bg-amber-500/10 dark:text-amber-100 dark:hover:bg-amber-500/20"
            >
              {retryingFolder ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <RefreshCw className="h-3.5 w-3.5" />}
              重新下载当前文件夹未完成 PDF
            </button>
            <input
              type="text"
              value={libraryQuery}
              onChange={(event) => setLibraryQuery(event.target.value)}
              placeholder="搜索当前目录论文"
              className="mt-2 w-full rounded-lg border border-slate-300 bg-white px-2.5 py-2 text-xs outline-none focus:border-indigo-400 dark:border-slate-700 dark:bg-slate-950/60"
            />
            <div className="mt-3 max-h-[56vh] space-y-2 overflow-y-auto">
              {filteredPapers.length > 0 ? (
                filteredPapers.map((paper) => {
                  const active = selectedPaper?.id === paper.id;
                  const retrying = retryingPaperId === paper.id;
                  const attaching = attachingPaperId === paper.id;
                  const status = paper.downloadStatus.toLowerCase();
                  const canRepairDownload = status === 'failed' || status === 'queued' || status === 'downloading';
                  return (
                    <article
                      key={paper.id}
                      className={`rounded-xl border p-3 text-xs transition ${
                        active
                          ? 'border-indigo-300 bg-indigo-100/80 dark:border-indigo-400/70 dark:bg-indigo-500/15'
                          : 'border-slate-200 bg-white hover:border-indigo-300 dark:border-slate-700 dark:bg-slate-950/70 dark:hover:border-indigo-500/40'
                      }`}
                    >
                      <button
                        type="button"
                        onClick={() => setSelectedPaper(paper)}
                        className="w-full text-left"
                      >
                        <h4 className="line-clamp-2 font-medium text-slate-800 dark:text-slate-100">{paper.title}</h4>
                        <p className="mt-1 line-clamp-1 text-[11px] text-slate-500 dark:text-slate-400">
                          {paper.journal || '发表未提供'} · {paper.year || '年份未知'}
                        </p>
                      </button>
                      <div className="mt-2 flex items-center justify-between gap-2">
                        <span className={`rounded-full border px-2 py-0.5 text-[10px] ${statusStyle(paper.downloadStatus)}`}>
                          {statusLabel(paper.downloadStatus)}
                        </span>
                        {canRepairDownload ? (
                          <div className="flex items-center gap-1">
                            <button
                              type="button"
                              onClick={() => void handleRetryDownload(paper)}
                              disabled={retrying}
                              className="inline-flex items-center gap-1 rounded-lg border border-rose-300 px-2 py-0.5 text-[10px] text-rose-700 hover:bg-rose-50 disabled:opacity-60 dark:border-rose-500/40 dark:text-rose-200 dark:hover:bg-rose-500/10"
                            >
                              {retrying ? <Loader2 className="h-3 w-3 animate-spin" /> : <RefreshCw className="h-3 w-3" />}
                              重试
                            </button>
                            {status === 'failed' ? (
                              <button
                                type="button"
                                onClick={() => {
                                  setManualURLPanelPaperId((prev) => (prev === paper.id ? null : paper.id));
                                  setManualURLDrafts((prev) => ({
                                    ...prev,
                                    [paper.id]: (prev[paper.id] || paper.url || '').trim(),
                                  }));
                                }}
                                className="rounded-lg border border-slate-300 px-2 py-0.5 text-[10px] text-slate-600 hover:border-indigo-300 hover:text-indigo-700 dark:border-slate-600 dark:text-slate-200 dark:hover:border-indigo-500/60 dark:hover:text-indigo-300"
                              >
                                手动链接
                              </button>
                            ) : null}
                            <button
                              type="button"
                              onClick={() => void handleAttachLocalPDF(paper)}
                              disabled={attaching}
                              className="inline-flex items-center gap-1 rounded-lg border border-emerald-300 px-2 py-0.5 text-[10px] text-emerald-700 hover:bg-emerald-50 disabled:opacity-60 dark:border-emerald-500/40 dark:text-emerald-200 dark:hover:bg-emerald-500/10"
                            >
                              {attaching ? <Loader2 className="h-3 w-3 animate-spin" /> : <FileText className="h-3 w-3" />}
                              本地PDF
                            </button>
                          </div>
                        ) : null}
                      </div>
                      {paper.downloadError && (
                        <p className="mt-1 line-clamp-2 text-[10px] text-rose-700 dark:text-rose-300">{paper.downloadError}</p>
                      )}
                      {manualURLPanelPaperId === paper.id && status === 'failed' && (
                        <div className="mt-2 rounded-lg border border-slate-200 bg-slate-50 p-2 dark:border-slate-700 dark:bg-slate-900/70">
                          <p className="text-[10px] text-slate-600 dark:text-slate-300">填写可访问的 http/https PDF 链接</p>
                          <input
                            ref={manualURLPanelPaperId === paper.id ? manualURLInputRef : undefined}
                            type="text"
                            value={manualURLDrafts[paper.id] || ''}
                            onChange={(event) => handleManualURLDraftChange(paper.id, event.target.value)}
                            placeholder="https://..."
                            className="mt-1 w-full rounded border border-slate-300 bg-white px-2 py-1 text-[11px] outline-none focus:border-indigo-400 dark:border-slate-600 dark:bg-slate-950"
                          />
                          <button
                            type="button"
                            onClick={() => void handleRetryWithManualURL(paper)}
                            disabled={submittingManualURLPaperId === paper.id}
                            className="mt-2 inline-flex items-center gap-1 rounded-md bg-indigo-600 px-2 py-1 text-[10px] text-white disabled:opacity-60"
                          >
                            {submittingManualURLPaperId === paper.id ? (
                              <Loader2 className="h-3 w-3 animate-spin" />
                            ) : (
                              <ExternalLink className="h-3 w-3" />
                            )}
                            使用该链接重试
                          </button>
                        </div>
                      )}
                    </article>
                  );
                })
              ) : (
                <p className="py-8 text-center text-xs text-slate-500 dark:text-slate-400">
                  {libraryLoading ? (
                    <span className="inline-flex items-center gap-1.5">
                      <Loader2 className="h-3.5 w-3.5 animate-spin" />
                      正在加载论文库...
                    </span>
                  ) : (
                    '当前目录暂无论文'
                  )}
                </p>
              )}
            </div>
          </div>
        </aside>
      </div>
    </div>
  );
}
