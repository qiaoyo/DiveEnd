import { useEffect, useState } from 'react';
import { BookOpen, BookText, ExternalLink, Languages, Loader2 } from 'lucide-react';
import { getTranslations, translatePaperSection } from '../../lib/backend';
import { useAppStore } from '../../stores/appStore';

export function DeepReadPanel() {
  const {
    isTranslating,
    prependTranslation,
    selectedPaper,
    setError,
    setIsTranslating,
    setTranslations,
    translations,
  } = useAppStore();
  const [section, setSection] = useState('Abstract');
  const [originalText, setOriginalText] = useState('');

  useEffect(() => {
    if (!selectedPaper) {
      setTranslations([]);
      setSection('Abstract');
      setOriginalText('');
      return;
    }

    setSection('Abstract');
    setOriginalText(selectedPaper.abstract || '');

    const loadTranslations = async () => {
      try {
        const history = await getTranslations(selectedPaper.id);
        setTranslations(history);
      } catch (error) {
        setError(error instanceof Error ? error.message : '加载翻译历史失败');
      }
    };

    void loadTranslations();
  }, [selectedPaper, setError, setTranslations]);

  if (!selectedPaper) {
    return (
      <div className="flex h-full flex-col items-center justify-center bg-[#f9f6f0] p-8 text-center dark:bg-[#141414]">
        <BookOpen className="mb-4 h-16 w-16 text-stone-300 dark:text-stone-700" />
        <h3 className="text-lg font-medium">DeepRead - 论文阅读</h3>
        <p className="mt-2 max-w-md text-sm leading-7 text-stone-500 dark:text-stone-400">
          先从右侧论文库选中一篇论文，再把需要精读的章节粘贴进来。翻译和摘要会被保存在当前论文的历史记录里。
        </p>
      </div>
    );
  }

  const handleTranslate = async () => {
    if (!originalText.trim()) {
      setError('请先粘贴要翻译的章节内容');
      return;
    }

    setIsTranslating(true);
    try {
      const record = await translatePaperSection(selectedPaper.id, section, originalText);
      prependTranslation(record);
      setSection('');
      setOriginalText('');
    } catch (error) {
      setError(error instanceof Error ? error.message : '翻译失败');
    } finally {
      setIsTranslating(false);
    }
  };

  return (
    <div className="flex h-full min-h-0 flex-col bg-[#f9f6f0] dark:bg-[#141414]">
      <div className="border-b border-stone-200 p-6 dark:border-stone-800">
        <div className="flex items-center justify-between">
          <div>
            <h2 className="flex items-center gap-2 text-lg font-semibold">
              <BookOpen className="h-5 w-5 text-emerald-600" />
              DeepRead - 论文阅读
            </h2>
            <p className="mt-1 text-sm text-stone-500 dark:text-stone-400">
              手动粘贴章节内容，保存 AI 翻译和摘要历史。
            </p>
          </div>
          {selectedPaper.url && (
            <a
              href={selectedPaper.url}
              target="_blank"
              rel="noreferrer"
              className="flex items-center gap-2 rounded-2xl border border-stone-200 bg-white px-4 py-2 text-sm text-stone-600 transition hover:border-emerald-400 hover:text-emerald-700 dark:border-stone-700 dark:bg-stone-900 dark:text-stone-300"
            >
              打开原文
              <ExternalLink className="h-4 w-4" />
            </a>
          )}
        </div>
      </div>

      <div className="flex min-h-0 flex-1 overflow-hidden">
        <div className="flex-1 overflow-y-auto border-r border-stone-200 p-6 dark:border-stone-800">
          <div className="rounded-[2rem] border border-stone-200 bg-white p-6 shadow-sm dark:border-stone-800 dark:bg-stone-900/70">
            <p className="text-xs uppercase tracking-[0.2em] text-stone-500 dark:text-stone-400">
              当前论文
            </p>
            <h1 className="mt-2 text-2xl font-semibold leading-9">{selectedPaper.title}</h1>
            <p className="mt-3 text-sm text-stone-600 dark:text-stone-300">{selectedPaper.authors}</p>
            <p className="mt-2 text-xs uppercase tracking-[0.2em] text-stone-400 dark:text-stone-500">
              {selectedPaper.journal || '未知来源'} · {selectedPaper.year || '年份未知'}
            </p>
            <p className="mt-5 text-sm leading-7 text-stone-600 dark:text-stone-300">
              {selectedPaper.abstract || '暂无摘要。'}
            </p>
          </div>

          <div className="mt-6 rounded-[2rem] border border-stone-200 bg-white p-6 shadow-sm dark:border-stone-800 dark:bg-stone-900/70">
            <div className="flex items-center gap-2">
              <BookText className="h-5 w-5 text-emerald-600" />
              <h3 className="text-lg font-semibold">手动章节翻译</h3>
            </div>

            <div className="mt-5 space-y-4">
              <div>
                <label className="mb-1 block text-sm font-medium">章节名</label>
                <input
                  type="text"
                  value={section}
                  onChange={(e) => setSection(e.target.value)}
                  placeholder="例如：Abstract / Method / Discussion"
                  className="w-full rounded-2xl border border-stone-200 bg-stone-50 px-4 py-3 text-sm outline-none transition focus:border-emerald-500 dark:border-stone-700 dark:bg-stone-950"
                />
              </div>

              <div>
                <label className="mb-1 block text-sm font-medium">原文内容</label>
                <textarea
                  value={originalText}
                  onChange={(e) => setOriginalText(e.target.value)}
                  placeholder="粘贴论文章节内容..."
                  className="min-h-[240px] w-full rounded-2xl border border-stone-200 bg-stone-50 px-4 py-3 text-sm leading-7 outline-none transition focus:border-emerald-500 dark:border-stone-700 dark:bg-stone-950"
                />
              </div>

              <button
                onClick={() => void handleTranslate()}
                disabled={isTranslating}
                className="flex items-center gap-2 rounded-2xl bg-emerald-600 px-5 py-3 text-sm font-medium text-white transition hover:bg-emerald-700 disabled:cursor-not-allowed disabled:opacity-60"
              >
                {isTranslating ? (
                  <>
                    <Loader2 className="h-4 w-4 animate-spin" />
                    翻译中...
                  </>
                ) : (
                  <>
                    <Languages className="h-4 w-4" />
                    生成翻译与摘要
                  </>
                )}
              </button>
            </div>
          </div>
        </div>

        <aside className="w-[380px] max-w-[42%] overflow-y-auto p-6">
          <div className="flex items-center gap-2">
            <Languages className="h-5 w-5 text-emerald-600" />
            <h3 className="text-lg font-semibold">翻译历史</h3>
          </div>

          <div className="mt-4 space-y-4">
            {translations.length > 0 ? (
              translations.map((record) => (
                <div
                  key={record.id}
                  className="rounded-[1.75rem] border border-stone-200 bg-white p-5 shadow-sm dark:border-stone-800 dark:bg-stone-900/70"
                >
                  <div>
                    <h4 className="font-semibold">{record.section}</h4>
                    <p className="mt-1 text-xs uppercase tracking-[0.18em] text-stone-400 dark:text-stone-500">
                      {new Date(record.createdAt).toLocaleString()}
                    </p>
                  </div>

                  <div className="mt-4 space-y-4 text-sm leading-7">
                    <div>
                      <p className="mb-1 text-xs uppercase tracking-[0.18em] text-stone-400 dark:text-stone-500">
                        原文
                      </p>
                      <p className="text-stone-600 dark:text-stone-300">{record.originalText}</p>
                    </div>

                    <div>
                      <p className="mb-1 text-xs uppercase tracking-[0.18em] text-stone-400 dark:text-stone-500">
                        中文翻译
                      </p>
                      <p className="text-stone-700 dark:text-stone-200">{record.translatedText}</p>
                    </div>

                    {record.summary && (
                      <div className="rounded-2xl bg-stone-50 p-4 dark:bg-stone-950">
                        <p className="mb-1 text-xs uppercase tracking-[0.18em] text-stone-400 dark:text-stone-500">
                          阅读摘要
                        </p>
                        <p className="text-stone-600 dark:text-stone-300">{record.summary}</p>
                      </div>
                    )}
                  </div>
                </div>
              ))
            ) : (
              <div className="rounded-[1.75rem] border border-dashed border-stone-300 bg-white/60 p-6 text-sm leading-7 text-stone-500 dark:border-stone-700 dark:bg-stone-900/40 dark:text-stone-400">
                当前论文还没有翻译历史。你可以先从摘要开始，系统会把每次翻译结果保存到这里。
              </div>
            )}
          </div>
        </aside>
      </div>
    </div>
  );
}
