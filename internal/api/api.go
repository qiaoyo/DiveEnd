package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/qiaoyo/DiveEnd/internal/config"
	"github.com/qiaoyo/DiveEnd/internal/data"
	"github.com/qiaoyo/DiveEnd/internal/llm"
	"github.com/qiaoyo/DiveEnd/internal/search"
	"github.com/qiaoyo/DiveEnd/internal/sync"
)

// SetupRoutes sets up all API routes
func SetupRoutes(db *data.DB, cfg *config.Config) http.Handler {
	mux := http.NewServeMux()

	// Serve static web files
	mux.HandleFunc("/", serveIndex)
	mux.Handle("/css/", http.StripPrefix("/css/", http.FileServer(http.Dir("web/css"))))
	mux.Handle("/js/", http.StripPrefix("/js/", http.FileServer(http.Dir("web/js"))))
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))

	// API endpoints
	mux.HandleFunc("/api/config", handleGetConfig(cfg))
	mux.HandleFunc("/api/config/save", handleSaveConfig(cfg))
	mux.HandleFunc("/api/folders", handleGetFolders(db))
	mux.HandleFunc("/api/folders/create", handleCreateFolder(db))
	mux.HandleFunc("/api/papers", handleGetPapers(db))
	mux.HandleFunc("/api/paper/", handleGetPaper(db))
	mux.HandleFunc("/api/search/deepstart", handleDeepStart(cfg))
	mux.HandleFunc("/api/paper/import", handleImportPapers(db))
	mux.HandleFunc("/api/paper/deepread", handleDeepRead(db, cfg))
	mux.HandleFunc("/api/paper/translations", handleGetTranslations(db))
	mux.HandleFunc("/api/sync/baidu", handleBaiduSync(cfg))

	return logRequest(mux)
}

func logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "web/index.html")
}

// Response helpers
func jsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func errorResponse(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": err.Error(),
	})
}

// Handlers
func handleGetConfig(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Don't return API secrets to frontend
		safeConfig := map[string]interface{}{
			"port":        cfg.Port,
			"data_path":   cfg.DataPath,
			"llm_provider": cfg.LLM.Provider,
			"llm_model":    cfg.LLM.Model,
			"baidu_enabled": cfg.BaiduCloud.Enabled,
			"baidu_bucket":  cfg.BaiduCloud.Bucket,
		}
		jsonResponse(w, safeConfig)
	}
}

