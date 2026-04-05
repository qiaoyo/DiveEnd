import React, { useState } from 'react';
import type { CategoryNode } from '../../types';

interface CategoryTreeProps {
  categories: CategoryNode[];
  onSelect: (categoryIds: string[]) => void;
  onBack: () => void;
}

export const CategoryTree: React.FC<CategoryTreeProps> = ({
  categories,
  onSelect,
  onBack,
}) => {
  const [selectedCategories, setSelectedCategories] = useState<Set<string>>(new Set());
  const [expandedNodes, setExpandedNodes] = useState<Set<string>>(new Set(
    categories.map(c => c.id)
  ));

  const toggleCategory = (categoryId: string) => {
    setSelectedCategories(prev => {
      const next = new Set(prev);
      if (next.has(categoryId)) {
        next.delete(categoryId);
      } else {
        next.add(categoryId);
      }
      return next;
    });
  };

  const toggleExpand = (nodeId: string, e: React.MouseEvent) => {
    e.stopPropagation();
    setExpandedNodes(prev => {
      const next = new Set(prev);
      if (next.has(nodeId)) {
        next.delete(nodeId);
      } else {
        next.add(nodeId);
      }
      return next;
    });
  };

  const renderCategory = (category: CategoryNode, depth: number = 0) => {
    const isExpanded = expandedNodes.has(category.id);
    const isSelected = selectedCategories.has(category.id);
    const hasChildren = category.children && category.children.length > 0;

    return (
      <div key={category.id} style={{ marginLeft: `${depth * 20}px` }}>
        <div
          className={`flex items-center p-2 rounded cursor-pointer transition-colors ${
            isSelected ? 'bg-blue-100 border-blue-300' : 'hover:bg-gray-100'
          }`}
          onClick={() => toggleCategory(category.id)}
        >
          {hasChildren && (
            <button
              onClick={(e) => toggleExpand(category.id, e)}
              className="mr-1 p-1 hover:bg-gray-200 rounded"
            >
              <span className={`inline-block transition-transform ${isExpanded ? 'rotate-90' : ''}`}>
                ▶
              </span>
            </button>
          )}
          {!hasChildren && <span className="w-6" />}

          <input
            type="checkbox"
            checked={isSelected}
            onChange={() => {}}
            onClick={(e) => e.stopPropagation()}
            className="mr-2 w-4 h-4 text-blue-600 rounded focus:ring-blue-500"
          />

          <span className="flex-1 font-medium text-gray-800">{category.name}</span>
          <span className="text-sm text-gray-500 bg-gray-100 px-2 py-0.5 rounded-full">
            {category.count}
          </span>
        </div>

        {hasChildren && isExpanded && (
          <div className="mt-1">
            {category.children!.map(child => renderCategory(child, depth + 1))}
          </div>
        )}
      </div>
    );
  };

  return (
    <div className="h-full flex flex-col">
      <div className="flex items-center justify-between mb-4">
        <div>
          <h2 className="text-lg font-semibold">Select Categories</h2>
          <p className="text-sm text-gray-600">
            Choose research areas to explore. Selected: {selectedCategories.size} categories
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
            onClick={() => onSelect(Array.from(selectedCategories))}
            disabled={selectedCategories.size === 0}
            className="px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600 disabled:bg-gray-300 disabled:cursor-not-allowed transition-colors"
          >
            Continue →
          </button>
        </div>
      </div>

      <div className="flex-1 overflow-auto border rounded-lg p-4 bg-gray-50">
        {categories.length === 0 ? (
          <div className="text-center py-8 text-gray-500">
            No categories available. Please search first.
          </div>
        ) : (
          categories.map(category => renderCategory(category))
        )}
      </div>
    </div>
  );
};
