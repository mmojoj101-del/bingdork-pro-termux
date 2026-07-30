# Configuration Reference

BingDork Pro uses a YAML configuration file with full support for environment variable overrides. By default, it looks for configuration in:
- `./bingdork.yaml`
- `./config/bingdork.yaml`
- `$HOME/.bingdork/bingdork.yaml`
- `/etc/bingdork/bingdork.yaml`

## Quick Start

```bash
# Generate default configuration
bingdork config --init

# Or write to specific path
bingdork config --init /path/to/config.yaml

# Use custom config
bingdork --config /path/to/config.yaml search "query"
```

## Environment Variables

All configuration values can be overridden via environment variables prefixed with `BINGDORK_`:

```bash
export BINGDORK_LOGGING_LEVEL=debug
export BINGDORK_NETWORK_TIMEOUT=60s
export BINGDORK_PROVIDERS_BING_RATE_LIMIT=5
export BINGDORK_OUTPUT_FORMAT=csv
export BINGDORK_NETWORK_EVASION_ENABLED=false
```

## Full Configuration Reference

```yaml
# BingDork Pro Configuration

logging:
  level: info                    # debug, info, warn, error, fatal, disabled
  format: console                # console, json
  output: stdout                 # stdout, file
  file: ""                       # Path to log file (required if output: file)
  no_color: false                # Disable colored output

network:
  timeout: 30s                   # Request timeout
  retry_count: 3                 # Number of retry attempts
  retry_wait_min: 1s             # Minimum retry wait
  retry_wait_max: 10s            # Maximum retry wait
  proxy: ""                      # Proxy URL (http, https, socks5)
  proxy_type: ""                 # Proxy type (http, https, socks5)
  rate_limit: 10                 # Requests per second
  rate_burst: 5                  # Rate limit burst
  http2: true                    # Enable HTTP/2
  compress: true                 # Enable compression
  keep_alive: 30s                # Keep-alive duration

  # User-Agent rotation list
  user_agents:
    - "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
    - "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.2 Safari/605.1.15"

  # Custom HTTP headers
  custom_headers: {}

  # CAPTCHA bypass configuration
  captcha:
    enabled: false               # Enable CAPTCHA bypass
    service: ""                  # Solving service (2captcha, anticaptcha, etc.)
    api_key: ""                  # API key for solving service
    auto_solve: true             # Auto-solve detected CAPTCHAs
    session_reuse: true          # Reuse sessions to avoid CAPTCHAs
    image_recognition: false     # OCR-based image solving
    audio_recognition: false     # Speech-to-text audio solving
    timeout: 60s                 # Solving timeout
    retry_on_failure: true       # Retry on solving failure

  # Anti-bot evasion configuration
  evasion:
    enabled: true                # Enable evasion techniques
    fingerprint_randomize: true  # Randomize TLS/HTTP fingerprint
    header_spoofing: true        # Randomize request headers
    ip_rotation: false           # Enable IP rotation
    behavior_mimic: true         # Human-like behavior simulation
    tls_fingerprint: true        # Randomize TLS cipher suites
    referrer_spoofing: true      # Spoof referrer headers
    random_delay_min: 1s         # Minimum delay between requests
    random_delay_max: 3s         # Maximum delay between requests
    proxy_rotation: false        # Rotate through proxy list
    rotate_every: 10             # Requests before proxy rotation

cache:
  type: memory                   # memory, disk
  memory_size: 10000             # Max entries in memory cache
  disk_path: ~/.bingdork/cache   # Disk cache path
  disk_size: 100MB               # Max disk cache size
  ttl: 5m                        # Cache TTL
  cleanup_interval: 1m           # Cache cleanup interval
  enabled: true                  # Enable caching

storage:
  type: sqlite                   # sqlite, boltdb, json, jsonl
  path: ~/.bingdork/data/bingdork.db
  boltdb: ~/.bingdork/data/bingdork.bolt

output:
  format: json                   # json, csv, txt, md, yaml, jsonl
  pretty_print: true             # Pretty-print JSON/YAML
  raw_html: false                # Include raw HTML in output

scheduler:
  workers: 5                     # Worker pool size
  queue_size: 100                # Task queue size
  rate_limit: 10                 # Tasks per second
  retry_count: 3                 # Task retry count
  priority: true                 # Enable priority queue
  resume: true                   # Resume interrupted jobs
  state_file: ~/.bingdork/state.json
  job_timeout: 5m                # Per-job timeout

providers:
  default: bing                  # Default search provider

  bing:
    enabled: true
    base_url: https://www.bing.com/search
    rate_limit: 10
    timeout: 30s
    captcha: true                # Enable CAPTCHA detection/avoidance

  google:
    enabled: false
    base_url: https://www.google.com/search
    rate_limit: 5
    timeout: 30s

  duckduckgo:
    enabled: false
    base_url: https://html.duckduckgo.com/html
    rate_limit: 5
    timeout: 30s

  brave:
    enabled: false
    base_url: https://search.brave.com/search
    rate_limit: 10
    timeout: 30s

  yahoo:
    enabled: false
    base_url: https://search.yahoo.com/search
    rate_limit: 5
    timeout: 30s

  mojeek:
    enabled: false
    base_url: https://www.mojeek.com/search
    rate_limit: 5
    timeout: 30s

  searxng:
    enabled: false
    base_url: http://localhost:8888
    rate_limit: 30
    timeout: 30s

api:
  enabled: false
  type: rest                     # rest, grpc
  host: 127.0.0.1
  port: 8080
  grpc_port: 9090
  cors: true
  tls: false

tui:
  enabled: false
  theme: default
  log_level: info

metrics:
  enabled: false
  type: prometheus
  port: 2112
  path: /metrics

plugins:
  enabled: false
  dir: ~/.bingdork/plugins
  allow_all: false
  allow: []
  deny: []
```
