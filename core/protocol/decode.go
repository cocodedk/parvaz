package protocol

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
)

// DecodeResponse parses a single-mode response envelope. A response carrying
// the "e" field becomes a *ServerError.
func DecodeResponse(data []byte) (*Response, error) {
	var env responseEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if env.E != "" {
		return nil, &ServerError{Message: env.E}
	}
	body, err := base64.StdEncoding.DecodeString(env.B)
	if err != nil {
		return nil, fmt.Errorf("decode body: %w", err)
	}
	return &Response{Status: env.S, Header: unflattenHeaders(env.H), Body: body}, nil
}

// DecodeBatchResponse parses a batch-mode response envelope. Per-item errors
// are preserved in order via BatchItemResult.Err. A top-level "e" field
// (whole-batch failure) returns an error instead.
func DecodeBatchResponse(data []byte) (*BatchResponse, error) {
	var env batchResponseEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("parse batch response: %w", err)
	}
	if env.E != "" {
		return nil, &ServerError{Message: env.E}
	}
	out := &BatchResponse{Items: make([]BatchItemResult, len(env.Q))}
	for i, item := range env.Q {
		if item.E != "" {
			out.Items[i] = BatchItemResult{Err: &ServerError{Message: item.E}}
			continue
		}
		body, err := base64.StdEncoding.DecodeString(item.B)
		if err != nil {
			out.Items[i] = BatchItemResult{Err: fmt.Errorf("decode item %d body: %w", i, err)}
			continue
		}
		out.Items[i] = BatchItemResult{
			Response: &Response{Status: item.S, Header: unflattenHeaders(item.H), Body: body},
		}
	}
	return out, nil
}

func unflattenHeaders(h respHeaders) http.Header {
	out := make(http.Header, len(h))
	for k, values := range h {
		for _, v := range values {
			out.Add(k, v)
		}
	}
	return out
}
