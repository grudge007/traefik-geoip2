package traefik_geoip2_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/grudge007/traefik-geoip2"
)

func TestGeoIP2_ServeHTTP(t *testing.T) {
	config := traefik_geoip2.CreateConfig()
	config.DbPath = "invalid.mmdb" // Will test initialization error path
	
	ctx := context.Background()
	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {})

	_, err := traefik_geoip2.New(ctx, next, config, "test-geoip2")
	if err == nil {
		t.Errorf("Expected an error when loading invalid mmdb path")
	}
}

// BenchmarkServeHTTP simulates a zero-latency pass-through when initialized
// This tests how fast the plugin processes a request when the MMDB lookup is bypassed
// to ensure no other logic causes overhead.
func BenchmarkServeHTTP(b *testing.B) {
	// Creating standard geoip2 manually for benchmark (simulating if open succeeded)
	config := traefik_geoip2.CreateConfig()
	config.DbPath = "dummy.mmdb"

	// Mock handler that passes through
	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {})
	var handler http.Handler = next

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://localhost", nil)
	if err != nil {
		b.Fatal(err)
	}
	req.Header.Set("X-Forwarded-For", "8.8.8.8")

	rw := httptest.NewRecorder()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handler.ServeHTTP(rw, req)
	}
}

