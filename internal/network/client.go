// Package network provides HTTP client with CAPTCHA bypass and anti-bot evasion.
package network

import (
	"context"
	"crypto/tls"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/bingdork/bingdork/pkg/useragent"
	"github.com/go-resty/resty/v2"
	"golang.org/x/net/http2"
	"golang.org/x/net/proxy"

	"github.com/bingdork/bingdork/internal/core"
	"github.com/bingdork/bingdork/internal/logger"
)

// Client wraps resty with evasion and CAPTCHA capabilities.
type Client struct {
	client     *resty.Client
	cfg        *core.NetworkConfig
	log        *logger.Logger
	mu         sync.Mutex
	reqCount   int
	startTime  time.Time
	rateLimiter *time.Ticker
	proxyIndex int
	userAgentIndex int
}

// NewClient creates a new HTTP client with the given configuration.
func NewClient(cfg *core.NetworkConfig, log *logger.Logger) (*Client, error) {
	restyClient := resty.New()

	// Timeout
	restyClient.SetTimeout(cfg.Timeout)

	// Transport configuration
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false,
			MinVersion:         tls.VersionTLS12,
			CipherSuites:       getCipherSuites(cfg.Evasion.TLSFingerprint),
		},
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     cfg.KeepAlive,
		DisableCompression:  !cfg.Compress,
		ForceAttemptHTTP2:   cfg.HTTP2,
	}

	restyClient.SetTransport(transport)

	// Cookie jar for session reuse
	jar, _ := cookiejar.New(nil)
	restyClient.SetCookieJar(jar)

	// Default headers
	restyClient.SetHeaders(map[string]string{
		"Accept":             "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
		"Accept-Language":    "en-US,en;q=0.9",
		"Accept-Encoding":    "gzip, deflate, br",
		"Connection":         "keep-alive",
		"Upgrade-Insecure-Requests": "1",
		"Sec-Fetch-Dest":     "document",
		"Sec-Fetch-Mode":     "navigate",
		"Sec-Fetch-Site":     "none",
		"Sec-Fetch-User":     "?1",
	})

	// Custom headers
	for k, v := range cfg.CustomHeaders {
		restyClient.SetHeader(k, v)
	}

	// Proxy
	if cfg.Proxy != "" {
		restyClient.SetProxy(cfg.Proxy)
	}

	// HTTP/2
	if cfg.HTTP2 {
		if err := http2.ConfigureTransport(transport); err != nil {
			log.Warn("failed to configure HTTP/2", logger.LogFields{"error": err})
		}
	}

	// Redirect policy
	restyClient.SetRedirectPolicy(resty.FlexibleRedirectPolicy(15))

	// Retry
	if cfg.RetryCount > 0 {
		restyClient.
			SetRetryCount(cfg.RetryCount).
			SetRetryWaitTime(cfg.RetryWaitMin).
			SetRetryMaxWaitTime(cfg.RetryWaitMax).
			AddRetryCondition(func(r *resty.Response, err error) bool {
				if err != nil {
					return true
				}
				// Retry on rate limits and server errors
				code := r.StatusCode()
				if code == http.StatusTooManyRequests || code == http.StatusServiceUnavailable ||
					code >= 500 || (code == http.StatusForbidden && cfg.CAPTCHA.AutoSolve) {
					return true
				}
				// Check Retry-After header
				if r.Header().Get("Retry-After") != "" {
					return true
				}
				return false
			})
	}

	rateLimit := cfg.RateLimit
	if rateLimit <= 0 {
		rateLimit = 10 // default
	}
	c := &Client{
		client:        restyClient,
		cfg:           cfg,
		log:           log,
		rateLimiter:   time.NewTicker(time.Second / time.Duration(rateLimit)),
		startTime:     time.Now(),
	}

	// Configure evasion if enabled
	if cfg.Evasion.Enabled {
		c.configureEvasion()
	}

	return c, nil
}

