package traefik_geoblock

import (
	"bufio"
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/oschwald/maxminddb-golang"
)

// ---------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------

type Config struct {
	MmdbPath        string   `json:"mmdbPath"`
	ExcludeFilePath string   `json:"excludeFilePath"`
	AllowedCountries []string `json:"allowedCountries"` // ISO-3166-1 alpha-2, e.g. ["US","DE"]
	DenyCountries   []string `json:"denyCountries"`
	DefaultAllow    bool     `json:"defaultAllow"` // true = allowlist mode off = denylist
}

func CreateConfig() *Config {
	return &Config{
		DefaultAllow: true,
	}
}

// ---------------------------------------------------------------------------
// Plugin
// ---------------------------------------------------------------------------

type GeoBlock struct {
	next            http.Handler
	name            string
	cfg             *Config

	db              *maxminddb.Reader  // opened once, never closed during lifetime
	excludedNets    []*net.IPNet        // subnets that always pass through

	mu              sync.RWMutex        // guards excludedNets hot-reload
}

// mmdbRecord — only pull what we need; maxminddb does partial decode.
type mmdbRecord struct {
	Country struct {
		ISOCode string `maxminddb:"iso_code"`
	} `maxminddb:"country"`
}

func New(_ context.Context, next http.Handler, cfg *Config, name string) (http.Handler, error) {
	db, err := maxminddb.Open(cfg.MmdbPath)
	if err != nil {
		return nil, err
	}

	g := &GeoBlock{
		next: next,
		name: name,
		cfg:  cfg,
		db:   db,
	}

	if err := g.loadExcluded(); err != nil {
		log.Printf("[geoblock] warning: could not load exclude file: %v", err)
	}

	// Hot-reload the subnet exclusion file every 60 s without restarting Traefik.
	go g.watchExcludeFile(60 * time.Second)

	return g, nil
}

// ---------------------------------------------------------------------------
// ServeHTTP — kept as lean as possible
// ---------------------------------------------------------------------------

func (g *GeoBlock) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	ip := realIP(req)

	if ip == nil || g.isExcluded(ip) {
		g.next.ServeHTTP(rw, req)
		return
	}

	var record mmdbRecord
	// Lookup does a single binary-search in the memory-mapped file — ~1–3 µs.
	if err := g.db.Lookup(ip, &record); err != nil {
		// On error, fall through to default policy.
		if g.cfg.DefaultAllow {
			g.next.ServeHTTP(rw, req)
		} else {
			http.Error(rw, "Forbidden", http.StatusForbidden)
		}
		return
	}

	country := record.Country.ISOCode

	if g.isDenied(country) {
		http.Error(rw, "Forbidden", http.StatusForbidden)
		return
	}

	if !g.isAllowed(country) {
		http.Error(rw, "Forbidden", http.StatusForbidden)
		return
	}

	g.next.ServeHTTP(rw, req)
}

// ---------------------------------------------------------------------------
// Allow / deny logic
// ---------------------------------------------------------------------------

func (g *GeoBlock) isDenied(country string) bool {
	for _, c := range g.cfg.DenyCountries {
		if strings.EqualFold(c, country) {
			return true
		}
	}
	return false
}

func (g *GeoBlock) isAllowed(country string) bool {
	if len(g.cfg.AllowedCountries) == 0 {
		return g.cfg.DefaultAllow
	}
	for _, c := range g.cfg.AllowedCountries {
		if strings.EqualFold(c, country) {
			return true
		}
	}
	return false
}

func (g *GeoBlock) isExcluded(ip net.IP) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	for _, n := range g.excludedNets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Exclude-subnet file loader
// ---------------------------------------------------------------------------

// Format: one CIDR per line, # comments allowed, blank lines ignored.
//
//   # internal ranges
//   10.0.0.0/8
//   172.16.0.0/12
//   192.168.0.0/16

func (g *GeoBlock) loadExcluded() error {
	if g.cfg.ExcludeFilePath == "" {
		return nil
	}
	f, err := os.Open(g.cfg.ExcludeFilePath)
	if err != nil {
		return err
	}
	defer f.Close()

	var nets []*net.IPNet
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		_, ipnet, err := net.ParseCIDR(line)
		if err != nil {
			log.Printf("[geoblock] skipping invalid CIDR %q: %v", line, err)
			continue
		}
		nets = append(nets, ipnet)
	}

	g.mu.Lock()
	g.excludedNets = nets
	g.mu.Unlock()
	return scanner.Err()
}

func (g *GeoBlock) watchExcludeFile(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		if err := g.loadExcluded(); err != nil {
			log.Printf("[geoblock] hot-reload failed: %v", err)
		}
	}
}

// ---------------------------------------------------------------------------
// IP extraction helpers
// ---------------------------------------------------------------------------

func realIP(req *http.Request) net.IP {
	// Check common reverse-proxy headers first.
	for _, h := range []string{"X-Real-Ip", "X-Forwarded-For"} {
		raw := req.Header.Get(h)
		if raw == "" {
			continue
		}
		// X-Forwarded-For can be a comma-separated list; take the first (client).
		raw = strings.TrimSpace(strings.SplitN(raw, ",", 2)[0])
		if ip := net.ParseIP(raw); ip != nil {
			return ip
		}
	}
	// Fall back to RemoteAddr.
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		return net.ParseIP(req.RemoteAddr)
	}
	return net.ParseIP(host)
}
```

---

### Example `excluded_subnets.txt`
```
# RFC-1918 private ranges — always let through
10.0.0.0/8
172.16.0.0/12
192.168.0.0/16

# Loopback
127.0.0.0/8

# Kubernetes pod CIDR (example)
10.244.0.0/16
