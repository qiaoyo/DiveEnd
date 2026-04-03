import React, { useState, useCallback } from 'react';

interface Paper {
  id: string;
  title: string;
  authors: string;
  abstract: string;
  year: number;
  pdfPath?: string;
  status: 'pending' | 'extracting' | 'extracted' | 'screening' | 'selected' | 'rejected';
  extractedData?: {
    fullText: string;
    sections: Array<{ title: string; content: string }>;
    metrics?: Record<string, any>;
  };
}

interface DecisionNode {
  id: string;
  message: string;
  dimension: string;
  options: Array<{
    key: string;
    label: string;
    paperIds: string[];
    count: number;
  }>;
  allowMultiSelect: boolean;
}

type ScreeningStage = 'upload' | 'extract' | 'screen' | 'results';

export const Screening: React.FC = () => {
  const [currentStage, setCurrentStage] = useState<ScreeningStage>('upload');
  const [papers, setPapers] = useState<Paper[]>([]);
  const [currentNode, setCurrentNode] = useState<DecisionNode | null>(null);
  const [selectedOptions, setSelectedOptions] = useState<string[]>([]);
  const [screeningHistory, setScreeningHistory] = useState<Array<{ dimension: string; choice: string }>>([]);
  const [isProcessing, setIsProcessing] = useState(false);

  // File upload handler
  const handleFileUpload = useCallback(async (files: FileList) => {
    setIsProcessing(true);
    const newPapers: Paper[] = [];

    for (let i = 0; i < files.length; i++) {
      const file = files[i];
      if (file.type === 'application/pdf' || file.name.endsWith('.pdf')) {
        newPapers.push({
          id: `paper-${Date.now()}-${i}`,
          title: file.name.replace('.pdf', ''),
          authors: '',
          abstract: '',
          year: new Date().getFullYear(),
          pdfPath: URL.createObjectURL(file),
          status: 'pending',
        });
      }
    }

    setPapers(prev => [...prev, ...newPapers]);
    setIsProcessing(false);

    if (newPapers.length > 0) {
      setCurrentStage('extract');
      // TODO: Trigger extraction process
    }
  }, []);

  // Mock extraction completion
  const handleExtractionComplete = useCallback(() => {
    setPapers(prev => prev.map(p =>
      p.status === 'extracting' ? { ...p, status: 'extracted' } : p
    ));
    setCurrentStage('screen');
    // TODO: Initialize screening with first decision node
    initializeScreening();
  }, []);

  // Initialize screening process
  const initializeScreening = useCallback(() => {
    // Mock decision node - in real implementation, this comes from LLM
    const mockNode: DecisionNode = {
      id: 'node-1',
      message: 'What research areas are you most interested in?',
      dimension: 'Research Area',
      options: [
        { key: 'ml', label: 'Machine Learning', paperIds: [], count: 15 },
        { key: 'nlp', label: 'Natural Language Processing', paperIds: [], count: 12 },
        { key: 'cv', label: 'Computer Vision', paperIds: [], count: 8 },
        { key: 'rl', label: 'Reinforcement Learning', paperIds: [], count: 5 },
      ],
      allowMultiSelect: true,
    };
    setCurrentNode(mockNode);
  }, []);

  // Handle option selection
  const handleOptionToggle = useCallback((optionKey: string) => {
    setSelectedOptions(prev => {
      if (prev.includes(optionKey)) {
        return prev.filter(k => k !== optionKey);
      }
      return [...prev, optionKey];
    });
  }, []);

  // Handle continue to next node
  const handleContinue = useCallback(() => {
    if (!currentNode || selectedOptions.length === 0) return;

    // Record history
    setScreeningHistory(prev => [
      ...prev,
      {
        dimension: currentNode.dimension,
        choice: selectedOptions.join(', '),
      },
    ]);

    // Check if we should continue or finish
    if (screeningHistory.length >= 2) {
      // Finish screening
      setCurrentStage('results');
    } else {
      // Generate next node (mock)
      const nextNode: DecisionNode = {
        id: `node-${screeningHistory.length + 2}`,
        message: 'What methodology types do you prefer?',
        dimension: 'Methodology',
        options: [
          { key: 'empirical', label: 'Empirical Study', paperIds: [], count: 10 },
          { key: 'theoretical', label: 'Theoretical Analysis', paperIds: [], count: 8 },
          { key: 'review', label: 'Survey/Review', paperIds: [], count: 5 },
        ],
        allowMultiSelect: true,
      };
      setCurrentNode(nextNode);
      setSelectedOptions([]);
    }
  }, [currentNode, selectedOptions, screeningHistory]);

  // Render upload stage
  const renderUploadStage = () => (
    <div className="text-center py-12">
      <div className="text-6xl mb-4">📄</div>
      <h2 className="text-2xl font-bold mb-2">Upload Papers for Screening</h2>
      <p className="text-gray-600 mb-6 max-w-md mx-auto">
        Upload PDF files to start the screening pipeline. The system will extract content,
        analyze papers, and guide you through an interactive decision tree.
      </p>

      <div className="flex justify-center gap-4">
        <label className="px-6 py-3 bg-blue-500 text-white rounded-lg hover:bg-blue-600 cursor-pointer transition-colors">
          <input
            type="file"
            multiple
            accept=".pdf"
            className="hidden"
            onChange={(e) => e.target.files && handleFileUpload(e.target.files)}
          />
          Select PDF Files
        </label>
        <button
          onClick={() => setCurrentStage('extract')}
          className="px-6 py-3 bg-gray-200 text-gray-700 rounded-lg hover:bg-gray-300 transition-colors"
        >
          Use Sample Papers
        </button>
      </div>
    </div>
  );

  // Render extract stage
  const renderExtractStage = () => (
    <div className="text-center py-12">
      <div className="text-6xl mb-4">⚙️</div>
      <h2 className="text-2xl font-bold mb-2">Extracting Paper Content</h2>
      <p className="text-gray-600 mb-6">
        The system is analyzing PDFs and extracting structured content...
      </p>

      <div className="max-w-md mx-auto">
        <div className="bg-gray-200 rounded-full h-2 mb-4">
          <div className="bg-blue-500 h-2 rounded-full animate-pulse" style={{ width: '60%' }} />
        </div>
        <p className="text-sm text-gray-500">Processing paper 3 of 5...</p>
      </div>

      {/* Mock completion button for testing */}
      <button
        onClick={handleExtractionComplete}
        className="mt-8 px-4 py-2 bg-green-500 text-white rounded hover:bg-green-600 transition-colors"
      >
        (Test) Complete Extraction
      </button>
    </div>
  );

  // Render screen stage
  const renderScreenStage = () => {
    if (!currentNode) return null;

    return (
      <div className="h-full flex flex-col">
        <div className="mb-6">
          <h2 className="text-xl font-semibold mb-2">{currentNode.message}</h2>
          <p className="text-gray-600">
            Dimension: <span className="font-medium">{currentNode.dimension}</span>
            {currentNode.allowMultiSelect && (
              <span className="ml-2 text-sm text-blue-600">(Multiple selection allowed)</span>
            )}
          </p>
        </div>

        {/* Selection History */}
        {screeningHistory.length > 0 && (
          <div className="mb-4 p-3 bg-gray-50 rounded-lg">
            <h3 className="text-sm font-medium text-gray-700 mb-2">Previous Selections</h3>
            <div className="space-y-1">
              {screeningHistory.map((step, index) => (
                <div key={index} className="text-sm text-gray-600">
                  <span className="font-medium">{step.dimension}:</span> {step.choice}
                </div>
              ))}
            </div>
          </div>
        )}

        {/* Options */}
        <div className="flex-1 overflow-auto">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
            {currentNode.options.map((option) => {
              const isSelected = selectedOptions.includes(option.key);
              return (
                <button
                  key={option.key}
                  onClick={() => handleOptionToggle(option.key)}
                  className={`p-4 border-2 rounded-lg text-left transition-all ${
                    isSelected
                      ? 'border-blue-500 bg-blue-50'
                      : 'border-gray-200 hover:border-gray-300 hover:bg-gray-50'
                  }`}
                >
                  <div className="flex items-center justify-between">
                    <span className="font-medium text-gray-900">{option.label}</span>
                    <span className="text-sm text-gray-500 bg-gray-100 px-2 py-0.5 rounded-full">
                      {option.count}
                    </span>
                  </div>
                  {isSelected && (
                    <div className="mt-2 text-blue-600 text-sm">✓ Selected</div>
                  )}
                </button>
              );
            })}
          </div>
        </div>

        {/* Actions */}
        <div className="flex justify-between items-center mt-4 pt-4 border-t">
          <div className="text-sm text-gray-600">
            Selected: <span className="font-medium">{selectedOptions.length}</span> options
          </div>
          <div className="flex gap-2">
            <button
              onClick={() => {
                setCurrentNode(null);
                setSelectedOptions([]);
                setScreeningHistory([]);
                setCurrentStage('upload');
              }}
              className="px-4 py-2 text-gray-600 hover:text-gray-800 transition-colors"
            >
              Start Over
            </button>
            <button
              onClick={handleContinue}
              disabled={selectedOptions.length === 0}
              className="px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600 disabled:bg-gray-300 disabled:cursor-not-allowed transition-colors"
            >
              Continue →
            </button>
          </div>
        </div>
      </div>
    );
  };

  // Render results stage
  const renderResultsStage = () => (
    <div className="text-center py-12">
      <div className="text-6xl mb-4">🎉</div>
      <h2 className="text-2xl font-bold mb-2">Screening Complete!</h2>
      <p className="text-gray-600 mb-6 max-w-md mx-auto">
        You have successfully screened {papers.length} papers through the decision tree.
        Selected papers have been added to your library.
      </p>

      <div className="max-w-md mx-auto mb-6">
        <div className="grid grid-cols-2 gap-4">
          <div className="p-4 bg-gray-50 rounded-lg">
            <div className="text-2xl font-bold text-blue-600">{papers.length}</div>
            <div className="text-sm text-gray-600">Total Papers</div>
          </div>
          <div className="p-4 bg-gray-50 rounded-lg">
            <div className="text-2xl font-bold text-green-600">{selectedPapers.size}</div>
            <div className="text-sm text-gray-600">Selected</div>
          </div>
        </div>
      </div>

      <button
        onClick={() => {
          setCurrentStage('upload');
          setPapers([]);
          setSelectedPapers(new Set());
          setCategories([]);
          setSelectionSteps([]);
          setScreeningHistory([]);
          setCurrentNode(null);
          setSelectedOptions([]);
        }}
        className="px-6 py-3 bg-blue-500 text-white rounded-lg hover:bg-blue-600 transition-colors"
      >
        Start New Screening
      </button>
    </div>
  );

  return (
    <div className="h-full">
      {currentStage === 'upload' && renderUploadStage()}
      {currentStage === 'extract' && renderExtractStage()}
      {currentStage === 'screen' && renderScreenStage()}
      {currentStage === 'results' && renderResultsStage()}
    </div>
  );
};
