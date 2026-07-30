package main

import (
	"os"
	"strings"
	"testing"
)

func TestRealStrongAndWeakLLMBudgetE2E(t *testing.T) {
	if strings.TrimSpace(os.Getenv("DIVEEND_REAL_LLM_E2E")) != "1" {
		t.Skip("set DIVEEND_REAL_LLM_E2E=1 to run against configured strong and weak LLM providers")
	}

	strongSeed, ok := readStrongLLMSeed()
	if !ok {
		t.Fatal("config/strong_llm.json is unavailable or incomplete")
	}
	weakSeed, ok := readWeakLLMSeed()
	if !ok {
		t.Fatal("config/weak_llm.json is unavailable or incomplete")
	}

	dataPath := t.TempDir()
	budget := newDailyTokenBudget(dataPath, defaultDailyLLMTokenBudget)
	clients := []*LLMClient{
		newLLMClientFromConfig(llmConfigFromSeed(strongSeed)),
		newLLMClientFromConfig(llmConfigFromSeed(weakSeed)),
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
