package app

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func (a *App) resolveDeepReadPDFPath(pdfPath string) (string, error) {
	return resolveManagedDeepReadPDFPath(a.config.DataPath, pdfPath)
}

func (a *App) deepReadManagedPDFAvailable(pdfPath string) bool {
	_, err := a.resolveDeepReadPDFPath(pdfPath)
	return err == nil
}

func resolveManagedDeepReadPDFPath(dataPath, pdfPath string) (string, error) {
	pdfPath = strings.TrimSpace(pdfPath)
	if pdfPath == "" {
		return "", fmt.Errorf("paper has no local pdf path")
	}
	if strings.ToLower(filepath.Ext(pdfPath)) != ".pdf" {
		return "", fmt.Errorf("local file is not a pdf")
	}

	papersRoot := filepath.Join(strings.TrimSpace(dataPath), "papers")
	root, err := filepath.Abs(papersRoot)
	if err != nil {
		return "", err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("managed pdf root is not available")
	}

	candidate, err := filepath.Abs(pdfPath)
	if err != nil {
		return "", err
	}
	candidate, err = filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", fmt.Errorf("local pdf is not available")
	}

	root = filepath.Clean(root)
	candidate = filepath.Clean(candidate)
	if candidate != root && !strings.HasPrefix(candidate, root+string(os.PathSeparator)) {
		return "", fmt.Errorf("local pdf is outside the managed papers directory")
	}
	if err := validateLocalPDFFile(candidate); err != nil {
		return "", err
	}
	return candidate, nil
}

func validateLocalPDFFile(path string) error {
	file, _, err := openValidatedLocalPDFFile(path)
	if err != nil {
		return err
	}
	return file.Close()
}

func validateOpenLocalPDFFile(file *os.File) error {
	if file == nil {
		return fmt.Errorf("local pdf file cannot be nil")
	}
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("local pdf is not available")
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return fmt.Errorf("local pdf is empty or invalid")
	}
	if info.Size() > pdfDownloadMaxBytes {
		return fmt.Errorf("local pdf is too large: %d bytes exceeds %d bytes limit", info.Size(), pdfDownloadMaxBytes)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("local pdf is not seekable")
	}
	header := make([]byte, 5)
	n, err := io.ReadFull(file, header)
	if err != nil && err != io.ErrUnexpectedEOF {
		return fmt.Errorf("local pdf is not readable")
	}
	if n < len(header) || string(header) != "%PDF-" {
		return fmt.Errorf("local file is not a valid pdf")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("local pdf is not seekable")
	}
	return nil
}

func openValidatedLocalPDFFile(path string) (*os.File, os.FileInfo, error) {
	return openValidatedLocalPDFFileWithOpen(path, os.Open)
}

func openValidatedLocalPDFFileWithOpen(path string, openFile func(string) (*os.File, error)) (*os.File, os.FileInfo, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, nil, fmt.Errorf("local pdf path cannot be empty")
	}
	if openFile == nil {
		return nil, nil, fmt.Errorf("local pdf open function cannot be nil")
	}

	linkInfo, err := os.Lstat(path)
	if err != nil {
		return nil, nil, fmt.Errorf("local pdf is not available")
	}
	if linkInfo.Mode()&os.ModeSymlink != 0 {
		return nil, nil, fmt.Errorf("local pdf is a symbolic link")
	}
	if !linkInfo.Mode().IsRegular() || linkInfo.Size() == 0 {
		return nil, nil, fmt.Errorf("local pdf is empty or invalid")
	}
	if linkInfo.Size() > pdfDownloadMaxBytes {
		return nil, nil, fmt.Errorf("local pdf is too large: %d bytes exceeds %d bytes limit", linkInfo.Size(), pdfDownloadMaxBytes)
	}

	file, err := openFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("local pdf is not available")
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		_ = file.Close()
		return nil, nil, fmt.Errorf("local pdf is empty or invalid")
	}
	if info.Size() > pdfDownloadMaxBytes {
		_ = file.Close()
		return nil, nil, fmt.Errorf("local pdf is too large: %d bytes exceeds %d bytes limit", info.Size(), pdfDownloadMaxBytes)
	}
	if !os.SameFile(linkInfo, info) {
		_ = file.Close()
		return nil, nil, fmt.Errorf("local pdf changed while opening")
	}

	header := make([]byte, 5)
	n, err := io.ReadFull(file, header)
	if err != nil && err != io.ErrUnexpectedEOF {
		_ = file.Close()
		return nil, nil, fmt.Errorf("local pdf is not readable")
	}
	if n < len(header) || string(header) != "%PDF-" {
		_ = file.Close()
		return nil, nil, fmt.Errorf("local file is not a valid pdf")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		_ = file.Close()
		return nil, nil, fmt.Errorf("local pdf is not seekable")
	}
	return file, info, nil
}
