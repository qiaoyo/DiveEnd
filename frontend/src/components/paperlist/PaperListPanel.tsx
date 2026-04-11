import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  ChevronLeft,
  ChevronRight,
  FileText,
  FolderOpen,
  FolderPlus,
  Search,
  Trash2,
} from 'lucide-react';
import { createFolder, deletePaper, getPapers } from '../../lib/backend';
import { useAppStore } from '../../stores/appStore';

export function PaperListPanel() {
  const navigate = useNavigate();
  const {
    activeFolderId,
    folders,
    papers,
    rightPanelCollapsed,
    selectedPaper,
    setActiveFolderId,
    setError,
    setFolders,
    setPapers,
    setSelectedPaper,
    toggleRightPanel,
  } = useAppStore();
  const [searchTerm, setSearchTerm] = useState('');

  const filteredPapers = papers.filter(
    (paper) =>
      paper.title.toLowerCase().includes(searchTerm.toLowerCase()) ||
      paper.authors.toLowerCase().includes(searchTerm.toLowerCase())
  );

  const handleFolderClick = async (folderId: string) => {
    try {
      const nextPapers = await getPapers(folderId);
      setActiveFolderId(folderId);
      setPapers(nextPapers);
    } catch (error) {
      setError(error instanceof Error ? error.message : '加载文件夹失败');
    }
  };

  const handleCreateFolder = async () => {
    const name = window.prompt('输入文件夹名称');
    if (!name?.trim()) return;

    try {
      const folder = await createFolder(name.trim());
      const nextFolders = [...folders.filter((item) => item.id !== folder.id), folder].sort((a, b) =>
        a.createdAt.localeCompare(b.createdAt)
      );
      setFolders(nextFolders);
    } catch (error) {
      setError(error instanceof Error ? error.message : '创建文件夹失败');
    }
  };

  const handlePaperClick = (paper: typeof papers[number]) => {
    setSelectedPaper(paper);
    navigate('/deepread');
  };

  const handleDelete = async (id: string) => {
    try {
      await deletePaper(id);
      const nextPapers = await getPapers(activeFolderId);
      setPapers(nextPapers);
      if (selectedPaper?.id === id) {
        setSelectedPaper(null);
      }
    } catch (error) {
      setError(error instanceof Error ? error.message : '删除论文失败');
    }
  };

  if (rightPanelCollapsed) {
    return (
      <div className="flex h-full flex-col items-center justify-between border-l border-stone-200 bg-[#efe9de] py-4 dark:border-stone-800 dark:bg-[#1c1c1a]">
        <div className="flex flex-col items-center gap-4">
          <div className="rounded-2xl bg-white/80 p-3 shadow-sm dark:bg-stone-900/80">
            <FolderOpen className="h-5 w-5 text-emerald-600" />
          </div>
          <span className="text-xs uppercase tracking-[0.3em] text-stone-500 [writing-mode:vertical-rl] dark:text-stone-400">
            Library
          </span>
        </div>
        <button
          onClick={toggleRightPanel}
          className="rounded-xl border border-stone-200 p-2 text-stone-600 transition hover:bg-white dark:border-stone-700 dark:text-stone-300 dark:hover:bg-stone-900"
          title="展开论文库"
        >
          <ChevronLeft className="h-4 w-4" />
        </button>
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col border-l border-stone-200 bg-[#efe9de] dark:border-stone-800 dark:bg-[#1c1c1a]">
      <div className="border-b border-stone-200 p-4 dark:border-stone-800">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <FolderOpen className="h-5 w-5 text-emerald-600" />
            <span className="font-semibold">论文库</span>
            <span className="text-sm text-stone-500 dark:text-stone-400">({papers.length})</span>
          </div>
          <div className="flex gap-2">
            <button
              onClick={() => void handleCreateFolder()}
              className="rounded-xl border border-stone-200 p-2 text-stone-600 transition hover:bg-white dark:border-stone-700 dark:text-stone-300 dark:hover:bg-stone-900"
              title="新建文件夹"
            >
              <FolderPlus className="h-4 w-4" />
            </button>
            <button
              onClick={toggleRightPanel}
              className="rounded-xl border border-stone-200 p-2 text-stone-600 transition hover:bg-white dark:border-stone-700 dark:text-stone-300 dark:hover:bg-stone-900"
              title="收起论文库"
            >
              <ChevronRight className="h-4 w-4" />
            </button>
          </div>
        </div>
      </div>

      <div className="border-b border-stone-200 p-3 dark:border-stone-800">
        <div className="mb-3 flex flex-wrap gap-2">
          {folders.map((folder) => (
            <button
              key={folder.id}
              onClick={() => void handleFolderClick(folder.id)}
              className={`rounded-full px-3 py-1.5 text-sm transition ${
                activeFolderId === folder.id
                  ? 'bg-emerald-600 text-white'
                  : 'bg-white text-stone-600 hover:text-emerald-700 dark:bg-stone-900 dark:text-stone-300'
              }`}
            >
              {folder.name}
            </button>
          ))}
        </div>

        <div className="relative">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-stone-400" />
          <input
            type="text"
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            placeholder="搜索论文..."
            className="w-full rounded-xl border border-stone-200 bg-white py-2 pl-9 pr-3 text-sm outline-none transition focus:border-emerald-500 dark:border-stone-700 dark:bg-stone-900"
          />
        </div>
      </div>

      <div className="flex-1 overflow-y-auto">
        {filteredPapers.length === 0 ? (
          <div className="flex h-full flex-col items-center justify-center p-4 text-center text-stone-500 dark:text-stone-400">
            <FileText className="mb-2 h-10 w-10 opacity-50" />
            <p className="text-sm">暂无论文</p>
            <p className="mt-1 text-xs">在 DeepStart 中搜索并导入</p>
          </div>
        ) : (
          <div className="space-y-2 p-3">
            {filteredPapers.map((paper) => (
              <div
                key={paper.id}
                onClick={() => handlePaperClick(paper)}
                className={`group cursor-pointer rounded-3xl border p-4 transition ${
                  selectedPaper?.id === paper.id
                    ? 'border-emerald-500 bg-emerald-50 dark:bg-emerald-950/20'
                    : 'border-stone-200 bg-white hover:border-emerald-300 dark:border-stone-800 dark:bg-stone-900/70'
                }`}
              >
                <div className="flex items-start gap-3">
                  <FileText className="mt-1 h-4 w-4 shrink-0 text-stone-400" />
                  <div className="min-w-0 flex-1">
                    <p className="line-clamp-2 text-sm font-medium leading-6">{paper.title}</p>
                    <p className="mt-1 line-clamp-1 text-xs text-stone-500 dark:text-stone-400">
                      {paper.authors}
                    </p>
                    <p className="mt-2 text-[11px] uppercase tracking-[0.18em] text-stone-400 dark:text-stone-500">
                      {paper.journal || '未知来源'}
                    </p>
                  </div>
                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      void handleDelete(paper.id);
                    }}
                    className="rounded-lg p-1 text-stone-400 opacity-0 transition hover:bg-stone-100 hover:text-red-500 group-hover:opacity-100 dark:hover:bg-stone-800"
                    title="删除论文"
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
