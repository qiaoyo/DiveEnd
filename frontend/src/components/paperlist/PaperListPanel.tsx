import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  ChevronLeft,
  ChevronRight,
  FileText,
  FolderOpen,
  FolderPlus,
  MoveRight,
  Pencil,
  Search,
  Trash2,
} from 'lucide-react';
import {
  createFolder,
  deleteFolderNode,
  deletePaper,
  getFolders,
  getPapers,
  moveFolderNode,
  movePapersToFolder,
  movePaperToFolder,
  renameFolderNode,
} from '../../lib/backend';
import { errorToUserMessage } from '../../lib/errors';
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
  const [creatingFolder, setCreatingFolder] = useState(false);
  const [newFolderName, setNewFolderName] = useState('');
  const [renamingFolderId, setRenamingFolderId] = useState('');
  const [renameFolderName, setRenameFolderName] = useState('');
  const [movingFolderId, setMovingFolderId] = useState('');
  const [moveFolderParentId, setMoveFolderParentId] = useState('');
  const [movingPaperId, setMovingPaperId] = useState('');
  const [moveTargetFolderId, setMoveTargetFolderId] = useState('');
  const [selectedPaperIds, setSelectedPaperIds] = useState<string[]>([]);
  const [batchTargetFolderId, setBatchTargetFolderId] = useState('');
  const [folderDeleteCandidateId, setFolderDeleteCandidateId] = useState('');

  const filteredPapers = papers.filter(
    (paper) =>
      paper.title.toLowerCase().includes(searchTerm.toLowerCase()) ||
      paper.authors.toLowerCase().includes(searchTerm.toLowerCase())
  );
  const selectedPaperIdSet = new Set(selectedPaperIds);
  const selectedPapers = papers.filter((paper) => selectedPaperIdSet.has(paper.id));
  const batchTargetOptions = folders.filter((folder) => selectedPapers.some((paper) => paper.folderId !== folder.id));
  const resolvedBatchTargetFolderId = batchTargetFolderId || batchTargetOptions[0]?.id || '';
  const allFilteredSelected =
    filteredPapers.length > 0 && filteredPapers.every((paper) => selectedPaperIdSet.has(paper.id));

  const handleFolderClick = async (folderId: string) => {
    try {
      const nextPapers = await getPapers(folderId);
      setActiveFolderId(folderId);
      setPapers(nextPapers);
      setSelectedPaperIds([]);
      setBatchTargetFolderId('');
      setMovingPaperId('');
      setMoveTargetFolderId('');
    } catch (error) {
      setError(errorToUserMessage(error, '加载文件夹失败'));
    }
  };

  const handleCreateFolder = async () => {
    const name = newFolderName.trim();
    if (!name) {
      setCreatingFolder(true);
      return;
    }

    try {
      const folder = await createFolder(name.trim());
      const nextFolders = [...folders.filter((item) => item.id !== folder.id), folder].sort((a, b) =>
        a.createdAt.localeCompare(b.createdAt)
      );
      setFolders(nextFolders);
      setNewFolderName('');
      setCreatingFolder(false);
    } catch (error) {
      setError(errorToUserMessage(error, '创建文件夹失败'));
    }
  };

  const handlePaperClick = (paper: typeof papers[number]) => {
    setSelectedPaper(paper);
    navigate('/deepread');
  };

  const startRenameFolder = (folder: typeof folders[number]) => {
    if (folder.isSystem) {
      return;
    }
    setRenamingFolderId(folder.id);
    setRenameFolderName(folder.name);
    setMovingFolderId('');
    setMoveFolderParentId('');
    setCreatingFolder(false);
  };

  const handleRenameFolder = async () => {
    const folder = folders.find((item) => item.id === renamingFolderId);
    if (!folder || folder.isSystem) {
      setRenamingFolderId('');
      setRenameFolderName('');
      return;
    }
    const name = renameFolderName.trim();
    if (!name) {
      setError('请填写新的文件夹名称');
      return;
    }

    try {
      await renameFolderNode({ folderId: folder.id, name });
      setFolders(await getFolders());
      if (activeFolderId) {
        const nextPapers = await getPapers(activeFolderId);
        setPapers(nextPapers);
        if (selectedPaper) {
          setSelectedPaper(nextPapers.find((paper) => paper.id === selectedPaper.id) ?? selectedPaper);
        }
      }
      setRenamingFolderId('');
      setRenameFolderName('');
    } catch (error) {
      setError(errorToUserMessage(error, '重命名文件夹失败'));
    }
  };

  const descendantFolderIds = (folderId: string): Set<string> => {
    const ids = new Set<string>();
    const walk = (id: string) => {
      ids.add(id);
      for (const folder of folders) {
        if ((folder.parentId ?? '') === id) {
          walk(folder.id);
        }
      }
    };
    walk(folderId);
    return ids;
  };

  const validParentFolderIdsForMove = (folderId: string): string[] => {
    const blocked = descendantFolderIds(folderId);
    return ['', ...folders.filter((folder) => !blocked.has(folder.id)).map((folder) => folder.id)];
  };

  const startMoveFolder = (folder: typeof folders[number]) => {
    if (folder.isSystem) {
      return;
    }
    const currentParentId = folder.parentId ?? '';
    const nextParentId =
      validParentFolderIdsForMove(folder.id).find((parentId) => parentId !== currentParentId) ?? currentParentId;
    setMovingFolderId(folder.id);
    setMoveFolderParentId(nextParentId);
    setRenamingFolderId('');
    setRenameFolderName('');
    setCreatingFolder(false);
  };

  const handleMoveFolder = async () => {
    const folder = folders.find((item) => item.id === movingFolderId);
    if (!folder || folder.isSystem) {
      setMovingFolderId('');
      setMoveFolderParentId('');
      return;
    }

    try {
      await moveFolderNode({ folderId: folder.id, parentId: moveFolderParentId });
      setFolders(await getFolders());
      if (activeFolderId) {
        const nextPapers = await getPapers(activeFolderId);
        setPapers(nextPapers);
        if (selectedPaper) {
          setSelectedPaper(nextPapers.find((paper) => paper.id === selectedPaper.id) ?? selectedPaper);
        }
      }
      setMovingFolderId('');
      setMoveFolderParentId('');
    } catch (error) {
      setError(errorToUserMessage(error, '移动文件夹失败'));
    }
  };

  const requestDeleteFolder = (folder: typeof folders[number]) => {
    if (folder.isSystem) {
      return;
    }
    setFolderDeleteCandidateId(folder.id);
  };

  const handleConfirmDeleteFolder = async () => {
    const folder = folders.find((item) => item.id === folderDeleteCandidateId);
    if (!folder || folder.isSystem) {
      setFolderDeleteCandidateId('');
      return;
    }

    try {
      const idsToRemove = descendantFolderIds(folder.id);
      await deleteFolderNode(folder.id);

      const nextFolders = folders.filter((item) => !idsToRemove.has(item.id));
      setFolders(nextFolders);
      setFolderDeleteCandidateId('');

      if (idsToRemove.has(activeFolderId)) {
        const nextFolder = nextFolders.find((item) => item.isSystem) ?? nextFolders[0];
        setSelectedPaper(null);
        setSelectedPaperIds([]);
        setBatchTargetFolderId('');
        if (nextFolder) {
          setActiveFolderId(nextFolder.id);
          setPapers(await getPapers(nextFolder.id));
        } else {
          setActiveFolderId('');
          setPapers([]);
        }
      }
    } catch (error) {
      setError(errorToUserMessage(error, '删除文件夹失败'));
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await deletePaper(id);
      const nextPapers = await getPapers(activeFolderId);
      setPapers(nextPapers);
      setSelectedPaperIds((ids) => ids.filter((paperId) => paperId !== id));
      if (selectedPaper?.id === id) {
        setSelectedPaper(null);
      }
    } catch (error) {
      setError(errorToUserMessage(error, '删除论文失败'));
    }
  };

  const startMovePaper = (paper: typeof papers[number]) => {
    const targetFolder = folders.find((folder) => folder.id !== paper.folderId);
    setMovingPaperId(paper.id);
    setMoveTargetFolderId(targetFolder?.id ?? '');
  };

  const handleMovePaper = async (paperId: string) => {
    const targetFolderId = moveTargetFolderId.trim();
    if (!targetFolderId) {
      setError('请选择目标文件夹');
      return;
    }

    try {
      await movePaperToFolder(paperId, targetFolderId);
      const nextPapers = await getPapers(activeFolderId);
      setPapers(nextPapers);
      setSelectedPaperIds((ids) => ids.filter((id) => id !== paperId));
      if (selectedPaper?.id === paperId) {
        setSelectedPaper(nextPapers.find((paper) => paper.id === paperId) ?? null);
      }
      setMovingPaperId('');
      setMoveTargetFolderId('');
    } catch (error) {
      setError(errorToUserMessage(error, '移动论文失败'));
    }
  };

  const togglePaperSelection = (paperId: string) => {
    setSelectedPaperIds((ids) => {
      if (ids.includes(paperId)) {
        return ids.filter((id) => id !== paperId);
      }
      return [...ids, paperId];
    });
  };

  const toggleAllFilteredPapers = () => {
    if (allFilteredSelected) {
      const filteredIds = new Set(filteredPapers.map((paper) => paper.id));
      setSelectedPaperIds((ids) => ids.filter((id) => !filteredIds.has(id)));
      return;
    }
    setSelectedPaperIds((ids) => {
      const next = new Set(ids);
      for (const paper of filteredPapers) {
        next.add(paper.id);
      }
      return [...next];
    });
  };

  const clearBatchSelection = () => {
    setSelectedPaperIds([]);
    setBatchTargetFolderId('');
  };

  const handleBatchMovePapers = async () => {
    const ids = selectedPaperIds.filter((paperId) => papers.some((paper) => paper.id === paperId));
    if (ids.length === 0) {
      setError('请先选择要移动的论文');
      return;
    }
    const targetFolderId = resolvedBatchTargetFolderId.trim();
    if (!targetFolderId) {
      setError('请选择批量移动目标文件夹');
      return;
    }

    try {
      await movePapersToFolder(ids, targetFolderId);
      const nextPapers = await getPapers(activeFolderId);
      setPapers(nextPapers);
      if (selectedPaper && ids.includes(selectedPaper.id)) {
        setSelectedPaper(nextPapers.find((paper) => paper.id === selectedPaper.id) ?? null);
      }
      clearBatchSelection();
    } catch (error) {
      setError(errorToUserMessage(error, '批量移动论文失败'));
    }
  };

  const folderDeleteCandidate = folders.find((folder) => folder.id === folderDeleteCandidateId);
  const folderDeleteIds = folderDeleteCandidate ? descendantFolderIds(folderDeleteCandidate.id) : new Set<string>();
  const folderDeleteChildCount = Math.max(0, folderDeleteIds.size - 1);
  const loadedDeletePaperCount = papers.filter((paper) => folderDeleteIds.has(paper.folderId)).length;

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
    <div className="relative flex h-full flex-col border-l border-stone-200 bg-[#efe9de] dark:border-stone-800 dark:bg-[#1c1c1a]">
      {folderDeleteCandidate && (
        <div className="absolute inset-0 z-40 flex items-center justify-center bg-stone-950/35 px-4 backdrop-blur-sm">
          <div className="w-full max-w-sm rounded-3xl border border-stone-200 bg-white p-5 shadow-2xl dark:border-stone-700 dark:bg-stone-950">
            <div className="flex items-center gap-2 text-red-600 dark:text-red-300">
              <Trash2 className="h-5 w-5" />
              <h3 className="font-semibold">确认删除文件夹</h3>
            </div>
            <p className="mt-3 text-sm leading-6 text-stone-600 dark:text-stone-300">
              将删除「{folderDeleteCandidate.path || folderDeleteCandidate.name}」及其子文件夹。这个操作会影响该目录下的论文和本地文件管理记录。
            </p>
            <div className="mt-3 rounded-2xl border border-red-100 bg-red-50 p-3 text-xs leading-6 text-red-700 dark:border-red-500/30 dark:bg-red-500/10 dark:text-red-200">
              <div>子文件夹数量：{folderDeleteChildCount}</div>
              <div>当前已加载列表中受影响论文：{loadedDeletePaperCount}</div>
              <div>删除后如果当前正在查看该目录，会自动切换回系统文件夹。</div>
            </div>
            <div className="mt-4 flex justify-end gap-2">
              <button
                type="button"
                onClick={() => setFolderDeleteCandidateId('')}
                className="rounded-xl border border-stone-200 bg-white px-3 py-2 text-sm text-stone-600 transition hover:bg-stone-50 dark:border-stone-700 dark:bg-stone-900 dark:text-stone-200 dark:hover:bg-stone-800"
              >
                取消
              </button>
              <button
                type="button"
                onClick={() => void handleConfirmDeleteFolder()}
                className="rounded-xl bg-red-600 px-3 py-2 text-sm font-semibold text-white transition hover:bg-red-500"
              >
                确认删除
              </button>
            </div>
          </div>
        </div>
      )}
      <div className="border-b border-stone-200 p-4 dark:border-stone-800">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <FolderOpen className="h-5 w-5 text-emerald-600" />
            <span className="font-semibold">论文库</span>
            <span className="text-sm text-stone-500 dark:text-stone-400">({papers.length})</span>
          </div>
          <div className="flex gap-2">
            <button
              onClick={() => setCreatingFolder((value) => !value)}
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
        {creatingFolder && (
          <form
            className="mb-3 rounded-2xl border border-emerald-200 bg-white/80 p-2 dark:border-emerald-500/40 dark:bg-stone-900/80"
            onSubmit={(event) => {
              event.preventDefault();
              void handleCreateFolder();
            }}
          >
            <input
              value={newFolderName}
              onChange={(event) => setNewFolderName(event.target.value)}
              autoFocus
              placeholder="新文件夹名称"
              className="w-full rounded-xl border border-stone-200 bg-white px-3 py-2 text-sm outline-none focus:border-emerald-500 dark:border-stone-700 dark:bg-stone-950"
            />
            <div className="mt-2 flex gap-2">
              <button
                type="submit"
                className="rounded-lg bg-emerald-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-emerald-500"
              >
                创建
              </button>
              <button
                type="button"
                onClick={() => {
                  setCreatingFolder(false);
                  setNewFolderName('');
                }}
                className="rounded-lg border border-stone-200 px-3 py-1.5 text-xs text-stone-600 hover:bg-white dark:border-stone-700 dark:text-stone-300 dark:hover:bg-stone-900"
              >
                取消
              </button>
            </div>
          </form>
        )}
        {renamingFolderId && (
          <form
            className="mb-3 rounded-2xl border border-amber-200 bg-white/80 p-2 dark:border-amber-500/40 dark:bg-stone-900/80"
            onSubmit={(event) => {
              event.preventDefault();
              void handleRenameFolder();
            }}
          >
            <input
              value={renameFolderName}
              onChange={(event) => setRenameFolderName(event.target.value)}
              autoFocus
              placeholder="新的文件夹名称"
              className="w-full rounded-xl border border-stone-200 bg-white px-3 py-2 text-sm outline-none focus:border-amber-500 dark:border-stone-700 dark:bg-stone-950"
            />
            <div className="mt-2 flex gap-2">
              <button
                type="submit"
                className="rounded-lg bg-amber-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-amber-500"
              >
                保存
              </button>
              <button
                type="button"
                onClick={() => {
                  setRenamingFolderId('');
                  setRenameFolderName('');
                }}
                className="rounded-lg border border-stone-200 px-3 py-1.5 text-xs text-stone-600 hover:bg-white dark:border-stone-700 dark:text-stone-300 dark:hover:bg-stone-900"
              >
                取消
              </button>
            </div>
          </form>
        )}
        {movingFolderId && (
          <form
            className="mb-3 rounded-2xl border border-sky-200 bg-white/80 p-2 dark:border-sky-500/40 dark:bg-stone-900/80"
            onSubmit={(event) => {
              event.preventDefault();
              void handleMoveFolder();
            }}
          >
            <label className="mb-1 block text-[11px] font-medium uppercase tracking-[0.18em] text-sky-700 dark:text-sky-300">
              移动文件夹到
            </label>
            <select
              value={moveFolderParentId}
              onChange={(event) => setMoveFolderParentId(event.target.value)}
              aria-label="移动文件夹目标父级"
              className="w-full rounded-xl border border-stone-200 bg-white px-3 py-2 text-sm outline-none focus:border-sky-500 dark:border-stone-700 dark:bg-stone-950"
            >
              {validParentFolderIdsForMove(movingFolderId).map((parentId) => {
                const parent = folders.find((folder) => folder.id === parentId);
                return (
                  <option key={parentId || 'root'} value={parentId}>
                    {parent ? parent.path || parent.name : '根目录'}
                  </option>
                );
              })}
            </select>
            <div className="mt-2 flex gap-2">
              <button
                type="submit"
                className="rounded-lg bg-sky-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-sky-500"
              >
                移动
              </button>
              <button
                type="button"
                onClick={() => {
                  setMovingFolderId('');
                  setMoveFolderParentId('');
                }}
                className="rounded-lg border border-stone-200 px-3 py-1.5 text-xs text-stone-600 hover:bg-white dark:border-stone-700 dark:text-stone-300 dark:hover:bg-stone-900"
              >
                取消
              </button>
            </div>
          </form>
        )}
        <div className="mb-3 flex flex-wrap gap-2">
          {folders.map((folder) => (
            <div
              key={folder.id}
              className={`inline-flex items-center overflow-hidden rounded-full text-sm transition ${
                activeFolderId === folder.id
                  ? 'bg-emerald-600 text-white'
                  : 'bg-white text-stone-600 hover:text-emerald-700 dark:bg-stone-900 dark:text-stone-300'
              }`}
            >
              <button
                type="button"
                onClick={() => void handleFolderClick(folder.id)}
                className="px-3 py-1.5"
                title={`打开文件夹 ${folder.path || folder.name}`}
              >
                {folder.name}
              </button>
              {!folder.isSystem && (
                <>
                  <button
                    type="button"
                    onClick={() => startRenameFolder(folder)}
                    className={`px-2 py-1.5 transition ${
                      activeFolderId === folder.id
                        ? 'text-white/80 hover:bg-emerald-700 hover:text-white'
                        : 'text-stone-400 hover:bg-amber-50 hover:text-amber-600 dark:hover:bg-amber-950/30'
                    }`}
                    title={`重命名文件夹 ${folder.name}`}
                  >
                    <Pencil className="h-3.5 w-3.5" />
                  </button>
                  <button
                    type="button"
                    onClick={() => startMoveFolder(folder)}
                    className={`px-2 py-1.5 transition ${
                      activeFolderId === folder.id
                        ? 'text-white/80 hover:bg-emerald-700 hover:text-white'
                        : 'text-stone-400 hover:bg-sky-50 hover:text-sky-600 dark:hover:bg-sky-950/30'
                    }`}
                    title={`移动文件夹 ${folder.name}`}
                  >
                    <MoveRight className="h-3.5 w-3.5" />
                  </button>
                  <button
                    type="button"
                    onClick={() => requestDeleteFolder(folder)}
                    className={`px-2 py-1.5 transition ${
                      activeFolderId === folder.id
                        ? 'text-white/80 hover:bg-emerald-700 hover:text-white'
                        : 'text-stone-400 hover:bg-red-50 hover:text-red-500 dark:hover:bg-red-950/30'
                    }`}
                    title={`删除文件夹 ${folder.name}`}
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </button>
                </>
              )}
            </div>
          ))}
        </div>

        {filteredPapers.length > 0 && (
          <div className="mb-3 rounded-2xl border border-stone-200 bg-white/70 p-2 dark:border-stone-700 dark:bg-stone-900/70">
            <div className="mb-2 flex items-center justify-between gap-2">
              <button
                type="button"
                onClick={toggleAllFilteredPapers}
                className="rounded-lg border border-stone-200 px-3 py-1.5 text-xs font-medium text-stone-600 hover:bg-white dark:border-stone-700 dark:text-stone-300 dark:hover:bg-stone-800"
              >
                {allFilteredSelected ? '取消全选' : '全选当前结果'}
              </button>
              <span className="text-xs text-stone-500 dark:text-stone-400">已选 {selectedPapers.length} 篇</span>
            </div>
            {selectedPapers.length > 0 && (
              <div className="flex flex-col gap-2">
                <select
                  value={resolvedBatchTargetFolderId}
                  onChange={(event) => setBatchTargetFolderId(event.target.value)}
                  aria-label="批量移动目标文件夹"
                  className="w-full rounded-xl border border-stone-200 bg-white px-3 py-2 text-sm outline-none focus:border-emerald-500 dark:border-stone-700 dark:bg-stone-950"
                >
                  <option value="">选择批量移动目标</option>
                  {batchTargetOptions.map((folder) => (
                    <option key={folder.id} value={folder.id}>
                      {folder.path || folder.name}
                    </option>
                  ))}
                </select>
                <div className="flex gap-2">
                  <button
                    type="button"
                    onClick={() => void handleBatchMovePapers()}
                    className="rounded-lg bg-emerald-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-emerald-500 disabled:cursor-not-allowed disabled:bg-stone-300"
                    disabled={!resolvedBatchTargetFolderId}
                  >
                    批量移动
                  </button>
                  <button
                    type="button"
                    onClick={clearBatchSelection}
                    className="rounded-lg border border-stone-200 px-3 py-1.5 text-xs text-stone-600 hover:bg-white dark:border-stone-700 dark:text-stone-300 dark:hover:bg-stone-800"
                  >
                    清空选择
                  </button>
                </div>
              </div>
            )}
          </div>
        )}

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
                  <label
                    className="mt-1 flex shrink-0 cursor-pointer items-center"
                    onClick={(event) => event.stopPropagation()}
                    title={`选择论文 ${paper.title}`}
                  >
                    <input
                      type="checkbox"
                      checked={selectedPaperIdSet.has(paper.id)}
                      onChange={() => togglePaperSelection(paper.id)}
                      aria-label={`选择论文 ${paper.title}`}
                      className="h-4 w-4 rounded border-stone-300 text-emerald-600 focus:ring-emerald-500"
                    />
                  </label>
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
                  <div className="flex shrink-0 gap-1">
                    {folders.some((folder) => folder.id !== paper.folderId) && (
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          startMovePaper(paper);
                        }}
                        className="rounded-lg p-1 text-stone-400 opacity-0 transition hover:bg-stone-100 hover:text-emerald-600 group-hover:opacity-100 dark:hover:bg-stone-800"
                        title="移动论文"
                      >
                        <FolderOpen className="h-3.5 w-3.5" />
                      </button>
                    )}
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
                {movingPaperId === paper.id && (
                  <form
                    className="mt-3 rounded-2xl border border-emerald-200 bg-emerald-50/70 p-2 dark:border-emerald-500/40 dark:bg-emerald-950/20"
                    onClick={(event) => event.stopPropagation()}
                    onSubmit={(event) => {
                      event.preventDefault();
                      void handleMovePaper(paper.id);
                    }}
                  >
                    <label className="mb-1 block text-[11px] font-medium uppercase tracking-[0.18em] text-emerald-700 dark:text-emerald-300">
                      移动到
                    </label>
                    <select
                      value={moveTargetFolderId}
                      onChange={(event) => setMoveTargetFolderId(event.target.value)}
                      aria-label="移动目标文件夹"
                      className="w-full rounded-xl border border-emerald-200 bg-white px-3 py-2 text-sm outline-none focus:border-emerald-500 dark:border-emerald-700 dark:bg-stone-950"
                    >
                      <option value="">选择目标文件夹</option>
                      {folders
                        .filter((folder) => folder.id !== paper.folderId)
                        .map((folder) => (
                          <option key={folder.id} value={folder.id}>
                            {folder.path || folder.name}
                          </option>
                        ))}
                    </select>
                    <div className="mt-2 flex gap-2">
                      <button
                        type="submit"
                        className="rounded-lg bg-emerald-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-emerald-500"
                      >
                        移动
                      </button>
                      <button
                        type="button"
                        onClick={() => {
                          setMovingPaperId('');
                          setMoveTargetFolderId('');
                        }}
                        className="rounded-lg border border-stone-200 px-3 py-1.5 text-xs text-stone-600 hover:bg-white dark:border-stone-700 dark:text-stone-300 dark:hover:bg-stone-900"
                      >
                        取消
                      </button>
                    </div>
                  </form>
                )}
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
