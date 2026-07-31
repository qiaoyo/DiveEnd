package httpx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// ReadLimitedHTTPBody reads at most limit bytes and rejects oversized bodies.
func ReadLimitedHTTPBody(reader io.Reader, limit int64) ([]byte, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("response body byte limit must be positive")
	}
	body, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("response body exceeds %d byte limit", limit)
	}
	return body, nil
}

// DecodeLimitedJSON decodes exactly one JSON value from a bounded response.
func DecodeLimitedJSON(reader io.Reader, limit int64, useNumber bool, target any) error {
	body, err := ReadLimitedHTTPBody(reader, limit)
	if err != nil {
		return err
	}

	decoder := json.NewDecoder(bytes.NewReader(body))
	if useNumber {
		decoder.UseNumber()
	}
	if err := decoder.Decode(target); err != nil {
		return err
	}

	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("response body contains multiple JSON values")
		}
		return err
	}
	return nil
}
