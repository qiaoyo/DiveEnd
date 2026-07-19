package main

import (
	"strings"
	"testing"
)

func TestRedactSensitiveText(t *testing.T) {
	input := `request failed Authorization: Bearer bearer-secret-12345 https://example.test?access_token=access-secret&refresh_token=refresh-secret api_key=sk-inline-secret client_secret="client-secret" {"token":"json-token"}`

	redacted := redactSensitiveText(input)

	for _, leaked := range []string{
		"bearer-secret-12345",
		"access-secret",
		"refresh-secret",
		"sk-inline-secret",
		"client-secret",
		"json-token",
	} {
		if strings.Contains(redacted, leaked) {
			t.Fatalf("expected %q to be redacted from %q", leaked, redacted)
		}
	}
	if !strings.Contains(redacted, redactedValue) {
		t.Fatalf("expected redacted marker in %q", redacted)
	}
}

func TestRedactSensitiveTextBoundsLargeErrors(t *testing.T) {
	redacted := redactSensitiveText(strings.Repeat("x", 2000))

	if len(redacted) > maxUserVisibleErrorBytes+len("... [truncated]") {
		t.Fatalf("expected bounded error, got length %d", len(redacted))
	}
	if !strings.Contains(redacted, "[truncated]") {
		t.Fatalf("expected truncation marker, got %q", redacted)
	}
}

func TestRedactURLQueryValuesInText(t *testing.T) {
	input := `Get "https://example.test/search?query=private+research&access_token=url-token-secret&page=1": failed`

	redacted := redactURLQueryValuesInText(input)

	for _, leaked := range []string{"private+research", "url-token-secret", "page=1"} {
		if strings.Contains(redacted, leaked) {
			t.Fatalf("expected %q to be redacted from %q", leaked, redacted)
		}
	}
	if !strings.Contains(redacted, "query=%5Bredacted%5D") {
		t.Fatalf("expected generic query value to be redacted in %q", redacted)
	}
}
