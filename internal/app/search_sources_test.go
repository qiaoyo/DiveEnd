package app

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func singleSourceSearchTestClient(source string) *SearchClient {
	config := defaultAppConfig()
	config.Search.EnableSemanticScholar = false
	config.Search.EnableArxiv = false
	config.Search.EnableOpenAlex = source == "openalex"
	config.Search.EnableOpenReview = source == "openreview"
	config.Search.EnableDBLP = source == "dblp"
	config.Search.OpenAlexAPIKey = "openalex-test-key"
	client := NewSearchClient(config)
	client.retryMax = 1
	return client
}

func TestSearchOpenAlexParsesAbstractIdentifiersAndOALocation(t *testing.T) {
	client := singleSourceSearchTestClient("openalex")
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host != "api.openalex.org" {
			t.Fatalf("unexpected host %q", request.URL.Host)
		}
		if request.URL.Query().Get("api_key") != "openalex-test-key" {
			t.Fatal("expected OpenAlex key in request")
		}
		if !strings.Contains(request.URL.Query().Get("select"), "abstract_inverted_index") {
			t.Fatal("expected a bounded OpenAlex field selection")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(bytes.NewBufferString(`{
  "results": [{
    "id": "https://openalex.org/W123",
    "display_name": "Reliable Planning for Language Model Agents",
    "publication_year": 2025,
    "cited_by_count": 42,
    "doi": "https://doi.org/10.1000/agent.1",
    "ids": {"openalex":"https://openalex.org/W123","doi":"https://doi.org/10.1000/agent.1"},
    "abstract_inverted_index": {"Reliable":[0],"agent":[1],"planning":[2]},
    "type": "article",
    "authorships": [{
      "author":{"display_name":"Ada Researcher"},
      "institutions":[{"display_name":"Example University"}]
    }],
    "primary_location": {
      "landing_page_url":"https://doi.org/10.1000/agent.1",
      "pdf_url":null,
      "source":{"display_name":"ICLR"}
    },
    "best_oa_location": {
      "landing_page_url":"https://example.org/paper",
      "pdf_url":"https://example.org/paper.pdf"
    },
    "topics":[{"display_name":"Artificial Intelligence"}]
  }]
}`)),
			Request: request,
		}, nil
	})}

	papers, err := client.Search("language model agent planning", 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(papers) != 1 {
		t.Fatalf("expected one paper, got %+v", papers)
	}
	paper := papers[0]
	if paper.ID != "W123" || paper.ExternalIDs["DOI"] != "10.1000/agent.1" {
		t.Fatalf("unexpected identifiers: %+v", paper)
	}
	if paper.Abstract != "Reliable agent planning" {
		t.Fatalf("unexpected reconstructed abstract %q", paper.Abstract)
	}
	if len(paper.PDFCandidates) == 0 || paper.PDFCandidates[0] != "https://example.org/paper.pdf" {
		t.Fatalf("expected OA PDF candidate, got %+v", paper.PDFCandidates)
	}
}

func TestSearchOpenReviewPreservesForumStatusAndPDF(t *testing.T) {
	client := singleSourceSearchTestClient("openreview")
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host != "api2.openreview.net" {
			t.Fatalf("unexpected host %q", request.URL.Host)
		}
		if request.URL.Query().Get("source") != "forum" {
			t.Fatal("expected forum-level OpenReview search")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(bytes.NewBufferString(`{
  "notes": [{
    "id":"note-1",
    "forum":"forum-1",
    "content":{
      "title":{"value":"General Agent Evaluation"},
      "abstract":{"value":"A reliable benchmark for language model agents."},
      "authors":{"value":["A. Author","B. Author"]},
      "keywords":{"value":["agents","evaluation"]},
      "venue":{"value":"ICLR 2026 rejected submission"}
    }
  }]
}`)),
			Request: request,
		}, nil
	})}

	papers, err := client.Search("agent evaluation benchmark", 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(papers) != 1 || papers[0].ExternalIDs["OpenReview"] != "forum-1" {
		t.Fatalf("unexpected OpenReview result: %+v", papers)
	}
	if papers[0].Category != "" || containsString(papers[0].Tags, "ICLR 2026 rejected submission") {
		t.Fatalf("expected venue metadata to stay separate from directions and tags, got %+v", papers[0])
	}
	if papers[0].Authors != "A. Author, B. Author" || !containsString(papers[0].Keywords, "evaluation") {
		t.Fatalf("expected OpenReview authors and keywords, got %+v", papers[0])
	}
	if !strings.Contains(papers[0].Journal, "rejected") {
		t.Fatalf("expected venue status to remain visible, got %q", papers[0].Journal)
	}
	if len(papers[0].PDFCandidates) != 1 || !strings.Contains(papers[0].PDFCandidates[0], "forum-1") {
		t.Fatalf("expected forum PDF URL, got %+v", papers[0].PDFCandidates)
	}
}

