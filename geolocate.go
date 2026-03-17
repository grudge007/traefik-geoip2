// Package traefik_geoip2 provides a Traefik middleware that geolocates an IP
// and injects the resulting country code into the HTTP request headers.
package traefik_geoip2

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/IncSW/geoip2"
)

// Config holds the configuration for the GeoIP2 middleware.
type Config struct {
	DbPath     string `json:"dbPath,omitempty"`
	HeaderName string `json:"headerName,omitempty"`
}

// CreateConfig creates the default plugin configuration.
func CreateConfig() *Config {
	return &Config{
		DbPath:     "",
		HeaderName: "X-Geo-Country",
	}
}

// GeoIP2 is the Traefik middleware.
type GeoIP2 struct {
	next       http.Handler
	name       string
	dbReader   *geoip2.CountryReader
	headerName string
}

// New creates a new GeoIP2 middleware.
func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	if config.DbPath == "" {
		return nil, fmt.Errorf("dbPath cannot be empty")
	}

	headerName := config.HeaderName
	if headerName == "" {
		headerName = "X-Geo-Country"
	}

	reader, err := geoip2.NewCountryReaderFromFile(config.DbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open mmdb, path: %s, error: %w", config.DbPath, err)
	}

	return &GeoIP2{
		next:       next,
		name:       name,
		dbReader:   reader,
		headerName: headerName,
	}, nil
}

func (g *GeoIP2) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	// Quick path out if reader is missing
	if g.dbReader == nil {
		g.next.ServeHTTP(rw, req)
		return
	}

	clientIP := getClientIP(req)
	ip := net.ParseIP(clientIP)

	if ip != nil {
		record, err := g.dbReader.Lookup(ip)
		if err == nil && record != nil && record.Country.ISOCode != "" {
			req.Header.Set(g.headerName, record.Country.ISOCode)
		}
	}

	// Always pass to the next handler
	g.next.ServeHTTP(rw, req)
}

// getClientIP extracts the client IP address from the request.
// It prioritizes the X-Forwarded-For header if present, falling back to RemoteAddr.
func getClientIP(req *http.Request) string {
	xff := req.Header.Get("X-Forwarded-For")
	if xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	ip, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		return req.RemoteAddr // Fallback to raw string if unable to split
	}

	return ip
}
