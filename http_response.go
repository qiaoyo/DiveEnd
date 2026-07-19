package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

var externalHTTPBodyLimitBytes int64 = 16 * 1024 * 1024
var pdfServiceResponseBodyLimitBytes int64 = 128 * 1024 * 1024
var serviceErrorBodyLimitBytes int64 = 64 * 1024

func readExternalHTTPBody(reader io.Reader) ([]byte, error) {
	return readLimitedHTTPBody(reader, externalHTTPBodyLimitBytes)
}

func readServiceErrorHTTPBody(reader io.Reader) ([]byte, error) {
	return readLimitedHTTPBody(reader, serviceErrorBodyLimitBytes)
}

func decodeExternalJSON(reader io.Reader, target any) error {
	return decodeLimitedJSON(reader, externalHTTPBodyLimitBytes, false, target)
}

func decodeExternalJSONUseNumber(reader io.Reader, target any) error {
	return decodeLimitedJSON(reader, externalHTTPBodyLimitBytes, true, target)
}

func decodePDFServiceJSON(reader io.Reader, target any) error {
	return decodeLimitedJSON(reader, pdfServiceResponseBodyLimitBytes, false, target)
}

func readLimitedHTTPBody(reader io.Reader, limit int64) ([]byte, error) {
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

func decodeLimitedJSON(reader io.Reader, limit int64, useNumber bool, target any) error {
	body, err := readLimitedHTTPBody(reader, limit)
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
