import React, { useState } from 'react';

interface SearchPanelProps {
  onSearch: (query: string, limit: number) => void;
  isLoading: boolean;
}

export const SearchPanel: React.FC<SearchPanelProps> = ({
  onSearch,
  isLoading,
}) => {
  const [query, setQuery] = useState('');
  const [limit, setLimit] = useState(20);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (query.trim()) {
      onSearch(query.trim(), limit);
    }
  };

  const quickQueries = [
    'transformer architecture',
    'large language models',
    'computer vision',
    'reinforcement learning',
    'neural networks',
  ];

  return (
    <div className="max-w-2xl mx-auto">
      <form onSubmit={handleSubmit} className="mb-6">
        <div className="mb-4">
          <label className="block text-sm font-medium text-gray-700 mb-2">
            Search Query
          </label>
          <input
            type="text"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Enter keywords, topics, or research questions..."
            className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent"
            disabled={isLoading}
          />
        </div>

        <div className="mb-4">
          <label className="block text-sm font-medium text-gray-700 mb-2">
            Results Limit: {limit}
          </label>
          <input
            type="range"
            min="10"
            max="100"
            step="10"
            value={limit}
            onChange={(e) => setLimit(parseInt(e.target.value))}
            className="w-full"
            disabled={isLoading}
          />
          <div className="flex justify-between text-xs text-gray-500 mt-1">
            <span>10</span>
            <span>50</span>
            <span>100</span>
          </div>
        </div>

        <button
          type="submit"
          disabled={isLoading || !query.trim()}
          className="w-full py-2 px-4 bg-blue-500 text-white rounded-lg hover:bg-blue-600 disabled:bg-gray-300 disabled:cursor-not-allowed flex items-center justify-center"
        >
          {isLoading ? (
            <>
              <svg className="animate-spin -ml-1 mr-3 h-5 w-5 text-white" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
              </svg>
              Searching...
            </>
          ) : (
            'Search Papers'
          )}
        </button>
      </form>

      <div className="border-t pt-4">
        <h3 className="text-sm font-medium text-gray-700 mb-2">Quick Queries</h3>
        <div className="flex flex-wrap gap-2">
          {quickQueries.map((q) => (
            <button
              key={q}
              onClick={() => setQuery(q)}
              className="px-3 py-1 text-sm bg-gray-100 hover:bg-gray-200 rounded-full text-gray-700 transition-colors"
            >
              {q}
            </button>
          ))}
        </div>
      </div>
    </div>
  );
};
