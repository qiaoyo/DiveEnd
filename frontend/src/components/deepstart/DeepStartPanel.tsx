import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Loader2, Search, Sparkles } from 'lucide-react';
import { startDeepStartSession } from '../../lib/backend';
import { useAppStore } from '../../stores/appStore';

export function DeepStartPanel() {
  const navigate = useNavigate();
  const { activeFolderId, folders, setActiveDeepStartSession, setError } = useAppStore();

  const [isStarting, setIsStarting] = useState(false);
  const [newPrompt, setNewPrompt] = useState('');

  const handleStartSession = async () => {
    const prompt = newPrompt.trim();
    if (!prompt) {
      setError('请先输入研究方向或问题');
      return;
    }

    setIsStarting(true);
    try {
      const detail = await startDeepStartSession(
        prompt,
        activeFolderId || folders[0]?.id || ''
      );
      setActiveDeepStartSession(detail);
      // 创建成功后跳转到会话详情页面
      navigate(`/session/${detail.summary.id}`);
    } catch (error) {
      setError(error instanceof Error ? error.message : '创建探索会话失败');
    } finally {
      setIsStarting(false);
    }
  };

  return (
    <div className="flex h-full flex-col bg-gradient-to-br from-blue-50 to-cyan-50 dark:from-stone-900 dark:to-stone-950">
      <div className="flex-1 flex flex-col items-center justify-center p-8">
        <div className="w-full max-w-2xl">
          <div className="text-center mb-8">
            <div className="inline-flex items-center justify-center w-16 h-16 rounded-full bg-blue-100 dark:bg-blue-950 mb-4">
              <Sparkles className="h-8 w-8 text-blue-600 dark:text-blue-400" />
            </div>
            <h1 className="text-3xl font-bold text-stone-900 dark:text-stone-100 mb-2">
              开始新的探索
            </h1>
            <p className="text-stone-600 dark:text-stone-400">
              输入你的研究问题，AI将帮你检索和筛选相关论文
            </p>
          </div>

          <div className="rounded-2xl border-2 border-blue-200 bg-white p-6 dark:border-blue-800 dark:bg-stone-800 shadow-xl">
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-stone-700 dark:text-stone-300 mb-2">
                  研究方向或问题
                </label>
                <textarea
                  value={newPrompt}
                  onChange={(e) => setNewPrompt(e.target.value)}
                  placeholder="例如：我想梳理这两年具身智能领域的论文"
                  rows={4}
                  className="w-full rounded-xl border border-stone-200 bg-white px-4 py-3 text-sm shadow-sm outline-none transition focus:border-blue-500 dark:border-stone-700 dark:bg-stone-900 dark:text-stone-100"
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-stone-700 dark:text-stone-300 mb-2">
                  示例问题
                </label>
                <div className="flex flex-wrap gap-2">
                  {[
                    '我想梳理这两年具身智能领域的论文',
                    'LLM code agent 的代表性论文和 tech report',
                    'VLA (Vision-Language-Action) 的最近进展',
                  ].map((example) => (
                    <button
                      key={example}
                      onClick={() => setNewPrompt(example)}
                      className="rounded-full border border-stone-200 bg-stone-50 px-3 py-1.5 text-xs text-stone-600 transition hover:border-blue-400 hover:bg-blue-50 dark:border-stone-700 dark:bg-stone-900 dark:text-stone-400 dark:hover:border-blue-600"
                    >
                      {example}
                    </button>
                  ))}
                </div>
              </div>

              <button
                onClick={() => void handleStartSession()}
                disabled={isStarting || !newPrompt.trim()}
                className="w-full flex items-center justify-center gap-2 rounded-xl bg-blue-600 px-6 py-3 text-base font-medium text-white transition hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
              >
                {isStarting ? (
                  <>
                    <Loader2 className="h-5 w-5 animate-spin" />
                    正在创建探索会话...
                  </>
                ) : (
                  <>
                    <Search className="h-5 w-5" />
                    开始探索
                  </>
                )}
              </button>
            </div>
          </div>

          <div className="mt-6 text-center">
            <button
              onClick={() => navigate('/history')}
              className="text-sm text-stone-600 hover:text-blue-600 dark:text-stone-400 dark:hover:text-blue-400 transition"
            >
              查看探索历史 →
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
