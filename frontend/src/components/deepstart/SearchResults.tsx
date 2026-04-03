import React from 'react';
import { SearchResult } from '../../pages/DeepStart';

interface SearchResultsProps {
  results: SearchResult[];
  selectedPapers: Set<string>;
  onPaperSelect: (paperId: string, selected: boolean) => void;
}

export const SearchResults: React.FC<SearchResultsProps> = ({
  results,
  selectedPapers,
  onPaperSelect,
}) => {
  if (results.length === 0) {
    return (
      <div className="text-center py-8 text-gray-500">
        No results found. Try adjusting your search query.
      </div>
    );
  }

  return (
    <div className="space-y-3">
      <div className="text-sm text-gray-600 mb-2">
        Found {results.length} papers
      </div>
      {results.map((paper) => (
        <div
          key={paper.ID}
          className={`p-4 border rounded-lg transition-colors cursor-pointer ${
            selectedPapers.has(paper.ID)
              ? 'border-blue-500 bg-blue-50'
              : 'border-gray-200 hover:border-gray-300'
          }`}
          onClick={() => onPaperSelect(paper.ID, !selectedPapers.has(paper.ID))}
        >
          <div className="flex items-start gap-3">
            <input
              type="checkbox"
              checked={selectedPapers.has(paper.ID)}
              onChange={(e) => {
                e.stopPropagation();
                onPaperSelect(paper.ID, e.target.checked);
              }}
              className="mt-1 w-4 h-4 text-blue-600 rounded focus:ring-blue-500"
            />
            <div className="flex-1 min-w-0">
              <h3 className="font-medium text-gray-900 line-clamp-2">
                {paper.Title}
              </h3>
              <p className="text-sm text-gray-600 mt-1">
                {paper.Authors}
              </p>
              <div className="flex items-center gap-3 mt-2 text-xs text-gray-500">
                <span>{paper.Year}</span>
                <span>•</span>
                <span>{paper.Journal || paper.Source}</span>
                {paper.Citations > 0 && (
                  <>
                    <span>•</span>
                    <span>{paper.Citations} citations</span>
                  </>
                )}
              </div>
              {paper.Abstract && (
                <p className="text-sm text-gray-600 mt-2 line-clamp-2">
                  {paper.Abstract}
                </p>
              )}
            </div>
          </div>
        </div>
      ))}
    </div>
  );
};
