package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/qiaoyo/DiveEnd/internal/contracts"
)

type deepStartQueryRewriter = contracts.QueryRewriter

func (a *App) rewriteDeepStartQueries(ctx context.Context, query string) ([]string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return []string{""}, nil
	}

	weak := a.currentWeakLLM()
	if weak == nil {
		return nil, deepStartAIUnavailableError("弱模型未初始化，无法完成 query 重写", nil)
	}

	for attempt := 1; attempt <= 2; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		var (
			rewritten []string
			err       error
		)
		if rewriter, ok := weak.(contextQueryRewriter); ok {
			rewritten, err = rewriter.RewriteSearchQueriesWithContext(ctx, query)
		} else if rewriter, ok := weak.(deepStartQueryRewriter); ok {
			rewritten, err = rewriter.RewriteSearchQueries(query)
		} else {
			return nil, deepStartAIUnavailableError("弱模型不支持 query 重写", nil)
		}
		queries := compactStrings(uniqueStrings(rewritten), 3)
		if err == nil && len(queries) > 0 {
			return queries, nil
		}
		if err == nil {
			err = fmt.Errorf("弱模型返回了空的检索词")
		}
		if attempt == 2 || !isRetryableLLMError(err) {
			return nil, deepStartAIUnavailableError("query 重写失败", err)
		}
		if err := sleepWithContext(ctx, 500*time.Millisecond); err != nil {
			return nil, err
		}
	}
	return nil, deepStartAIUnavailableError("query 重写失败", nil)
}

func (a *App) deepStartSearchWithRewrittenQueries(
	ctx context.Context,
	originalQuery string,
	limit int,
	perSourceLimit int,
) ([]SearchPaper, string, SearchRetrievalStats, error) {
	stats := SearchRetrievalStats{
		Query:         strings.TrimSpace(originalQuery),
		OriginalQuery: strings.TrimSpace(originalQuery),
		QueryHits:     map[string]int{},
	}
	if a.search == nil {
		return []SearchPaper{}, "搜索服务当前不可用。", stats, nil
	}

	queries, rewriteErr := a.rewriteDeepStartQueries(ctx, originalQuery)
	if rewriteErr != nil {
		return nil, "", stats, rewriteErr
	}
	stats.RewrittenQueries = append([]string{}, queries...)

	results := make([]SearchPaper, 0, limit)
	var warningParts []string

	totalRaw := 0
	lastStats := SearchRetrievalStats{Query: strings.TrimSpace(originalQuery)}
	successfulQueries := 0
	failedQueries := make([]string, 0, len(queries))
	queryLimit := limit
	if len(queries) > 1 && limit <= 50 {
		queryLimit = ((limit + len(queries) - 1) / len(queries)) * 2
		if queryLimit < 8 {
			queryLimit = 8
		}
		if queryLimit > limit {
			queryLimit = limit
		}
	}

	for _, rewrittenQuery := range queries {
		if err := ctx.Err(); err != nil {
			return nil, "", stats, ErrDeepStartTaskCancelled
		}

		var (
			papers []SearchPaper
			err    error
		)
		if configurable, ok := a.search.(interface {
			SearchWithPerSourceLimit(ctx context.Context, query string, limit int, perSourceLimit int) ([]SearchPaper, error)
		}); ok && perSourceLimit > 0 {
			papers, err = configurable.SearchWithPerSourceLimit(ctx, rewrittenQuery, queryLimit, perSourceLimit)
		} else {
			papers, err = a.search.SearchWithContext(ctx, rewrittenQuery, queryLimit)
		}
		queryStats := a.search.LastSearchStats()
		if strings.TrimSpace(queryStats.Query) == "" {
			queryStats.Query = rewrittenQuery
		}
		if queryStats.RawCount == 0 && len(papers) > 0 {
			queryStats.RawCount = len(papers)
		}
		if queryStats.DedupCount == 0 && len(papers) > 0 {
			queryStats.DedupCount = len(papers)
		}
		if queryStats.FinalCount == 0 && len(papers) > 0 {
			queryStats.FinalCount = len(papers)
		}

		lastStats = queryStats
		totalRaw += queryStats.RawCount

		if err != nil {
			if isDeepStartCancelledError(err) {
				return nil, "", stats, ErrDeepStartTaskCancelled
			}
			failedQueries = append(failedQueries, fmt.Sprintf("%s: %v", rewrittenQuery, err))
			stats.QueryHits[rewrittenQuery] = 0
			continue
		}

		successfulQueries++
		stats.QueryHits[rewrittenQuery] = len(papers)
		results = mergeSearchPaperPools(results, papers)
	}

	results = rankSearchPapersByQueries(results, append([]string{originalQuery}, queries...))
	dedupCount := len(results)
	if len(results) > limit {
		results = results[:limit]
	}
	stats.Query = strings.TrimSpace(firstNonEmpty(lastStats.Query, originalQuery))
	stats.RawCount = totalRaw
	stats.DedupCount = dedupCount
	stats.FinalCount = len(results)
	if stats.RawCount == 0 && len(results) > 0 {
		stats.RawCount = len(results)
	}
	if len(stats.QueryHits) == 0 {
		stats.QueryHits = map[string]int{}
	}

	if len(failedQueries) > 0 {
		warningParts = append(warningParts, "部分重写 query 失败："+strings.Join(failedQueries, " | "))
	}
	if successfulQueries == 0 && len(results) == 0 {
		if len(warningParts) == 0 {
			warningParts = append(warningParts, "本轮检索暂时失败")
		}
		return []SearchPaper{}, strings.Join(warningParts, " "), stats, nil
	}

	return results, strings.Join(warningParts, " "), stats, nil
}
