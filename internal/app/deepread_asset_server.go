package app

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const deepReadPDFResourceTTL = 30 * time.Minute

type deepReadPDFResource struct {
	PaperID   string
	Path      string
	ExpiresAt time.Time
}

func (a *App) assetServerHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/deepread/pdf/", a.serveDeepReadPDFResource)
	return mux
}

func (a *App) storeDeepReadPDFResource(token string, resource deepReadPDFResource) {
	a.pdfResourceMu.Lock()
	defer a.pdfResourceMu.Unlock()

	now := time.Now()
	for existingToken, existing := range a.pdfResources {
		if existing.ExpiresAt.Before(now) {
			delete(a.pdfResources, existingToken)
		}
	}
	a.pdfResources[token] = resource
}

func (a *App) loadDeepReadPDFResource(token string) (deepReadPDFResource, bool) {
	a.pdfResourceMu.Lock()
	defer a.pdfResourceMu.Unlock()

	resource, ok := a.pdfResources[token]
	if !ok {
		return deepReadPDFResource{}, false
	}
	if resource.ExpiresAt.Before(time.Now()) {
		delete(a.pdfResources, token)
		return deepReadPDFResource{}, false
	}
	return resource, true
}

func (a *App) serveDeepReadPDFResource(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/deepread/pdf/"))
	if token == "" || strings.Contains(token, "/") {
		http.NotFound(w, r)
		return
	}

	resource, ok := a.loadDeepReadPDFResource(token)
	if !ok {
		http.NotFound(w, r)
		return
	}
	pdfPath, err := a.resolveDeepReadPDFPath(resource.Path)
	if err != nil {
		http.Error(w, "pdf is not available", http.StatusForbidden)
		return
	}

	file, err := os.Open(pdfPath)
	if err != nil {
		http.Error(w, "pdf is not available", http.StatusNotFound)
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil || info.IsDir() {
		http.Error(w, "pdf is not available", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", filepath.Base(pdfPath)))
	http.ServeContent(w, r, filepath.Base(pdfPath), info.ModTime(), file)
}
