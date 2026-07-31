package app

import (
	"io"

	"github.com/qiaoyo/DiveEnd/internal/platform/httpx"
	"github.com/qiaoyo/DiveEnd/internal/platform/redaction"
)

// These wrappers preserve the legacy root-package names while old Wails
// adapters are migrated. New internal packages should import platform helpers
// directly instead of adding more root-level utility functions.
var externalHTTPBodyLimitBytes int64 = 16 * 1024 * 1024
var pdfServiceResponseBodyLimitBytes int64 = 128 * 1024 * 1024
var serviceErrorBodyLimitBytes int64 = 64 * 1024

func readExternalHTTPBody(reader io.Reader) ([]byte, error) {
	return httpx.ReadLimitedHTTPBody(reader, externalHTTPBodyLimitBytes)
}

func readServiceErrorHTTPBody(reader io.Reader) ([]byte, error) {
	return httpx.ReadLimitedHTTPBody(reader, serviceErrorBodyLimitBytes)
}

func decodeExternalJSON(reader io.Reader, target any) error {
	return httpx.DecodeLimitedJSON(reader, externalHTTPBodyLimitBytes, false, target)
}

func decodeExternalJSONUseNumber(reader io.Reader, target any) error {
	return httpx.DecodeLimitedJSON(reader, externalHTTPBodyLimitBytes, true, target)
}

func decodePDFServiceJSON(reader io.Reader, target any) error {
	return httpx.DecodeLimitedJSON(reader, pdfServiceResponseBodyLimitBytes, false, target)
}

const redactedValue = redaction.RedactedValue
const maxUserVisibleErrorBytes = redaction.MaxUserVisibleErrorBytes

func redactSensitiveText(value string) string {
	return redaction.RedactSensitiveText(value)
}

func redactSensitiveValue(value any) string {
	return redaction.RedactSensitiveValue(value)
}

func redactURLQueryValuesInText(value string) string {
	return redaction.RedactURLQueryValuesInText(value)
}

func redactErrorText(err error) string {
	return redaction.RedactErrorText(err)
}
