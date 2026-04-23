// Package codec decompresses HTTP Content-Encoding payloads.
//
// Pure stream-decoder package: no network, no I/O beyond reading from a
// byte slice. Supports identity, gzip (stdlib), brotli (andybalholm/brotli),
// and zstd (klauspost/compress/zstd). Chained encodings per RFC 7231 §3.1.2.2
// are decoded in reverse of the order they were applied.
package codec

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"strings"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
)

// Decode applies the given Content-Encoding(s) to data in reverse order,
// returning the decompressed bytes. An empty or "identity" encoding is a
// passthrough.
func Decode(data []byte, contentEncoding string) ([]byte, error) {
	encodings := parseEncodings(contentEncoding)
	// RFC 7231: encodings are listed in the order applied. Decode in reverse.
	for i := len(encodings) - 1; i >= 0; i-- {
		out, err := decodeOne(data, encodings[i])
		if err != nil {
			return nil, err
		}
		data = out
	}
	return data, nil
}

func parseEncodings(header string) []string {
	if header == "" {
		return nil
	}
	parts := strings.Split(header, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(strings.ToLower(p))
		if p == "" || p == "identity" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func decodeOne(data []byte, enc string) ([]byte, error) {
	switch enc {
	case "gzip", "x-gzip":
		return decodeGzip(data)
	case "br":
		return decodeBrotli(data)
	case "zstd":
		return decodeZstd(data)
	default:
		return nil, fmt.Errorf("codec: unknown Content-Encoding %q", enc)
	}
}

func decodeGzip(data []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("codec: gzip init: %w", err)
	}
	defer r.Close()
	return io.ReadAll(r)
}

func decodeBrotli(data []byte) ([]byte, error) {
	return io.ReadAll(brotli.NewReader(bytes.NewReader(data)))
}

func decodeZstd(data []byte) ([]byte, error) {
	r, err := zstd.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("codec: zstd init: %w", err)
	}
	defer r.Close()
	return io.ReadAll(r)
}
