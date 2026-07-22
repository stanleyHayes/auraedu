package httpx

import (
	"encoding/json"
	"errors"
	"io"
)

const MaxInternalJSONResponseBytes int64 = 1 << 20

var ErrResponseTooLarge = errors.New("httpx: response body too large")

// DecodeJSONResponse decodes a trusted-shape internal service response without
// allowing a broken or compromised dependency to drive unbounded allocation.
// json.Unmarshal also rejects a second value or non-whitespace trailing data.
func DecodeJSONResponse(body io.Reader, destination any) error {
	data, err := io.ReadAll(io.LimitReader(body, MaxInternalJSONResponseBytes+1))
	if err != nil {
		return err
	}
	if int64(len(data)) > MaxInternalJSONResponseBytes {
		return ErrResponseTooLarge
	}
	return json.Unmarshal(data, destination)
}
