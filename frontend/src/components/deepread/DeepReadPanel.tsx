import { useEffect, useMemo, useState } from 'react';
import { BookOpen, BookText, ExternalLink, Languages, Loader2, MessageSquare } from 'lucide-react';
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

  const latestTranslation = useMemo(() => translations[0], [translations]);

  if (!selectedPaper) {
    return (
      <div className="flex h-full flex-col items-center justify-center text-center">
        <BookOpen className="mb-4 h-14 w-14 text-slate-300 dark:text-slate-700" />
        <h3 className="text-lg font-semibold">DeepRead - 沉浸阅读</h3>
        <p className="mt-2 max-w-md text-sm leading-7 text-slate-500 dark:text-slate-300">
          先从右侧论文库选中一篇论文，再进入三栏阅读与翻译工作流。
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
    <div className="flex h-full min-h-0 flex-col overflow-hidden bg-slate-50 text-slate-900 dark:bg-slate-950 dark:text-slate-100">
      <div className="border-b border-slate-200/80 bg-white/70 px-5 py-3 backdrop-blur-md dark:border-slate-700/50 dark:bg-slate-900/55">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <p className="text-xs uppercase tracking-[0.2em] text-slate-500 dark:text-slate-300">DeepRead</p>
            <h2 className="max-w-3xl truncate text-base font-semibold">{selectedPaper.title}</h2>
          </div>
          {selectedPaper.url && (
            <a
              href={selectedPaper.url}
              target="_blank"
              rel="noreferrer"
              className="inline-flex items-center gap-1.5 rounded-xl border border-slate-200 bg-white/80 px-3 py-1.5 text-sm text-slate-600 transition hover:border-indigo-300 hover:text-indigo-700 dark:border-slate-700 dark:bg-slate-900/80 dark:text-slate-200 dark:hover:border-indigo-500/60 dark:hover:text-indigo-300"
            >
              打开原文
              <ExternalLink className="h-4 w-4" />
            </a>
          )}
        </div>
      </div>

      <div className="grid min-h-0 flex-1 gap-3 p-3 lg:grid-cols-[0.95fr_1.6fr_1.05fr]">
        <aside className="de-glass min-h-0 rounded-2xl p-4">
          <h3 className="text-xs uppercase tracking-[0.18em] text-slate-500 dark:text-slate-300">Outline</h3>
          <div className="mt-3 space-y-2">
            {['Title', 'Abstract', 'Introduction', 'Method', 'Experiments', 'Conclusion'].map((item) => (
              <button
                key={item}
                onClick={() => setSection(item)}
                className={`w-full rounded-lg border px-3 py-2 text-left text-sm transition ${
                  section === item
                    ? 'border-indigo-400 bg-indigo-50 dark:border-indigo-500/50 dark:bg-indigo-500/15'
                    : 'border-slate-200 bg-white/80 hover:border-indigo-300 dark:border-slate-700 dark:bg-slate-900/80 dark:hover:border-indigo-500/50'
                }`}
              >
                {item}
              </button>
            ))}
          </div>

          <div className="mt-4 rounded-xl border border-slate-200 bg-white/75 p-3 text-xs leading-6 text-slate-500 dark:border-slate-700 dark:bg-slate-900/75 dark:text-slate-300">
            <p>作者：{selectedPaper.authors || '未知'}</p>
            <p>来源：{selectedPaper.journal || '未知来源'}</p>
            <p>年份：{selectedPaper.year || '未知'}</p>
          </div>
        </aside>

        <section className="de-glass min-h-0 rounded-2xl p-4">
          <h3 className="text-xs uppercase tracking-[0.18em] text-slate-500 dark:text-slate-300">PDF Original Text</h3>
          <div className="mt-3 flex min-h-0 flex-col gap-3">
            <input
              type="text"
              value={section}
              onChange={(event) => setSection(event.target.value)}
              placeholder="章节名"
              className="rounded-xl border border-slate-200 bg-white/85 px-3 py-2 text-sm outline-none transition focus:border-indigo-500 dark:border-slate-700 dark:bg-slate-900/85"
            />
            <textarea
              value={originalText}
              onChange={(event) => setOriginalText(event.target.value)}
              placeholder="粘贴论文原文内容..."
              className="de-reading min-h-[320px] flex-1 rounded-xl border border-slate-200 bg-white/85 px-4 py-3 text-sm leading-7 outline-none transition focus:border-indigo-500 dark:border-slate-700 dark:bg-slate-900/85"
            />
            <button
              onClick={() => void handleTranslate()}
              disabled={isTranslating}
              className="inline-flex w-fit items-center gap-2 rounded-xl bg-indigo-600 px-4 py-2 text-sm font-medium text-white transition hover:bg-indigo-500 disabled:cursor-not-allowed disabled:opacity-60"
            >
              {isTranslating ? <Loader2 className="h-4 w-4 animate-spin" /> : <Languages className="h-4 w-4" />}
              {isTranslating ? '翻译中...' : '生成翻译与摘要'}
            </button>
          </div>
        </section>

        <aside className="de-glass min-h-0 rounded-2xl p-4">
          <div className="space-y-3">
            <div className="rounded-xl border border-indigo-300 bg-indigo-50/80 p-3 dark:border-indigo-500/50 dark:bg-indigo-500/15">
              <h3 className="flex items-center gap-2 text-xs uppercase tracking-[0.18em] text-indigo-700 dark:text-indigo-200">
                <Languages className="h-3.5 w-3.5" />
                AI Translation
              </h3>
              <p className="mt-2 text-sm leading-7 text-slate-700 dark:text-slate-100">
                {latestTranslation?.translatedText || '划选或粘贴段落后，这里会显示最新翻译。'}
              </p>
            </div>

            <div className="rounded-xl border border-violet-300 bg-violet-50/80 p-3 dark:border-violet-500/50 dark:bg-violet-500/15">
              <h3 className="flex items-center gap-2 text-xs uppercase tracking-[0.18em] text-violet-700 dark:text-violet-200">
                <BookText className="h-3.5 w-3.5" />
                Notes
              </h3>
              <p className="mt-2 text-sm leading-7 text-slate-700 dark:text-slate-100">
                {latestTranslation?.summary || '翻译摘要会自动生成，可继续整理成你的阅读笔记。'}
              </p>
            </div>

            <div className="rounded-xl border border-slate-200 bg-white/80 p-3 dark:border-slate-700 dark:bg-slate-900/80">
              <h3 className="flex items-center gap-2 text-xs uppercase tracking-[0.18em] text-slate-500 dark:text-slate-300">
                <MessageSquare className="h-3.5 w-3.5" />
                History
              </h3>
              <div className="mt-2 max-h-[280px] space-y-2 overflow-y-auto">
                {translations.length > 0 ? (
                  translations.map((record) => (
                    <div key={record.id} className="rounded-lg border border-slate-200 bg-slate-50/80 p-2.5 text-xs dark:border-slate-700 dark:bg-slate-800/70">
                      <p className="font-medium">{record.section}</p>
                      <p className="mt-1 line-clamp-3 text-slate-600 dark:text-slate-200">
                        {record.summary || record.translatedText}
                      </p>
                    </div>
                  ))
                ) : (
                  <p className="text-xs text-slate-500 dark:text-slate-300">暂无翻译历史</p>
                )}
              </div>
            </div>
          </div>
        </aside>
      </div>
    </div>
  );
}
