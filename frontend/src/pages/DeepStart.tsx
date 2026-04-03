import React, { useState, useCallback } from 'react';
import { SearchPanel } from '../components/deepstart/SearchPanel';
import { SearchResults } from '../components/deepstart/SearchResults';
import { CategoryTree } from '../components/deepstart/CategoryTree';
import { SelectionPanel } from '../components/deepstart/SelectionPanel';

export interface SearchResult {
  ID: string;
  Title: string;
  Authors: string;
  Abstract: string;
  Year: number;
  Journal: string;
  URL: string;
  Source: string;
  Citations: number;
  PDFURL: string;
}

export interface CategoryNode {
  id: string;
  name: string;
  count: number;
  children?: CategoryNode[];
}

export interface SelectionStep {
  id: string;
  dimension: string;
  selected: string[];
  remainingCount: number;
}

type DeepStartStep = 'search' | 'categories' | 'select' | 'import';

export const DeepStart: React.FC = () => {
  const [currentStep, setCurrentStep] = useState<DeepStartStep>('search');
  const [searchResults, setSearchResults] = useState<SearchResult[]>([]);
  const [selectedPapers, setSelectedPapers] = useState<Set<string>>(new Set());
  const [categories, setCategories] = useState<CategoryNode[]>([]);
  const [selectionSteps, setSelectionSteps] = useState<SelectionStep[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [targetFolder, setTargetFolder] = useState<string>('');

  const handleSearch = useCallback(async (query: string, limit: number) => {
    setIsLoading(true);
    try {
      // @ts-ignore - Wails runtime
      const results = await window.go.main.App.SearchPapers(query, limit);
      setSearchResults(results || []);
      setCurrentStep('categories');

      // Generate mock categories for now
      // TODO: Replace with AI classification
      const mockCategories: CategoryNode[] = [
        {
          id: '1',
          name: 'Machine Learning',
          count: Math.floor(results.length * 0.4),
          children: [
            { id: '1-1', name: 'Deep Learning', count: Math.floor(results.length * 0.2) },
            { id: '1-2', name: 'Reinforcement Learning', count: Math.floor(results.length * 0.1) },
          ],
        },
        {
          id: '2',
          name: 'Natural Language Processing',
          count: Math.floor(results.length * 0.3),
          children: [
            { id: '2-1', name: 'Language Models', count: Math.floor(results.length * 0.15) },
            { id: '2-2', name: 'Machine Translation', count: Math.floor(results.length * 0.1) },
          ],
        },
        {
          id: '3',
          name: 'Computer Vision',
          count: Math.floor(results.length * 0.3),
        },
      ];
      setCategories(mockCategories);
    } catch (error) {
      console.error('Search failed:', error);
      // TODO: Show error toast
    } finally {
      setIsLoading(false);
    }
  }, []);

  const handleCategorySelect = useCallback((categoryIds: string[]) => {
    // Filter papers based on selected categories
    // For now, just move to selection step
    const step: SelectionStep = {
      id: Date.now().toString(),
      dimension: 'Research Area',
      selected: categoryIds,
      remainingCount: searchResults.length,
    };
    setSelectionSteps([step]);
    setCurrentStep('select');
  }, [searchResults.length]);

  const handlePaperSelect = useCallback((paperId: string, selected: boolean) => {
    setSelectedPapers(prev => {
      const next = new Set(prev);
      if (selected) {
        next.add(paperId);
      } else {
        next.delete(paperId);
      }
      return next;
    });
  }, []);

  const handleImport = useCallback(async () => {
    setIsLoading(true);
    try {
      const papersToImport = searchResults.filter(p => selectedPapers.has(p.ID));

      // Convert to SearchPaper format expected by backend
      const searchPapers = papersToImport.map(p => ({
        ID: p.ID,
        Title: p.Title,
        Authors: p.Authors,
        Abstract: p.Abstract,
        Year: p.Year,
        Journal: p.Journal,
        URL: p.URL,
        Category: '',
        Tags: [],
      }));

      // @ts-ignore - Wails runtime
      await window.go.main.App.ImportPapers(targetFolder, searchPapers);

      setCurrentStep('import');
    } catch (error) {
      console.error('Import failed:', error);
      // TODO: Show error toast
    } finally {
      setIsLoading(false);
    }
  }, [searchResults, selectedPapers, targetFolder]);

  const renderStepIndicator = () => (
    <div className="flex items-center space-x-2 mb-6">
      {[
        { key: 'search', label: 'Search' },
        { key: 'categories', label: 'Categories' },
        { key: 'select', label: 'Select' },
        { key: 'import', label: 'Import' },
      ].map((step, index) => (
        <React.Fragment key={step.key}>
          <div
            className={`px-3 py-1 rounded-full text-sm font-medium ${
              currentStep === step.key
                ? 'bg-blue-500 text-white'
                : index < ['search', 'categories', 'select', 'import'].indexOf(currentStep)
                ? 'bg-green-500 text-white'
                : 'bg-gray-200 text-gray-600'
            }`}
          >
            {step.label}
          </div>
          {index < 3 && (
            <div className="w-4 h-px bg-gray-300" />
          )}
        </React.Fragment>
      ))}
    </div>
  );

  return (
    <div className="h-full flex flex-col p-6">
      <h1 className="text-2xl font-bold mb-2">DeepStart - Paper Discovery</h1>
      <p className="text-gray-600 mb-4">
        Discover papers with AI-guided exploration and multi-round selection.
      </p>

      {renderStepIndicator()}

      <div className="flex-1 overflow-auto">
        {currentStep === 'search' && (
          <SearchPanel onSearch={handleSearch} isLoading={isLoading} />
        )}

        {currentStep === 'categories' && (
          <CategoryTree
            categories={categories}
            onSelect={handleCategorySelect}
            onBack={() => setCurrentStep('search')}
          />
        )}

        {currentStep === 'select' && (
          <SelectionPanel
            papers={searchResults}
            selectedPapers={selectedPapers}
            selectionSteps={selectionSteps}
            onPaperSelect={handlePaperSelect}
            onImport={handleImport}
            onBack={() => setCurrentStep('categories')}
            isLoading={isLoading}
            targetFolder={targetFolder}
            onTargetFolderChange={setTargetFolder}
          />
        )}

        {currentStep === 'import' && (
          <div className="text-center py-12">
            <div className="text-6xl mb-4">✅</div>
            <h2 className="text-2xl font-bold mb-2">Import Complete!</h2>
            <p className="text-gray-600 mb-4">
              Successfully imported {selectedPapers.size} papers to your library.
            </p>
            <button
              onClick={() => {
                setCurrentStep('search');
                setSearchResults([]);
                setSelectedPapers(new Set());
                setCategories([]);
                setSelectionSteps([]);
              }}
              className="px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600"
            >
              Start New Search
            </button>
          </div>
        )}
      </div>
    </div>
  );
};