func TestSearchDBLPParsesMetadataWithoutAbstract(t *testing.T) {
	client := singleSourceSearchTestClient("dblp")
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host != "dblp.org" {
			t.Fatalf("unexpected host %q", request.URL.Host)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(bytes.NewBufferString(`{
  "result":{"hits":{"hit":[{
    "info":{
      "authors":{"author":[{"text":"Ada Researcher"},{"text":"Lin Engineer"}]},
      "title":"Vision &amp; Language Agent Planning.",
      "venue":"ICRA",
      "year":"2025",
      "doi":"10.1000/robot.1",
      "url":"https://dblp.org/rec/conf/icra/example",
      "ee":"https://doi.org/10.1000/robot.1",
      "type":"Conference and Workshop Papers"
    }
  }]}}
}`)),
			Request: request,
		}, nil
	})}

	papers, err := client.Search("vision language agent planning", 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(papers) != 1 {
		t.Fatalf("expected one DBLP paper, got %+v", papers)
	}
	if papers[0].Title != "Vision & Language Agent Planning." || papers[0].Authors != "Ada Researcher, Lin Engineer" {
		t.Fatalf("unexpected DBLP metadata: %+v", papers[0])
	}
	if papers[0].ExternalIDs["DOI"] != "10.1000/robot.1" {
		t.Fatalf("expected normalized DOI, got %+v", papers[0].ExternalIDs)
	}
}

func TestSearchDBLPParsesSingleHitObject(t *testing.T) {
	client := singleSourceSearchTestClient("dblp")
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(bytes.NewBufferString(`{
  "result":{"hits":{"hit":{
    "info":{
      "authors":{"author":{"text":"Ada Researcher"}},
      "title":"A Single DBLP Hit",
      "venue":"ICML",
      "year":"2026",
      "url":"https://dblp.org/rec/conf/icml/example"
    }
  }}}
}`)),
			Request: request,
		}, nil
	})}

	papers, err := client.Search("a single dblp hit", 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(papers) != 1 || papers[0].Title != "A Single DBLP Hit" {
		t.Fatalf("expected one parsed DBLP paper, got %+v", papers)
	}
	if papers[0].Authors != "Ada Researcher" || papers[0].Year != 2026 {
		t.Fatalf("unexpected single-hit metadata: %+v", papers[0])
	}
}

func TestSearchDBLPTreatsEmptyResponseAsAvailableSource(t *testing.T) {
	client := singleSourceSearchTestClient("dblp")
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(bytes.NewBufferString(`{
  "result":{"hits":{"@total":"0"}}
}`)),
			Request: request,
		}, nil
	})}

	var finalProgress SearchProgressEvent
	client.SetProgressReporter(func(progress SearchProgressEvent) {
		if progress.Phase == "completed" {
			finalProgress = progress
		}
	})

	papers, err := client.Search("no matching dblp records", 10)
	if err != nil {
		t.Fatalf("expected empty DBLP response to be successful, got %v", err)
	}
	if len(papers) != 0 {
		t.Fatalf("expected no papers, got %+v", papers)
	}
	if len(finalProgress.Sources) != 1 || !finalProgress.Sources[0].Success ||
		finalProgress.Sources[0].Status != "success" || finalProgress.Sources[0].ResultCount != 0 {
		t.Fatalf("expected DBLP to be available with zero results, got %+v", finalProgress)
	}
}

