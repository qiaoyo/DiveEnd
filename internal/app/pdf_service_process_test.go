package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindPDFServiceDirectoryFromExplicitEnvironment(t *testing.T) {
	serviceDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(serviceDir, "app"), 0700); err != nil {
		t.Fatalf("MkdirAll app: %v", err)
	}
	if err := os.WriteFile(filepath.Join(serviceDir, "app", "main.py"), []byte("# test"), 0600); err != nil {
		t.Fatalf("WriteFile main.py: %v", err)
	}
	t.Setenv("DIVEEND_PDF_SERVICE_DIR", serviceDir)

	resolved, err := findPDFServiceDirectory()
	if err != nil {
		t.Fatalf("findPDFServiceDirectory() error = %v", err)
	}
	if resolved != serviceDir {
		t.Fatalf("expected %q, got %q", serviceDir, resolved)
	}
}

func TestResolvePDFServicePythonPrefersManagedVirtualenv(t *testing.T) {
	serviceDir := t.TempDir()
	pythonPath := filepath.Join(serviceDir, ".venv", "bin", "python")
	if err := os.MkdirAll(filepath.Dir(pythonPath), 0700); err != nil {
		t.Fatalf("MkdirAll venv bin: %v", err)
	}
	if err := os.WriteFile(pythonPath, []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatalf("WriteFile python: %v", err)
	}
	t.Setenv("DIVEEND_PDF_PYTHON", "")

	resolved, err := resolvePDFServicePython(serviceDir)
	if err != nil {
		t.Fatalf("resolvePDFServicePython() error = %v", err)
	}
	if resolved != pythonPath {
		t.Fatalf("expected managed Python %q, got %q", pythonPath, resolved)
	}
}

func TestVerifyPDFServicePythonReturnsActionableFailure(t *testing.T) {
	fakePython := filepath.Join(t.TempDir(), "python")
	if err := os.WriteFile(fakePython, []byte("#!/bin/sh\nexit 1\n"), 0700); err != nil {
		t.Fatalf("WriteFile fake python: %v", err)
	}

	err := verifyPDFServicePython(context.Background(), fakePython)
	if err == nil {
		t.Fatal("expected dependency verification failure")
	}
	if !strings.Contains(err.Error(), "scripts/setup_pdf_service.sh") {
		t.Fatalf("expected setup command in error, got %v", err)
	}
}
