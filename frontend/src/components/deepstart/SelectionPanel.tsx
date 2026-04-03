import React from 'react';
import { SearchResult, SelectionStep } from '../../pages/DeepStart';

interface SelectionPanelProps {
  papers: SearchResult[];
  selectedPapers: Set<string>;
  selectionSteps: SelectionStep[];
  onPaperSelect: (paperId: string, selected: boolean) => void;
  onImport: () => void;
  onBack: () => void;
  isLoading: boolean;
  targetFolder: string;
  onTargetFolderChange: (folderId: string) => void;
}

export const SelectionPanel: React.FC<SelectionPanelProps> = ({
  papers,
  selectedPapers,
  selectionSteps,
  onPaperSelect,
  onImport,
  onBack,
  isLoading,
  targetFolder,
  onTargetFolderChange,
}) => {
  return (
    <div className="h-full flex flex-col">
      <div className="flex items-center justify-between mb-4">
        <div>
          <h2 className="text-lg font-semibold">Select Papers</h2>
          <p className="text-sm text-gray-600">
            {selectedPapers.size} of {papers.length} papers selected
          </p>
        </div>
        <div className="flex gap-2">
          <button
            onClick={onBack}
            className="px-4 py-2 text-gray-600 hover:text-gray-800 transition-colors"
          >
            ← Back
          </button>
          <button
            onClick={onImport}
            disabled={selectedPapers.size === 0 || isLoading}
            className="px-4 py-2 bg-green-500 text-white rounded hover:bg-green-600 disabled:bg-gray-300 disabled:cursor-not-allowed transition-colors"
          >
            {isLoading ? 'Importing...' : `Import ${selectedPapers.size} Papers →`}
          </button>
        </div>
      </div>

      {/* Selection Steps */}
      {selectionSteps.length > 0 && (
        <div className="mb-4 p-3 bg-blue-50 rounded-lg">
          <h3 className="text-sm font-medium text-blue-900 mb-2">Selection Path</h3>
          <div className="space-y-1">
            {selectionSteps.map((step, index) => (
              <div key={step.id} className="flex items-center text-sm">
                <span className="w-6 h-6 rounded-full bg-blue-500 text-white flex items-center justify-center text-xs mr-2">
                  {index + 1}
                </span>
                <span className="text-gray-700">
                  {step.dimension}: {step.selected.join(', ')}
                </span>
                <span className="ml-auto text-gray-500">
                  {step.remainingCount} papers
                </span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Target Folder Selection */}
      <div className="mb-4">
        <label className="block text-sm font-medium text-gray-700 mb-1">
          Target Folder for Import
        </label>
        <select
          value={targetFolder}
          onChange={(e) => onTargetFolderChange(e.target.value)}
          className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent"
        >
          <option value="">Select a folder...</option>
          {/* TODO: Load folders from backend */}
          <option value="inbox">Inbox</option>
        </select>
      </div>

      {/* Papers List */}
      <div className="flex-1 overflow-auto border rounded-lg">
        <div className="divide-y">
          {papers.map((paper) => (
            <div
              key={paper.ID}
              className={`p-4 hover:bg-gray-50 cursor-pointer transition-colors ${
                selectedPapers.has(paper.ID) ? 'bg-blue-50 border-l-4 border-l-blue-500' : ''
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
      </div>
    </div>
  );
};