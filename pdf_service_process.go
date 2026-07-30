package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	pdfServiceStartupTimeout = 20 * time.Second
	pdfServiceStopTimeout    = 3 * time.Second
)

type managedPDFService struct {
	client     *PDFServiceClient
	serviceDir string
	pythonPath string

	startMu sync.Mutex
	mu      sync.Mutex
	cmd     *exec.Cmd
	done    chan error
}

func newManagedPDFService(client *PDFServiceClient) (*managedPDFService, error) {
	serviceDir, err := findPDFServiceDirectory()
	if err != nil {
		return nil, err
	}
	pythonPath, err := resolvePDFServicePython(serviceDir)
	if err != nil {
		return nil, err
	}
	return &managedPDFService{
		client:     client,
		serviceDir: serviceDir,
		pythonPath: pythonPath,
	}, nil
}

func (s *managedPDFService) EnsureRunning(ctx context.Context) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("PDF service manager is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	s.startMu.Lock()
	defer s.startMu.Unlock()

	if status := s.client.Status(ctx); status.Healthy && status.Ready {
		return nil
	}
	if err := verifyPDFServicePython(ctx, s.pythonPath); err != nil {
		return err
	}

	s.mu.Lock()
	if s.cmd == nil {
		cmd := exec.CommandContext(
			context.Background(),
			s.pythonPath,
			"-m", "uvicorn",
			"app.main:app",
			"--host", "127.0.0.1",
			"--port", "50051",
		)
		cmd.Dir = s.serviceDir
		cmd.Env = append(os.Environ(), "PYTHONUNBUFFERED=1")
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		if err := cmd.Start(); err != nil {
			s.mu.Unlock()
			return fmt.Errorf("start PDF service: %w", err)
		}

		done := make(chan error, 1)
		s.cmd = cmd
		s.done = done
		go func() {
			err := cmd.Wait()
			done <- err
			close(done)

			s.mu.Lock()
			if s.cmd == cmd {
				s.cmd = nil
				s.done = nil
			}
			s.mu.Unlock()
		}()
	}
	done := s.done
	s.mu.Unlock()

	startupCtx, cancel := context.WithTimeout(ctx, pdfServiceStartupTimeout)
	defer cancel()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		status := s.client.Status(startupCtx)
		if status.Healthy && status.Ready {
			return nil
		}

		select {
		case err, ok := <-done:
			if !ok || err == nil {
				return fmt.Errorf("PDF service exited before becoming ready")
			}
			return fmt.Errorf("PDF service exited before becoming ready: %w", err)
		case <-startupCtx.Done():
			return fmt.Errorf("PDF service did not become ready within %s: %w", pdfServiceStartupTimeout, startupCtx.Err())
		case <-ticker.C:
		}
	}
}

func (s *managedPDFService) Stop() {
	if s == nil {
		return
	}
	s.startMu.Lock()
	defer s.startMu.Unlock()

	s.mu.Lock()
	cmd := s.cmd
	done := s.done
	s.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return
	}

	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		_ = cmd.Process.Kill()
	}
	if done == nil {
		return
	}

	select {
	case <-done:
	case <-time.After(pdfServiceStopTimeout):
		_ = cmd.Process.Kill()
		<-done
	}
}

func findPDFServiceDirectory() (string, error) {
	candidates := make([]string, 0, 10)
	if configured := strings.TrimSpace(os.Getenv("DIVEEND_PDF_SERVICE_DIR")); configured != "" {
		candidates = append(candidates, configured)
	}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(cwd, "services", "pdf_service"))
	}
	if executable, err := os.Executable(); err == nil {
		current := filepath.Dir(executable)
		for range 8 {
			candidates = append(candidates, filepath.Join(current, "services", "pdf_service"))
			parent := filepath.Dir(current)
			if parent == current {
				break
			}
			current = parent
		}
	}

	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		absolute, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		absolute = filepath.Clean(absolute)
		if _, ok := seen[absolute]; ok {
			continue
		}
		seen[absolute] = struct{}{}
		entrypoint := filepath.Join(absolute, "app", "main.py")
		info, err := os.Stat(entrypoint)
		if err == nil && info.Mode().IsRegular() {
			return absolute, nil
		}
	}
	return "", fmt.Errorf("PDF service source was not found; expected services/pdf_service/app/main.py")
}

func resolvePDFServicePython(serviceDir string) (string, error) {
	candidates := []string{}
	if configured := strings.TrimSpace(os.Getenv("DIVEEND_PDF_PYTHON")); configured != "" {
		candidates = append(candidates, configured)
	}
	candidates = append(candidates,
		filepath.Join(serviceDir, ".venv", "bin", "python"),
		"python3",
		"python",
	)

	for _, candidate := range candidates {
		resolved := candidate
		if !filepath.IsAbs(candidate) {
			pathValue, err := exec.LookPath(candidate)
			if err != nil {
				continue
			}
			resolved = pathValue
		}
		info, err := os.Stat(resolved)
		if err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0 {
			return resolved, nil
		}
	}
	return "", fmt.Errorf("Python for the PDF service was not found; create services/pdf_service/.venv")
}

func verifyPDFServicePython(ctx context.Context, pythonPath string) error {
	checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(
		checkCtx,
		pythonPath,
		"-c",
		"import fastapi, uvicorn, pymupdf4llm, fitz, pydantic_settings",
	)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("PDF service Python dependencies are unavailable; run scripts/setup_pdf_service.sh: %w", err)
	}
	return nil
}

func (a *App) startManagedPDFService() {
	if a.pdfService == nil || strings.TrimSpace(os.Getenv("DIVEEND_PDF_SERVICE_URL")) != "" || strings.HasSuffix(os.Args[0], ".test") {
		return
	}
	manager, err := newManagedPDFService(a.pdfService)
	if err != nil {
		log.Printf("PDF service auto-start unavailable: %v", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), pdfServiceStartupTimeout+5*time.Second)
	defer cancel()
	if err := manager.EnsureRunning(ctx); err != nil {
		log.Printf("PDF service auto-start failed: %v", err)
		manager.Stop()
		return
	}
	a.pdfServiceProcess = manager
	log.Printf("PDF service is ready")
}
