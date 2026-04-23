package codec

import (
	"bytes"
	"compress/gzip"
	"strings"
	"testing"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
)

func gzipBytes(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func brBytes(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := brotli.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func zstdBytes(t *testing.T, data []byte) []byte {
	t.Helper()
	w, err := zstd.NewWriter(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	return w.EncodeAll(data, nil)
}

func TestDecode_Identity(t *testing.T) {
	orig := []byte("hello world")
	for _, enc := range []string{"", "identity", "IDENTITY"} {
		got, err := Decode(orig, enc)
		if err != nil {
			t.Errorf("encoding %q: %v", enc, err)
			continue
		}
		if !bytes.Equal(got, orig) {
			t.Errorf("encoding %q: got %q want %q", enc, got, orig)
		}
	}
}

func TestDecode_Gzip(t *testing.T) {
	orig := []byte(strings.Repeat("the quick brown fox ", 10))
	compressed := gzipBytes(t, orig)
	got, err := Decode(compressed, "gzip")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, orig) {
		t.Errorf("gzip decode mismatch")
	}
	// Case-insensitive + x-gzip alias
	got2, err := Decode(compressed, "x-gzip")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got2, orig) {
		t.Errorf("x-gzip decode mismatch")
	}
}

func TestDecode_Brotli(t *testing.T) {
	orig := []byte(strings.Repeat("parvaz parvaz parvaz ", 10))
	compressed := brBytes(t, orig)
	got, err := Decode(compressed, "br")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, orig) {
		t.Errorf("brotli decode mismatch")
	}
}

func TestDecode_Zstd(t *testing.T) {
	orig := []byte(strings.Repeat("zstd-data ", 20))
	compressed := zstdBytes(t, orig)
	got, err := Decode(compressed, "zstd")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, orig) {
		t.Errorf("zstd decode mismatch")
	}
}

func TestDecode_UnknownEncoding_ReturnsError(t *testing.T) {
	_, err := Decode([]byte("anything"), "lzma")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "lzma") {
		t.Errorf("error = %v, want one mentioning 'lzma'", err)
	}
}

func TestDecode_Chained_gzipThenBr(t *testing.T) {
	// Data was gzip'd then br'd ⇒ Content-Encoding: "gzip, br".
	// Decode order is reverse: br first, then gzip.
	orig := []byte(strings.Repeat("chain-me ", 15))
	chained := brBytes(t, gzipBytes(t, orig))
	got, err := Decode(chained, "gzip, br")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, orig) {
		t.Errorf("chained decode mismatch: got %q want %q", got, orig)
	}
	// Whitespace tolerance
	got2, err := Decode(chained, " gzip ,  br ")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got2, orig) {
		t.Errorf("chained decode with whitespace mismatch")
	}
}
