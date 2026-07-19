package main

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

const redactedValue = "[redacted]"
const maxUserVisibleErrorBytes = 1200

var sensitiveTextPatterns = []struct {
	pattern     *regexp.Regexp
	replacement string
}{
	{
		pattern:     regexp.MustCompile(`(?i)\b(Authorization\s*[:=]\s*)Bearer\s+([A-Za-z0-9._~+/=-]{8,})`),
		replacement: `${1}Bearer ` + redactedValue,
	},
	{
		pattern:     regexp.MustCompile(`(?i)\bBearer\s+([A-Za-z0-9._~+/=-]{8,})`),
		replacement: `Bearer ` + redactedValue,
	},
	{
		pattern:     regexp.MustCompile(`\b(sk-[A-Za-z0-9_-]{8,})\b`),
		replacement: redactedValue,
	},
	{
		pattern: regexp.MustCompile(
			`(?i)([?&](?:access_token|refresh_token|api_key|apikey|key|token|client_secret|clientsecret)=)([^&#\s]+)`,
		),
		replacement: `${1}` + redactedValue,
	},
	{
		pattern: regexp.MustCompile(
			`(?i)\b((?:api[_-]?key|apikey|access[_-]?token|refresh[_-]?token|client[_-]?secret|clientsecret|token)\s*[:=]\s*)(\[redacted\]|"[^"]*"|'[^']*'|[^\s,;)}\]]+)`,
		),
		replacement: `${1}` + redactedValue,
	},
	{
		pattern: regexp.MustCompile(
			`(?i)(["']?(?:api[_-]?key|apiKey|access[_-]?token|accessToken|refresh[_-]?token|refreshToken|client[_-]?secret|clientSecret|token)["']?\s*:\s*)(\[redacted\]|"[^"]*"|'[^']*'|[^\s,}]+)`,
		),
		replacement: `${1}` + redactedValue,
	},
}

var httpURLPattern = regexp.MustCompile(`https?://[^\s"'<>]+`)

func redactSensitiveText(value string) string {
	redacted := strings.TrimSpace(value)
	if redacted == "" {
		return ""
	}
	for _, rule := range sensitiveTextPatterns {
		redacted = rule.pattern.ReplaceAllString(redacted, rule.replacement)
	}
	if len(redacted) <= maxUserVisibleErrorBytes {
		return redacted
	}
	return redacted[:maxUserVisibleErrorBytes] + "... [truncated]"
}

func redactSensitiveValue(value any) string {
	return redactSensitiveText(fmt.Sprint(value))
}

func redactURLQueryValuesInText(value string) string {
	redacted := redactSensitiveText(value)
	if redacted == "" {
		return ""
	}
	return httpURLPattern.ReplaceAllStringFunc(redacted, func(rawURL string) string {
		parsed, err := url.Parse(rawURL)
		if err != nil || parsed.RawQuery == "" {
			return rawURL
		}
		query := parsed.Query()
		for key := range query {
			query[key] = []string{redactedValue}
		}
		parsed.RawQuery = query.Encode()
		return parsed.String()
	})
}

func redactErrorText(err error) string {
	if err == nil {
		return ""
	}
	return redactSensitiveText(err.Error())
}
