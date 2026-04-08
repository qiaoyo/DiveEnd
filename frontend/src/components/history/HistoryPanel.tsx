import { useNavigate } from 'react-router-dom';
import { useAppStore } from '../../stores/appStore';
import { formatDistanceToNow } from 'date-fns';
import { zhCN } from 'date-fns/locale';

export function HistoryPanel() {
  const navigate = useNavigate();
  const { deepStartSessions } = useAppStore();

  const handleSessionClick = (sessionId: string) => {
    navigate(`/session/${sessionId}`);
  };

  return (
    <div className="flex h-full flex-col bg-stone-50 dark:bg-stone-900">
      <div className="border-b border-stone-200 bg-white px-6 py-4 dark:border-stone-700 dark:bg-stone-800">
        <div className="flex items-center justify-between">
          <div>
            <h2 className="text-2xl font-bold text-stone-900 dark:text-stone-100">
              探索历史
            </h2>
            <p className="text-sm text-stone-600 dark:text-stone-400 mt-1">
              共 {deepStartSessions.length} 个探索会话
            </p>
          </div>
          <button
            onClick={() => navigate('/deepstart')}
            className="rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 transition-colors"
          >
            新建探索
          </button>
        </div>
      </div>

      <div className="flex-1 overflow-y-auto p-6">
        {deepStartSessions.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-full text-center">
            <div className="text-6xl mb-4">📭</div>
            <h3 className="text-xl font-semibold text-stone-700 dark:text-stone-300 mb-2">
              还没有探索记录
            </h3>
            <p className="text-stone-600 dark:text-stone-400 mb-6">
              开始你的第一次论文探索之旅
            </p>
            <button
              onClick={() => navigate('/deepstart')}
              className="rounded-lg bg-blue-600 px-6 py-3 text-white font-medium hover:bg-blue-700 transition-colors"
            >
              开始探索
            </button>
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {deepStartSessions.map((session) => (
              <button
                key={session.id}
                onClick={() => handleSessionClick(session.id)}
                className="group rounded-xl border-2 border-stone-200 bg-white p-5 text-left transition-all hover:border-blue-400 hover:shadow-lg dark:border-stone-700 dark:bg-stone-800 dark:hover:border-blue-600"
              >
                <div className="flex items-start justify-between mb-3">
                  <h3 className="font-semibold text-stone-900 dark:text-stone-100 line-clamp-2 flex-1">
                    {session.title}
                  </h3>
                </div>

                <p className="text-sm text-stone-600 dark:text-stone-400 line-clamp-2 mb-3">
                  {session.rootPrompt}
                </p>

                <div className="flex items-center justify-between text-xs text-stone-500 dark:text-stone-500">
                  <span>
                    {formatDistanceToNow(new Date(session.updatedAt), {
                      addSuffix: true,
                      locale: zhCN,
                    })}
                  </span>
                  <span className="opacity-0 group-hover:opacity-100 transition-opacity">
                    点击继续 →
                  </span>
                </div>
              </button>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