// configureEvasion sets up anti-bot evasion techniques.
func (c *Client) configureEvasion() {
	// Register middleware for evasion
	c.client.OnBeforeRequest(func(rc *resty.Client, req *resty.Request) error {
		c.mu.Lock()
		defer c.mu.Unlock()

		// Rate limiting
		if c.cfg.RateLimit > 0 {
			c.rateLimiter = time.NewTicker(time.Second / time.Duration(c.cfg.RateLimit))
		}

		// User-Agent rotation
		if c.cfg.Evasion.FingerprintRandomize {
			ua := c.getRandomUserAgent()
			req.SetHeader("User-Agent", ua)
		}

		// Header spoofing
		if c.cfg.Evasion.HeaderSpoofing {
			c.spoofHeaders(req)
		}

		// Referrer spoofing
		if c.cfg.Evasion.ReferrerSpoofing {
			c.spoofReferrer(req)
		}

		// Random delay for behavioral mimicking
		if c.cfg.Evasion.BehaviorMimic {
			c.randomDelay()
		}

		// IP rotation
		if c.cfg.Evasion.IPRotation && c.cfg.Evasion.ProxyRotation {
			c.rotateProxy(req)
		}

		// TLS fingerprint randomization
		if c.cfg.Evasion.TLSFingerprint {
			c.randomizeTLSParams()
		}

		c.reqCount++
		return nil
	})

	// Post-request handling
	c.client.OnAfterResponse(func(rc *resty.Client, resp *resty.Response) error {
		// Check for CAPTCHA challenge
		if c.cfg.CAPTCHA.Enabled && c.isCAPTCHAChallenge(resp) {
			c.log.Warn("CAPTCHA challenge detected", logger.LogFields{
				"url":    resp.Request.URL,
				"status": resp.StatusCode(),
			})
			if c.cfg.CAPTCHA.AutoSolve {
				return c.handleCAPTCHA(resp)
			}
		}
		return nil
	})
}