func TestMergeDuplicateSearchPapersUsesStableIDsAndCombinesProvenance(t *testing.T) {
	merged := mergeDuplicateSearchPapers([]SearchPaper{
		{
			ID:          "W123",
			Title:       "A Unified Academic Search System",
			Authors:     "A. Author",
			Abstract:    "Detailed abstract.",
			Year:        2025,
			Source:      "openalex",
			Sources:     []string{"openalex"},
			ExternalIDs: map[string]string{"DOI": "10.1000/search.1"},
		},
		{
			ID:            "dblp-record",
			Title:         "A Unified Academic Search System",
			Authors:       "A. Author",
			Year:          2026,
			Journal:       "SIGIR",
			Source:        "dblp",
			Sources:       []string{"dblp"},
			ExternalIDs:   map[string]string{"DOI": "https://doi.org/10.1000/search.1"},
			PDFCandidates: []string{"https://doi.org/10.1000/search.1"},
		},
	})
	if len(merged) != 1 {
		t.Fatalf("expected one merged paper, got %+v", merged)
	}
	if len(merged[0].Sources) != 2 || !strings.Contains(merged[0].SourceLabel, "OpenAlex") || !strings.Contains(merged[0].SourceLabel, "DBLP") {
		t.Fatalf("expected combined provenance, got %+v", merged[0])
	}
	if merged[0].Journal != "SIGIR" || merged[0].Abstract == "" {
		t.Fatalf("expected complementary metadata, got %+v", merged[0])
	}
}

func TestMergeDuplicateSearchPapersPrefersAcceptedVenueOverCoRR(t *testing.T) {
	merged := mergeDuplicateSearchPapers([]SearchPaper{
		{
			ID:               "corr-version",
			Title:            "A Long Benchmark for Embodied Agents",
			Authors:          "Alice Researcher",
			Year:             2026,
			PublicationYear:  2026,
			PublicationVenue: "CoRR 2026",
			Journal:          "CoRR 2026",
			Source:           "openreview",
			Sources:          []string{"openreview"},
		},
		{
			ID:               "rss-version",
			Title:            "A Long Benchmark for Embodied Agents",
			Authors:          "Alice Researcher",
			Year:             2026,
			PublicationYear:  2026,
			PublicationVenue: "RSS 2026 RoboData Workshop",
			Journal:          "RSS 2026 RoboData Workshop",
			Source:           "openreview",
			Sources:          []string{"openreview"},
		},
	})
	if len(merged) != 1 {
		t.Fatalf("expected archive and venue versions to merge, got %+v", merged)
	}
	if merged[0].PublicationVenue != "RSS 2026 RoboData Workshop" {
		t.Fatalf("expected accepted venue to win over CoRR archive, got %q", merged[0].PublicationVenue)
	}
}

func TestSearchRetryDelayUsesBackoffAndRetryAfter(t *testing.T) {
	if got := searchRetryDelay(time.Second, 3, nil); got != 4*time.Second {
		t.Fatalf("expected exponential backoff, got %v", got)
	}
	err := &searchProviderHTTPError{
		statusCode: http.StatusTooManyRequests,
		retryAfter: 7 * time.Second,
	}
	if got := searchRetryDelay(time.Second, 2, err); got != 7*time.Second {
		t.Fatalf("expected Retry-After to win, got %v", got)
	}
}

func TestFilterSearchPapersUsesOneGeneralRelevanceRule(t *testing.T) {
	papers := []SearchPaper{
		{ID: "relevant", Title: "Language Model Agent Planning", Abstract: "A benchmark for planning agents."},
		{ID: "noise", Title: "Structural Equation Modeling", Abstract: "A statistics paper."},
	}
	filtered := filterSearchPapersByQueries(papers, []string{"language model agent planning"})
	if len(filtered) != 1 || filtered[0].ID != "relevant" {
		t.Fatalf("expected general relevance filtering to remove noise, got %+v", filtered)
	}

	sparse := filterSearchPapersByQueries(
		[]SearchPaper{{ID: "sparse", Title: "Retrieved Synonym", Abstract: ""}},
		[]string{"language model agent planning"},
	)
	if len(sparse) != 1 {
		t.Fatalf("expected conservative fallback for sparse metadata, got %+v", sparse)
	}
}

func TestSearchSourceCancellationStillUsesCallerContext(t *testing.T) {
	client := singleSourceSearchTestClient("openalex")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.SearchWithContext(ctx, "agent planning", 10); err == nil {
		t.Fatal("expected cancelled source search")
	}
}
