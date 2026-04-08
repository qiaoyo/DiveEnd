import { useNavigate } from 'react-router-dom';
import { useAppStore } from '../../stores/appStore';

export function MainPanel() {
  const navigate = useNavigate();
  const { deepStartSessions } = useAppStore();

  const hasHistory = deepStartSessions.length > 0;

  return (
    <div className="flex h-full flex-col bg-gradient-to-br from-stone-50 to-stone-100 dark:from-stone-900 dark:to-stone-950">
      <div className="flex-1 flex flex-col items-center justify-center p-8">
        <div className="w-full max-w-4xl">
          <h1 className="text-4xl font-bold text-center mb-12 text-stone-800 dark:text-stone-100">
            DiveEnd
          </h1>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
            {/* 探索历史 */}
            <button
              onClick={() => navigate('/history')}
              disabled={!hasHistory}
              className="group relative overflow-hidden rounded-2xl border-2 border-stone-200 bg-white p-8 text-left transition-all hover:border-stone-400 hover:shadow-xl disabled:opacity-50 disabled:cursor-not-allowed dark:border-stone-700 dark:bg-stone-800 dark:hover:border-stone-500"
            >
              <div className="absolute inset-0 bg-gradient-to-br from-amber-50 to-orange-50 opacity-0 transition-opacity group-hover:opacity-100 dark:from-amber-950/20 dark:to-orange-950/20" />
              <div className="relative">
                <div className="text-5xl mb-4">📚</div>
                <h3 className="text-xl font-semibold mb-2 text-stone-900 dark:text-stone-100">
                  探索历史
                </h3>
                <p className="text-sm text-stone-600 dark:text-stone-400">
                  {hasHistory
                    ? `${deepStartSessions.length} 个会话记录`
                    : '暂无历史记录'}
                </p>
              </div>
            </button>

            {/* DeepStart */}
            <button
              onClick={() => navigate('/deepstart')}
              className="group relative overflow-hidden rounded-2xl border-2 border-blue-200 bg-white p-8 text-left transition-all hover:border-blue-400 hover:shadow-xl dark:border-blue-800 dark:bg-stone-800 dark:hover:border-blue-600"
            >
              <div className="absolute inset-0 bg-gradient-to-br from-blue-50 to-cyan-50 opacity-0 transition-opacity group-hover:opacity-100 dark:from-blue-950/20 dark:to-cyan-950/20" />
              <div className="relative">
                <div className="text-5xl mb-4">🚀</div>
                <h3 className="text-xl font-semibold mb-2 text-stone-900 dark:text-stone-100">
                  DeepStart
                </h3>
                <p className="text-sm text-stone-600 dark:text-stone-400">
                  开始新的论文探索
                </p>
              </div>
            </button>

            {/* DeepRead */}
            <button
              onClick={() => navigate('/deepread')}
              className="group relative overflow-hidden rounded-2xl border-2 border-purple-200 bg-white p-8 text-left transition-all hover:border-purple-400 hover:shadow-xl dark:border-purple-800 dark:bg-stone-800 dark:hover:border-purple-600"
            >
              <div className="absolute inset-0 bg-gradient-to-br from-purple-50 to-pink-50 opacity-0 transition-opacity group-hover:opacity-100 dark:from-purple-950/20 dark:to-pink-950/20" />
              <div className="relative">
                <div className="text-5xl mb-4">📖</div>
                <h3 className="text-xl font-semibold mb-2 text-stone-900 dark:text-stone-100">
                  DeepRead
                </h3>
                <p className="text-sm text-stone-600 dark:text-stone-400">
                  沉浸式论文阅读
                </p>
              </div>
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
