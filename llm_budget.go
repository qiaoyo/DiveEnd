package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const defaultDailyLLMTokenBudget int64 = 100_000_000

type LLMUsageSnapshot struct {
	Date       string `json:"date"`
	UsedTokens int64  `json:"usedTokens"`
	Limit      int64  `json:"limit"`
	Remaining  int64  `json:"remaining"`
}

type dailyTokenBudget struct {
	mu         sync.Mutex
	path       string
	limit      int64
	date       string
	usedTokens int64
}

type tokenReservation struct {
	budget   *dailyTokenBudget
	reserved int64
	date     string
	done     bool
}

type persistedLLMUsage struct {
	Date       string `json:"date"`
	UsedTokens int64  `json:"usedTokens"`
}

func newDailyTokenBudget(dataPath string, limit int64) *dailyTokenBudget {
	if limit <= 0 {
		limit = defaultDailyLLMTokenBudget
	}
	budget := &dailyTokenBudget{
		path:  filepath.Join(dataPath, ".diveend", "llm_usage.json"),
		limit: limit,
		date:  localUsageDate(time.Now()),
	}
	budget.load()
	return budget
}

func (b *dailyTokenBudget) Reserve(messages []llmMessage, maxOutputTokens int64) (*tokenReservation, error) {
	if b == nil {
		return &tokenReservation{}, nil
	}
	if maxOutputTokens < 0 {
		maxOutputTokens = 0
	}
	requestTokens := estimateMessageTokens(messages) + maxOutputTokens
	if requestTokens < 1 {
		requestTokens = 1
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	b.rolloverLocked(time.Now())
	if requestTokens > b.limit-b.usedTokens {
		return nil, fmt.Errorf(
			"daily LLM token budget exceeded: %d of %d tokens used, request requires up to %d tokens",
			b.usedTokens,
			b.limit,
			requestTokens,
		)
	}
	b.usedTokens += requestTokens
	b.persistLocked()
	return &tokenReservation{budget: b, reserved: requestTokens, date: b.date}, nil
}

func (r *tokenReservation) Finish(actualTokens int64) {
	if r == nil || r.done || r.budget == nil {
		return
	}
	r.done = true
	if actualTokens < 1 {
		actualTokens = r.reserved
	}
	r.budget.mu.Lock()
	defer r.budget.mu.Unlock()
	r.budget.rolloverLocked(time.Now())
	if r.budget.date == r.date {
		r.budget.usedTokens += actualTokens - r.reserved
	} else {
		r.budget.usedTokens += actualTokens
	}
	if r.budget.usedTokens < 0 {
		r.budget.usedTokens = 0
	}
	r.budget.persistLocked()
}

func (r *tokenReservation) Cancel() {
	if r == nil || r.done || r.budget == nil {
		return
	}
	r.done = true
	r.budget.mu.Lock()
	defer r.budget.mu.Unlock()
	r.budget.rolloverLocked(time.Now())
	if r.budget.date == r.date {
		r.budget.usedTokens -= r.reserved
	}
	if r.budget.usedTokens < 0 {
		r.budget.usedTokens = 0
	}
	r.budget.persistLocked()
}

func (b *dailyTokenBudget) Snapshot() LLMUsageSnapshot {
	if b == nil {
		return LLMUsageSnapshot{}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rolloverLocked(time.Now())
	remaining := b.limit - b.usedTokens
	if remaining < 0 {
		remaining = 0
	}
	return LLMUsageSnapshot{
		Date:       b.date,
		UsedTokens: b.usedTokens,
		Limit:      b.limit,
		Remaining:  remaining,
	}
}

func (b *dailyTokenBudget) load() {
	data, err := os.ReadFile(b.path)
	if err != nil {
		return
	}
	var persisted persistedLLMUsage
	if json.Unmarshal(data, &persisted) != nil || persisted.Date != b.date || persisted.UsedTokens < 0 {
		return
	}
	if persisted.UsedTokens > b.limit {
		persisted.UsedTokens = b.limit
	}
	b.usedTokens = persisted.UsedTokens
}

func (b *dailyTokenBudget) rolloverLocked(now time.Time) {
	date := localUsageDate(now)
	if date == b.date {
		return
	}
	b.date = date
	b.usedTokens = 0
	b.persistLocked()
}

func (b *dailyTokenBudget) persistLocked() {
	data, err := json.MarshalIndent(persistedLLMUsage{Date: b.date, UsedTokens: b.usedTokens}, "", "  ")
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(b.path), 0700); err != nil {
		return
	}
	_ = writeFileAtomic(b.path, data, 0600)
}

func localUsageDate(now time.Time) string {
	return now.Format("2006-01-02")
}

func estimateMessageTokens(messages []llmMessage) int64 {
	var runes int
	for _, message := range messages {
		runes += utf8.RuneCountInString(strings.TrimSpace(message.Content)) + 8
	}
	if runes == 0 {
		return 0
	}
	return int64((runes + 3) / 4)
}