// spoofHeaders randomizes request headers to avoid fingerprinting.
func (c *Client) spoofHeaders(req *resty.Request) {
	// Randomize Accept header variants
	accepts := []string{
		"text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
		"text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		"text/html,application/xhtml+xml;q=0.9,application/xml;q=0.8,*/*;q=0.7",
	}
	req.SetHeader("Accept", accepts[rand.Intn(len(accepts))])

	// Randomize Accept-Language
	languages := []string{
		"en-US,en;q=0.9",
		"en-US,en;q=0.9,es;q=0.8",
		"en-GB,en;q=0.9,en-US;q=0.8",
		"en;q=0.9,fr;q=0.8",
	}
	req.SetHeader("Accept-Language", languages[rand.Intn(len(languages))])

	// Randomize Sec-CH-UA
	brands := []string{
		`"Chromium";v="131", "Google Chrome";v="131", "Not?A_Brand";v="24"`,
		`"Chromium";v="131", "Not A(Brand";v="99"`,
		`"Firefox";v="133", "Gecko";v="133"`,
	}
	req.SetHeader("Sec-CH-UA", brands[rand.Intn(len(brands))])
	req.SetHeader("Sec-CH-UA-Mobile", "?0")

	platforms := []string{"Windows", "macOS", "Linux"}
	req.SetHeader("Sec-CH-UA-Platform", fmt.Sprintf(`"%s"`, platforms[rand.Intn(len(platforms))]))

	// Sec-Fetch headers
	fetchDest := []string{"document", "empty", "iframe"}
	fetchMode := []string{"navigate", "no-cors", "cors"}
	fetchSite := []string{"none", "same-origin", "cross-site"}

	req.SetHeader("Sec-Fetch-Dest", fetchDest[rand.Intn(len(fetchDest))])
	req.SetHeader("Sec-Fetch-Mode", fetchMode[rand.Intn(len(fetchMode))])
	req.SetHeader("Sec-Fetch-Site", fetchSite[rand.Intn(len(fetchSite))])
}

// spoofReferrer sets a realistic referrer header.
func (c *Client) spoofReferrer(req *resty.Request) {
	referrers := []string{
		"https://www.google.com/",
		"https://duckduckgo.com/",
		"https://www.bing.com/",
		"https://www.yahoo.com/",
		"https://www.google.com/search?q=test",
		"https://www.bing.com/search?q=search",
		"",
	}
	ref := referrers[rand.Intn(len(referrers))]
	if ref != "" {
		req.SetHeader("Referer", ref)
	}
}

// randomDelay introduces a human-like random delay.
func (c *Client) randomDelay() {
	min := c.cfg.Evasion.RandomDelayMin
	max := c.cfg.Evasion.RandomDelayMax
	if min <= 0 {
		min = 500 * time.Millisecond
	}
	if max <= 0 {
		max = 2 * time.Second
	}
	delay := min + time.Duration(rand.Int63n(int64(max-min)))
	time.Sleep(delay)
}

// getRandomUserAgent returns a random user agent.
func (c *Client) getRandomUserAgent() string {
	if len(c.cfg.UserAgents) > 0 {
		return c.cfg.UserAgents[rand.Intn(len(c.cfg.UserAgents))]
	}
	return useragent.Get()
}

func (c *Client) rotateProxy(req *resty.Request) {
	if len(c.cfg.Evasion.ProxyList) == 0 {
		return
	}
	c.mu.Lock()
	idx := c.proxyIndex % len(c.cfg.Evasion.ProxyList)
	c.proxyIndex++
	c.mu.Unlock()
	c.client.SetProxy(c.cfg.Evasion.ProxyList[idx])
}

// randomizeTLSParams randomizes TLS fingerprint parameters.
func (c *Client) randomizeTLSParams() {
	// This is a simplified TLS randomization.
	// For production, use a library like utls for full fingerprint randomization.
	if transport, ok := c.client.GetClient().Transport.(*http.Transport); ok {
		// Randomize cipher suites
		allCiphers := []uint16{
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_CHACHA20_POLY1305_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		}
		n := rand.Intn(len(allCiphers)) + 5 // at least 5
		if n > len(allCiphers) {
			n = len(allCiphers)
		}
		selected := make([]uint16, n)
		perm := rand.Perm(len(allCiphers))
		for i := 0; i < n; i++ {
			selected[i] = allCiphers[perm[i]]
		}
		transport.TLSClientConfig.CipherSuites = selected
	}
}

// isCAPTCHAChallenge detects if a response contains a CAPTCHA challenge.
func (c *Client) isCAPTCHAChallenge(resp *resty.Response) bool {
	body := string(resp.Body())

	// Common CAPTCHA indicators
	indicators := []string{
		"captcha",
		"recaptcha",
		"hcaptcha",
		"g-recaptcha",
		"cf-turnstile",
		"data-sitekey",
		"_cf_chl_opt",
		"challenge-platform",
		"CAPTCHA",
		"are you a robot",
		"verify you're human",
		"security check",
		"unusual traffic",
	}

	bodyLower := strings.ToLower(body)
	for _, indicator := range indicators {
		if strings.Contains(bodyLower, indicator) {
			return true
		}
	}

	// Check status codes that often accompany CAPTCHAs
	if resp.StatusCode() == http.StatusForbidden ||
		resp.StatusCode() == http.StatusTooManyRequests ||
		resp.StatusCode() == http.StatusServiceUnavailable {
		return true
	}

	return false
}

// handleCAPTCHA attempts to solve a CAPTCHA challenge.
func (c *Client) handleCAPTCHA(resp *resty.Response) error {
	c.log.Info("attempting CAPTCHA bypass")

	// Strategy 1: Session reuse - use existing cookies
	if c.cfg.CAPTCHA.SessionReuse {
		c.log.Debug("attempting session reuse for CAPTCHA bypass")
		// The cookie jar already maintains session; retry with existing cookies
		return nil
	}

	// Strategy 2: Use third-party CAPTCHA solving service
	if c.cfg.CAPTCHA.APIKey != "" && c.cfg.CAPTCHA.Service != "" {
		return c.solveWithService(resp)
	}

	// Strategy 3: Image recognition
	if c.cfg.CAPTCHA.ImageRecognition {
		return c.solveImageCAPTCHA(resp)
	}

	// Strategy 4: Audio recognition
	if c.cfg.CAPTCHA.AudioRecognition {
		return c.solveAudioCAPTCHA(resp)
	}

	c.log.Warn("no CAPTCHA bypass method available, returning error")
	return fmt.Errorf("CAPTCHA challenge detected and no bypass method succeeded")
}

// solveWithService uses a third-party CAPTCHA solving service.
func (c *Client) solveWithService(resp *resty.Response) error {
	// Placeholder for third-party service integration.
	// Supported services: 2Captcha, Anti-Captcha, DeathByCaptcha, etc.
	c.log.Info("sending CAPTCHA to solving service", logger.LogFields{
		"service": c.cfg.CAPTCHA.Service,
	})
	// In production, this would:
	// 1. Extract site key from response body
	// 2. Submit to solving service API
	// 3. Poll for solution
	// 4. Inject solution token into the request
	// 5. Retry the original request with the solved token
	return fmt.Errorf("CAPTCHA solving service not fully implemented: %s", c.cfg.CAPTCHA.Service)
}

// solveImageCAPTCHA attempts to solve image-based CAPTCHAs using OCR.
func (c *Client) solveImageCAPTCHA(resp *resty.Response) error {
	// Placeholder for image-based CAPTCHA solving.
	// In production, this would use Tesseract, Google Vision, or custom CNN.
	c.log.Info("attempting image CAPTCHA recognition")
	return fmt.Errorf("image CAPTCHA recognition not fully implemented")
}

// solveAudioCAPTCHA attempts to solve audio-based CAPTCHAs.
func (c *Client) solveAudioCAPTCHA(resp *resty.Response) error {
	// Placeholder for audio-based CAPTCHA solving.
	// In production, this would use speech-to-text APIs.
	c.log.Info("attempting audio CAPTCHA recognition")
	return fmt.Errorf("audio CAPTCHA recognition not fully implemented")
}

// Get performs a GET request with full evasion support.
func (c *Client) Get(ctx context.Context, url string) (*resty.Response, error) {
	c.mu.Lock()
	if c.cfg.RateLimit > 0 && c.rateLimiter != nil {
		<-c.rateLimiter.C
	}
	c.mu.Unlock()

	req := c.client.R().
		SetContext(ctx).
		SetHeader("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")

	return req.Get(url)
}

// Post performs a POST request with full evasion support.
func (c *Client) Post(ctx context.Context, url string, data map[string]string) (*resty.Response, error) {
	c.mu.Lock()
	if c.cfg.RateLimit > 0 && c.rateLimiter != nil {
		<-c.rateLimiter.C
	}
	c.mu.Unlock()

	req := c.client.R().
		SetContext(ctx).
		SetFormData(data)

	return req.Post(url)
}

// Do performs a raw request with the given method, URL, and options.
func (c *Client) Do(ctx context.Context, method, url string, opts ...func(*resty.Request)) (*resty.Response, error) {
	c.mu.Lock()
	if c.cfg.RateLimit > 0 && c.rateLimiter != nil {
		<-c.rateLimiter.C
	}
	c.mu.Unlock()

	req := c.client.R().SetContext(ctx)
	for _, opt := range opts {
		opt(req)
	}

	return req.Execute(method, url)
}

// Transport allows access to the underlying http.Transport for customization.
func (c *Client) Transport() *http.Transport {
	if tr, ok := c.client.GetClient().Transport.(*http.Transport); ok {
		return tr
	}
	return nil
}

// Resty returns the underlying resty client for advanced usage.
func (c *Client) Resty() *resty.Client {
	return c.client
}

// ResetJar clears the cookie jar.
func (c *Client) ResetJar() {
	jar, _ := cookiejar.New(nil)
	c.client.SetCookieJar(jar)
}

// Stats returns request statistics.
func (c *Client) Stats() map[string]interface{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	return map[string]interface{}{
		"requests":     c.reqCount,
		"uptime":       time.Since(c.startTime).String(),
		"rate":         float64(c.reqCount) / time.Since(c.startTime).Seconds(),
	}
}

// Close cleans up the client resources.
func (c *Client) Close() {
	c.rateLimiter.Stop()
	c.client.GetClient().CloseIdleConnections()
}

// getCipherSuites returns a list of cipher suites, optionally randomized.
func getCipherSuites(randomize bool) []uint16 {
	suites := []uint16{
		tls.TLS_AES_128_GCM_SHA256,
		tls.TLS_AES_256_GCM_SHA384,
		tls.TLS_CHACHA20_POLY1305_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
		tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
		tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
		tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
	}
	if randomize {
		rand.Shuffle(len(suites), func(i, j int) {
			suites[i], suites[j] = suites[j], suites[i]
		})
	}
	return suites
}

// init registers the proxy dialer.
func init() {
	rand.Seed(time.Now().UnixNano())
}

// SOCKS5Dialer creates a SOCKS5 proxy dialer.
func SOCKS5Dialer(proxyURL string) (proxy.Dialer, error) {
	u, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("parsing proxy URL: %w", err)
	}
	return proxy.FromURL(u, proxy.Direct)
}

// RotatingProxyManager manages a pool of proxies for rotation.
type RotatingProxyManager struct {
	proxies []string
	index   int
	mu      sync.Mutex
}

// NewRotatingProxyManager creates a new proxy rotation manager.
func NewRotatingProxyManager(proxies []string) *RotatingProxyManager {
	return &RotatingProxyManager{
		proxies: proxies,
	}
}

// Next returns the next proxy URL in rotation.
func (m *RotatingProxyManager) Next() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.proxies) == 0 {
		return ""
	}
	proxy := m.proxies[m.index]
	m.index = (m.index + 1) % len(m.proxies)
	return proxy
}

// Add adds proxies to the pool.
func (m *RotatingProxyManager) Add(proxies ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.proxies = append(m.proxies, proxies...)
}

// Size returns the number of proxies in the pool.
func (m *RotatingProxyManager) Size() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.proxies)
}

// ProxyDialerWrapper wraps a proxy with direct dialer fallback.
type ProxyDialerWrapper struct {
	Dialer proxy.Dialer
	Direct net.Dialer
}

// Dial connects to the given address via the proxy or direct.
func (w *ProxyDialerWrapper) Dial(network, addr string) (net.Conn, error) {
	if w.Dialer != nil {
		return w.Dialer.Dial(network, addr)
	}
	return w.Direct.Dial(network, addr)
}
