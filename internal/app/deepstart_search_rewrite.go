package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/qiaoyo/DiveEnd/internal/contracts"
)

type deepStartQueryRewriter = contracts.QueryRewriter

func (a *App) rewriteDeepStartQueries(ctx context.Context, query string) ([]string, string) {
	query = strings.TrimSpace(query)
	if query == "" {
		return []string{""}, ""
	}

	fallback := fallbackDeepStartRewriteQueries(query)
	if weak := a.currentWeakLLM(); weak != nil {
		if err := ctx.Err(); err != nil {
			return fallback, fmt.Sprintf("弱模型 query 重写已取消，已回退：%v", err)
		}
		if rewriter, ok := weak.(contextQueryRewriter); ok {
			rewritten, err := rewriter.RewriteSearchQueriesWithContext(ctx, query)
			if err == nil {
				queries := uniqueStrings(append(compactStrings(rewritten, 3), fallback...))
				if len(queries) > 3 {
					queries = queries[:3]
				}
				if len(queries) > 0 {
					return queries, ""
				}
			}
			queries := fallback
			if len(queries) > 0 {
				return queries, fmt.Sprintf("弱模型 query 重写失败，已回退：%v", err)
			}
			return []string{query}, fmt.Sprintf("弱模型 query 重写失败，已回退原始 query：%v", err)
		}
		if rewriter, ok := weak.(deepStartQueryRewriter); ok {
			rewritten, err := rewriter.RewriteSearchQueries(query)
			if err == nil {
				queries := uniqueStrings(append(compactStrings(rewritten, 3), fallback...))
				if len(queries) > 3 {
					queries = queries[:3]
				}
				if len(queries) > 0 {
					return queries, ""
				}
			}
			queries := fallback
			if len(queries) > 0 {
				return queries, fmt.Sprintf("弱模型 query 重写失败，已回退：%v", err)
			}
			return []string{query}, fmt.Sprintf("弱模型 query 重写失败，已回退原始 query：%v", err)
		}
	}

	if len(fallback) > 0 {
		return fallback, ""
	}
	return []string{query}, ""
}

func fallbackDeepStartRewriteQueries(query string) []string {
	query = strings.TrimSpace(query)
	if query == "" {
		return []string{}
	}

	candidates := make([]string, 0, 4)
	asciiKeywords := extractASCIISearchKeywords(query, 8)
	if len(asciiKeywords) > 0 {
		candidates = append(candidates, strings.Join(asciiKeywords, " "))
	}

	for _, candidate := range buildSearchQueryCandidates(query) {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if isMostlyASCIIQuery(candidate) {
			candidates = append(candidates, candidate)
		}
	}

	if len(candidates) == 0 {
		candidates = append(candidates, query)
	}
	candidates = uniqueStrings(candidates)
	if len(candidates) > 3 {
		candidates = candidates[:3]
	}
	return candidates
}

func isMostlyASCIIQuery(query string) bool {
	query = strings.TrimSpace(query)
	if query == "" {
		return false
	}
	total := 0
	ascii := 0
	for _, r := range query {
		if r == ' ' || r == '\t' {
			continue
		}
		total++
		if r <= 127 {
			ascii++
		}
	}
	if total == 0 {
		return false
	}
	return ascii*100/total >= 70
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

	queries, rewriteWarning := a.rewriteDeepStartQueries(ctx, originalQuery)
	stats.RewrittenQueries = append([]string{}, queries...)

	results := make([]SearchPaper, 0, limit)
	var warningParts []string
	if strings.TrimSpace(rewriteWarning) != "" {
		warningParts = append(warningParts, strings.TrimSpace(rewriteWarning))
	}

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
