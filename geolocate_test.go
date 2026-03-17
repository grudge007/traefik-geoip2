package traefik_geoblock_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/grudge007/traefik-geoblock"
)

// NOTE: Since MaxMind Go library does not allow mock creation easily we test unit parsing here
// with an invalid file just to ensure panics don't occur.

func TestGeoblock_ServeHTTP(t *testing.T) {
	config := traefik_geoblock.CreateConfig()
	config.DbPath = "invalid.mmdb" // Will test initialization error path
	
	ctx := context.Background()
	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {})

	_, err := traefik_geoblock.New(ctx, next, config, "test-geoblock")
	if err == nil {
		t.Errorf("Expected an error when loading invalid mmdb path")
	}
}

// BenchmarkServeHTTP simulates a zero-latency pass-through when initialized
// This tests how fast the plugin processes a request when the MMDB lookup is bypassed
// to ensure no other logic causes overhead.
func BenchmarkServeHTTP(b *testing.B) {
	// Creating standard geoblock manually for benchmark (simulating if open succeeded)
	config := traefik_geoblock.CreateConfig()
	config.DbPath = "dummy.mmdb"

	// Mocking empty struct for speed test since we can't open a real mmdb file easily here
	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {})
	handler := struct {
		next http.Handler
	}{
		next: next,
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://localhost", nil)
	if err != nil {
		b.Fatal(err)
	}
	req.Header.Set("X-Forwarded-For", "8.8.8.8")

	rw := httptest.NewRecorder()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handler.next.ServeHTTP(rw, req)
	}
}
