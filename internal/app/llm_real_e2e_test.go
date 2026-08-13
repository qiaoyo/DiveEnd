package app

import (
	"os"
	"strings"
	"testing"
)

func TestRealStrongAndWeakLLMBudgetE2E(t *testing.T) {
	if strings.TrimSpace(os.Getenv("DIVEEND_REAL_LLM_E2E")) != "1" {
		t.Skip("set DIVEEND_REAL_LLM_E2E=1 to run against configured strong and weak LLM providers")
	}

	config, err := LoadAppConfig()
	if err != nil {
		t.Fatalf("load configured LLMs: %v", err)
	}
	config = normalizeAppConfig(config)
	if isVolcengineBaseURL(config.LLM.BaseURL) && config.LLM.WireAPI != "chat_completions" {
		t.Fatalf("strong Volcengine config selected the wrong wire API: %q", config.LLM.WireAPI)
	}
	if isVolcengineBaseURL(config.WeakLLM.BaseURL) && config.WeakLLM.WireAPI != "chat_completions" {
		t.Fatalf("weak Volcengine config selected the wrong wire API: %q", config.WeakLLM.WireAPI)
	}
	t.Logf("configured LLM routes: strong=%s/%s weak=%s/%s", config.LLM.ProviderName, config.LLM.WireAPI, config.WeakLLM.ProviderName, config.WeakLLM.WireAPI)

	dataPath := t.TempDir()
	budget := newDailyTokenBudget(dataPath, defaultDailyLLMTokenBudget)
	clients := []*LLMClient{
		NewStrongLLMClient(config),
		NewWeakLLMClient(config),
	}
	for _, client := range clients {
		client.tokenBudget = budget
		response, err := client.chat([]llmMessage{{
			Role:    "user",
			Content: `Return exactly this JSON object and nothing else: {"ok":true}`,
		}})
		if err != nil {
			t.Fatalf("%s real request failed: %v", client.safeProviderLabel(), err)
		}
		if !strings.Contains(response, `"ok"`) {
			t.Fatalf("%s returned an unexpected response shape", client.safeProviderLabel())
		}
	}

	snapshot := budget.Snapshot()
	if snapshot.UsedTokens <= 0 || snapshot.UsedTokens >= snapshot.Limit {
		t.Fatalf("unexpected real usage snapshot: %+v", snapshot)
	}
	reloaded := newDailyTokenBudget(dataPath, defaultDailyLLMTokenBudget).Snapshot()
	if reloaded.UsedTokens != snapshot.UsedTokens {
		t.Fatalf("expected usage to persist across restart: before=%+v after=%+v", snapshot, reloaded)
	}
}
