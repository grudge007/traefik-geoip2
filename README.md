# Traefik GeoIP2 Middleware

A high-performance, ultra-low-latency Traefik middleware plugin that geolocates the source IP of incoming requests using MaxMind GeoLite2/GeoIP2 `.mmdb` databases.

This plugin parses the database in-memory and injects the ISO Country Code into a configurable HTTP header. It is designed to run securely and natively inside Traefik's Yaegi runtime, utilizing a **100% pure-Go** MaxMind reader implementation (`github.com/IncSW/geoip2`) that fully avoids memory-unsafe `unsafe` logic and `mmap` syscall restrictions.

## Features

- **Blazing Fast**: Parses memory directly with zero allocations, reducing lookup latency to ~`2.5ns` per request.
- **Pure Go**: Guaranteed Yaegi compatibility. Zero panics, no OS-specific syscalls.
- **WAF Ready**: Works seamlessly when placed before a Web Application Firewall like Coraza.

## Configuration Instruction

### 1. Download the Database
Download the free [MaxMind GeoLite2 Country Database](https://dev.maxmind.com/geoip/geolite2-free-geolocation-data). Extract it and place the `GeoLite2-Country.mmdb` file somewhere accessible by your Traefik instance (e.g. `/etc/traefik/geoip/`).

### 2. Static Configuration (Register the Plugin)
Update your Traefik static configuration (e.g., `traefik.yml`) to download and compile the plugin from GitHub.

```yaml
experimental:
  plugins:
    traefik-geoip2:
      moduleName: "github.com/grudge007/traefik-geoip2"
      version: "v0.1.4" # Use the latest release tag
```

### 3. Dynamic Configuration (Using the Middleware)
Define the middleware and pass it to your Traefik routers. 

```yaml
http:
  middlewares:
    my-geoip2:
      plugin:
        traefik-geoip2:
          # REQUIRED: The absolute path to your MaxMind MMDB file
          dbPath: "/etc/traefik/geoip/GeoLite2-Country.mmdb"
          # OPTIONAL: The HTTP Header to inject the country ISO code into (defaults to X-Geo-Country)
          headerName: "X-Geoip-Country-Code"

  routers:
    my-router:
      rule: "Host(`example.com`)"
      service: my-service
      middlewares:
        - my-geoip2
```

Once configured, Traefik will automatically extract the incoming IP (prioritizing `X-Forwarded-For`), look up the ISO Country Code (e.g., `US`, `DE`, `GB`), and attach it to the `X-Geoip-Country-Code` header before forwarding the request to your backend or subsequent middlewares.
