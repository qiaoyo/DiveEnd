import React, { useState, useCallback, useEffect } from 'react';
import { Document, Page, pdfjs } from 'react-pdf';
import 'react-pdf/dist/esm/Page/AnnotationLayer.css';
import 'react-pdf/dist/esm/Page/TextLayer.css';
import {
  ZoomIn,
  ZoomOut,
  ChevronLeft,
  ChevronRight,
  RotateCw,
  Download,
  Search,
  Bookmark
} from 'lucide-react';

// Configure PDF.js worker
pdfjs.GlobalWorkerOptions.workerSrc = `//cdnjs.cloudflare.com/ajax/libs/pdf.js/${pdfjs.version}/pdf.worker.min.js`;

interface PDFViewerProps {
  url: string;
  filename?: string;
  initialPage?: number;
  onPageChange?: (page: number) => void;
  onTextSelection?: (text: string) => void;
  className?: string;
}

export const PDFViewer: React.FC<PDFViewerProps> = ({
  url,
  filename,
  initialPage = 1,
  onPageChange,
  onTextSelection,
  className = '',
}) => {
  const [numPages, setNumPages] = useState<number>(0);
  const [pageNumber, setPageNumber] = useState<number>(initialPage);
  const [scale, setScale] = useState<number>(1.0);
  const [rotation, setRotation] = useState<number>(0);
  const [isLoading, setIsLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);
  const [searchText, setSearchText] = useState<string>('');
  const [showSearch, setShowSearch] = useState<boolean>(false);

  const onDocumentLoadSuccess = useCallback(({ numPages }: { numPages: number }) => {
    setNumPages(numPages);
    setIsLoading(false);
    setError(null);
  }, []);

  const onDocumentLoadError = useCallback((error: Error) => {
    setError(`Failed to load PDF: ${error.message}`);
    setIsLoading(false);
  }, []);

  const goToPage = useCallback((page: number) => {
    const newPage = Math.max(1, Math.min(page, numPages));
    setPageNumber(newPage);
    onPageChange?.(newPage);
  }, [numPages, onPageChange]);

  const changeScale = useCallback((delta: number) => {
    setScale(prev => Math.max(0.5, Math.min(3.0, prev + delta)));
  }, []);

  const rotate = useCallback(() => {
    setRotation(prev => (prev + 90) % 360);
  }, []);

  const handleTextSelection = useCallback(() => {
    const selection = window.getSelection();
    if (selection && selection.toString().trim()) {
      onTextSelection?.(selection.toString());
    }
  }, [onTextSelection]);

  const handleDownload = useCallback(() => {
    const link = document.createElement('a');
    link.href = url;
    link.download = filename || 'document.pdf';
    link.click();
  }, [url, filename]);

  // Keyboard shortcuts
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) {
        return;
      }

      switch (e.key) {
        case 'ArrowLeft':
          e.preventDefault();
          goToPage(pageNumber - 1);
          break;
        case 'ArrowRight':
          e.preventDefault();
          goToPage(pageNumber + 1);
          break;
        case '+':
        case '=':
          e.preventDefault();
          changeScale(0.1);
          break;
        case '-':
          e.preventDefault();
          changeScale(-0.1);
          break;
        case 'f':
          if (e.ctrlKey || e.metaKey) {
            e.preventDefault();
            setShowSearch(true);
          }
          break;
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [goToPage, pageNumber, changeScale]);

  if (error) {
    return (
      <div className={`flex flex-col items-center justify-center h-full p-8 ${className}`}>
        <div className="text-red-500 mb-4">
          <svg className="w-16 h-16" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
          </svg>
        </div>
        <h3 className="text-lg font-semibold text-gray-900 mb-2">Failed to Load PDF</h3>
        <p className="text-gray-600 text-center max-w-md">{error}</p>
      </div>
    );
  }

  return (
    <div className={`flex flex-col h-full ${className}`}>
      {/* Toolbar */}
      <div className="flex items-center justify-between px-4 py-2 border-b bg-muted/50">
        <div className="flex items-center gap-2">
          <button
            onClick={() => goToPage(pageNumber - 1)}
            disabled={pageNumber <= 1}
            className="p-1.5 rounded-md hover:bg-muted disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            title="Previous Page"
          >
            <ChevronLeft className="w-4 h-4" />
          </button>

          <span className="text-sm text-muted-foreground min-w-[100px] text-center">
            {pageNumber} / {numPages || '-'}
          </span>

          <button
            onClick={() => goToPage(pageNumber + 1)}
            disabled={pageNumber >= numPages}
            className="p-1.5 rounded-md hover:bg-muted disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            title="Next Page"
          >
            <ChevronRight className="w-4 h-4" />
          </button>
        </div>

        <div className="flex items-center gap-1">
          <button
            onClick={() => setShowSearch(!showSearch)}
            className={`p-1.5 rounded-md transition-colors ${showSearch ? 'bg-primary text-primary-foreground' : 'hover:bg-muted'}`}
            title="Search (Ctrl+F)"
          >
            <Search className="w-4 h-4" />
          </button>

          <div className="w-px h-4 bg-border mx-1" />

          <button
            onClick={() => changeScale(-0.1)}
            className="p-1.5 rounded-md hover:bg-muted transition-colors"
            title="Zoom Out (-)"
          >
            <ZoomOut className="w-4 h-4" />
          </button>

          <span className="text-sm text-muted-foreground min-w-[60px] text-center">
            {Math.round(scale * 100)}%
          </span>

          <button
            onClick={() => changeScale(0.1)}
            className="p-1.5 rounded-md hover:bg-muted transition-colors"
            title="Zoom In (+)"
          >
            <ZoomIn className="w-4 h-4" />
          </button>

          <div className="w-px h-4 bg-border mx-1" />

          <button
            onClick={rotate}
            className="p-1.5 rounded-md hover:bg-muted transition-colors"
            title="Rotate"
          >
            <RotateCw className="w-4 h-4" />
          </button>

          <button
            onClick={handleDownload}
            className="p-1.5 rounded-md hover:bg-muted transition-colors"
            title="Download PDF"
          >
            <Download className="w-4 h-4" />
          </button>
        </div>
      </div>

      {/* Search Bar */}
      {showSearch && (
        <div className="flex items-center gap-2 px-4 py-2 border-b bg-muted/30">
          <Search className="w-4 h-4 text-muted-foreground" />
          <input
            type="text"
            value={searchText}
            onChange={(e) => setSearchText(e.target.value)}
            placeholder="Search in document..."
            className="flex-1 bg-transparent border-none outline-none text-sm placeholder:text-muted-foreground"
            autoFocus
          />
          <button
            onClick={() => {
              setSearchText('');
              setShowSearch(false);
            }}
            className="text-muted-foreground hover:text-foreground"
          >
            ×
          </button>
        </div>
      )}

      {/* PDF Document */}
      <div
        className="flex-1 overflow-auto bg-muted/20"
        onMouseUp={handleTextSelection}
      >
        <Document
          file={url}
          onLoadSuccess={onDocumentLoadSuccess}
          onLoadError={onDocumentLoadError}
          loading={
            <div className="flex items-center justify-center h-full">
              <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
            </div>
          }
        >
          <Page
            pageNumber={pageNumber}
            scale={scale}
            rotate={rotation}
            renderTextLayer={true}
            renderAnnotationLayer={true}
            className="shadow-lg"
          />
        </Document>
      </div>
    </div>
  );
};

export default PDFViewer;