func handleSaveConfig(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			LLMProvider string `json:"llm_provider"`
			LLMAPIKey   string `json:"llm_api_key"`
			LLMModel    string `json:"llm_model"`
			LLMBaseURL  string `json:"llm_base_url"`
			BaiduEnabled bool   `json:"baidu_enabled"`
			BaiduAK      string `json:"baidu_ak"`
			BaiduSK      string `json:"baidu_sk"`
			BaiduBucket  string `json:"baidu_bucket"`
			DataPath     string `json:"data_path"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			errorResponse(w, err)
			return
		}

		cfg.LLM.Provider = req.LLMProvider
		if req.LLMAPIKey != "" {
			cfg.LLM.APIKey = req.LLMAPIKey
		}
		cfg.LLM.Model = req.LLMModel
		cfg.LLM.BaseURL = req.LLMBaseURL
		cfg.BaiduCloud.Enabled = req.BaiduEnabled
		if req.BaiduAK != "" {
			cfg.BaiduCloud.AK = req.BaiduAK
		}
		if req.BaiduSK != "" {
			cfg.BaiduCloud.SK = req.BaiduSK
		}
		cfg.BaiduCloud.Bucket = req.BaiduBucket
		if req.DataPath != "" {
			cfg.DataPath = req.DataPath
		}

		if err := config.Save(cfg); err != nil {
			errorResponse(w, err)
			return
		}

		jsonResponse(w, map[string]string{"status": "ok"})
	}
}

func handleGetFolders(db *data.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		folders, err := db.GetFolders()
		if err != nil {
			errorResponse(w, err)
			return
		}
		jsonResponse(w, map[string]interface{}{
			"folders": folders,
		})
	}
}

func handleCreateFolder(db *data.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			errorResponse(w, err)
			return
		}

		id, err := db.CreateFolder(req.Name, req.Description)
		if err != nil {
			errorResponse(w, err)
			return
		}

		jsonResponse(w, map[string]interface{}{
			"id": id,
		})
	}
}

func handleGetPapers(db *data.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		folderIDStr := r.URL.Query().Get("folder_id")
		if folderIDStr == "" {
			errorResponse(w, nil)
			return
		}
		folderID, err := strconv.ParseInt(folderIDStr, 10, 64)
		if err != nil {
			errorResponse(w, err)
			return
		}

		papers, err := db.GetPapersByFolder(folderID)
		if err != nil {
			errorResponse(w, err)
			return
		}

		jsonResponse(w, map[string]interface{}{
			"papers": papers,
		})
	}
}

func handleGetPaper(db *data.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get paper ID from URL: /api/paper/123
		pathParts := strings.Split(r.URL.Path, "/")
		if len(pathParts) < 4 {
			errorResponse(w, nil)
			return
		}
		paperID, err := strconv.ParseInt(pathParts[3], 10, 64)
		if err != nil {
			errorResponse(w, err)
			return
		}

		paper, err := db.GetPaper(paperID)
		if err != nil {
			errorResponse(w, err)
			return
		}

		jsonResponse(w, paper)
	}
}

type DeepStartRequest struct {
	Query string `json:"query"`
}

type DeepStartResponse struct {
	Results []search.SearchResult `json:"results"`
	Classification []llm.PaperClassification `json:"classification"`
}

func handleDeepStart(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req DeepStartRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			errorResponse(w, err)
			return
		}

		llmClient := llm.NewClient(cfg.LLM)
		searcher := search.NewSearcher(llmClient)

		// Search for papers
		results, err := searcher.Search(req.Query, 30)
		if err != nil {
			errorResponse(w, err)
			return
		}

		// Convert to LLM paper info for classification
		var paperInfos []llm.PaperInfo
		for _, res := range results {
			paperInfos = append(paperInfos, llm.PaperInfo{
				Title:    res.Title,
				Authors:  res.Authors,
				Abstract: res.Abstract,
			})
		}

		// Classify using LLM
		classification, err := llmClient.ClassifyPapers(req.Query, paperInfos)
		if err != nil {
			log.Printf("LLM classification warning: %v", err)
			// Still return search results even if classification fails
		}

		jsonResponse(w, DeepStartResponse{
			Results: results,
			Classification: classification,
		})
	}
}

type ImportRequest struct {
	Papers []struct {
		Title    string `json:"title"`
		Authors  string `json:"authors"`
		Abstract string `json:"abstract"`
		URL      string `json:"url"`
		Category string `json:"category"`
	} `json:"papers"`
	FolderID int64  `json:"folder_id"`
}

func handleImportPapers(db *data.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req ImportRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			errorResponse(w, err)
			return
		}

		var imported []int64
		for _, p := range req.Papers {
			paper := &data.Paper{
				Title:    p.Title,
				Authors:  p.Authors,
				Abstract: p.Abstract,
				URL:      p.URL,
				FolderID: req.FolderID,
				Category: p.Category,
			}
			id, err := db.InsertPaper(paper)
			if err != nil {
				log.Printf("Failed to import paper %s: %v", p.Title, err)
				continue
			}
			imported = append(imported, id)
		}

		jsonResponse(w, map[string]interface{}{
			"imported_count": len(imported),
			"imported_ids":   imported,
		})
	}
}

type DeepReadRequest struct {
	PaperID int64  `json:"paper_id"`
	Section string `json:"section"`
	Text    string `json:"text"`
}

func handleDeepRead(db *data.DB, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req DeepReadRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			errorResponse(w, err)
			return
		}

		llmClient := llm.NewClient(cfg.LLM)
		translated, summary, err := llmClient.TranslateAndSummarize(req.Section, req.Text)
		if err != nil {
			errorResponse(w, err)
			return
		}

		// Save to database
		err = db.SaveTranslation(req.PaperID, req.Section, req.Text, translated, summary)
		if err != nil {
			log.Printf("Warning: failed to save translation: %v", err)
		}

		jsonResponse(w, map[string]interface{}{
			"translation": translated,
			"summary":     summary,
		})
	}
}

func handleGetTranslations(db *data.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		paperIDStr := r.URL.Query().Get("paper_id")
		paperID, err := strconv.ParseInt(paperIDStr, 10, 64)
		if err != nil {
			errorResponse(w, err)
			return
		}

		translations, err := db.GetTranslationsForPaper(paperID)
		if err != nil {
			errorResponse(w, err)
			return
		}

		jsonResponse(w, map[string]interface{}{
			"translations": translations,
		})
	}
}

func handleBaiduSync(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !cfg.BaiduCloud.Enabled {
			errorResponse(w, nil)
			jsonResponse(w, map[string]interface{}{
				"error": "Baidu Cloud sync not enabled",
			})
			return
		}

		client := sync.NewBaiduClient(cfg.BaiduCloud)
		err := client.SyncDirectory(cfg.DataPath, cfg.BaiduCloud.RemoteDir)
		if err != nil {
			errorResponse(w, err)
			return
		}

		jsonResponse(w, map[string]interface{}{
			"status": "ok",
			"message": "Sync completed",
		})
	}
}
