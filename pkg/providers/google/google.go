// Package google implements the Google search provider via HTML scraping.
package google

import (
	"context"
	"fmt"
	"math/rand"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/bingdork/bingdork/internal/core"
	"github.com/bingdork/bingdork/internal/logger"
	"github.com/bingdork/bingdork/internal/network"
	"github.com/bingdork/bingdork/pkg/parser"
)

// Provider implements the Google search engine provider via HTML scraping.
type Provider struct {
	client       *network.Client
	log          *logger.Logger
	cfg          *core.ProviderConfig
	mu           sync.Mutex
	requestCount int
	lastReqTime  time.Time
	baseURL      string
}

// New creates a new Google provider.
func New(cfg *core.ProviderConfig, netCfg *core.NetworkConfig, log *logger.Logger) (*Provider, error) {
	client, err := network.NewClient(netCfg, log)
	if err != nil {
		return nil, fmt.Errorf("creating google client: %w", err)
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://www.google.com/search"
	}

	return &Provider{
		client:  client,
		log:     log,
		cfg:     cfg,
		baseURL: baseURL,
	}, nil
}

// ID returns the provider identifier.
func (p *Provider) ID() core.ProviderID {
	return core.ProviderGoogle
}

// Search executes a search query against Google.
func (p *Provider) Search(ctx context.Context, query *core.SearchQuery) (*core.ResultSet, error) {
	log := p.log.FromContext(ctx).With(logger.LogFields{
		"provider": "google",
		"query":    query.Query,
		"page":     query.Page,
	})

	log.Debug("starting google search")

	searchURL := p.buildSearchURL(query)
	log.Debug("request URL", logger.LogFields{"url": searchURL})

	resp, err := p.client.Get(ctx, searchURL)
	if err != nil {
		return nil, fmt.Errorf("google request failed: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("google returned status %d", resp.StatusCode())
	}

	body := resp.String()

	if p.cfg.Options["raw_html"] == "true" {
		log.Trace("raw HTML response", logger.LogFields{
			"html": body,
			"size": len(body),
		})
	}

	results, err := p.parseResults(body, query)
	if err != nil {
		return nil, fmt.Errorf("parsing google results: %w", err)
	}

	log.Info("google search completed", logger.LogFields{
		"results": len(results),
	})

	return &core.ResultSet{
		Results:   results,
		Query:     query.Query,
		Provider:  core.ProviderGoogle,
		Timestamp: time.Now(),
	}, nil
}

// NextPage fetches the next page of results.
func (p *Provider) NextPage(ctx context.Context, query *core.SearchQuery) (*core.ResultSet, error) {
	query.Page++
	return p.Search(ctx, query)
}

// Health checks if Google is reachable.
func (p *Provider) Health(ctx context.Context) *core.ProviderHealth {
	start := time.Now()
	_, err := p.client.Get(ctx, "https://www.google.com/")
	latency := time.Since(start)

	health := &core.ProviderHealth{
		Provider:  core.ProviderGoogle,
		LastCheck: time.Now(),
		Latency:   latency,
	}

	if err != nil {
		health.Healthy = false
		health.Error = err.Error()
	} else {
		health.Healthy = true
	}

	return health
}

// Capabilities returns what Google supports.
func (p *Provider) Capabilities() *core.ProviderCapabilities {
	return &core.ProviderCapabilities{
		Pagination: true,
		SafeSearch: true,
		Language:   true,
		Region:     true,
		Advanced: []string{
			"site:", "intitle:", "inurl:", "filetype:",
			"ext:", "link:", "related:", "inanchor:",
			"cache:", "define:", "stocks:", "weather:",
			"map:", "movie:", "source:", "before:", "after:",
			"allintext:", "allintitle:", "allinurl:",
		},
		MaxPages:  100,
		RateLimit: int(p.cfg.RateLimit),
	}
}

// RateLimit returns current rate limit status.
func (p *Provider) RateLimit(ctx context.Context) *core.RateLimitInfo {
	p.mu.Lock()
	defer p.mu.Unlock()
	return &core.RateLimitInfo{
		RequestsPerMinute: int(p.cfg.RateLimit),
		Remaining:         int(p.cfg.RateLimit) - p.requestCount,
		ResetAt:           p.lastReqTime.Add(time.Minute),
	}
}

// Metadata returns provider-specific metadata.
func (p *Provider) Metadata(ctx context.Context) map[string]interface{} {
	return map[string]interface{}{
		"provider":     "google",
		"base_url":     p.baseURL,
		"version":      "1.0",
		"capabilities": p.Capabilities(),
	}
}

// buildSearchURL constructs the Google search URL with query parameters.
func (p *Provider) buildSearchURL(query *core.SearchQuery) string {
	params := url.Values{}
	params.Set("q", query.Query)
	params.Set("hl", "en")
	params.Set("gl", "US")

	// Page: Google uses 'start' parameter (0-based, increments by 10)
	if query.Page > 0 {
		params.Set("start", fmt.Sprintf("%d", query.Page*10))
	}

	// SafeSearch
	if p.cfg.Options["safe_search"] == "off" {
		params.Set("safe", "off")
	} else {
		params.Set("safe", "active")
	}

	// Language
	if lang, ok := query.Options["language"]; ok {
		params.Set("hl", lang)
	} else if p.cfg.Options["language"] != "" {
		params.Set("hl", p.cfg.Options["language"])
	}

	// Region
	if region, ok := query.Options["region"]; ok {
		params.Set("gl", region)
	} else if p.cfg.Options["region"] != "" {
		params.Set("gl", p.cfg.Options["region"])
	}

	// Count (Google uses 'num' parameter, max 100)
	if count, ok := query.Options["count"]; ok {
		params.Set("num", count)
	} else {
		params.Set("num", "10")
	}

	// Additional options (passthrough)
	for k, v := range query.Options {
		switch k {
		case "language", "region", "count", "raw_html", "safe_search":
			continue
		default:
			params.Set(k, v)
		}
	}

	return fmt.Sprintf("%s?%s", p.baseURL, params.Encode())
}

// parseResults extracts search results from Google HTML response.
func (p *Provider) parseResults(body string, query *core.SearchQuery) ([]*core.Result, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parsing HTML: %w", err)
	}

	var results []*core.Result
	position := 0

	// Google main result selectors (modern Google HTML)
	selectors := []string{
		"div.g",                        // Main search result container
		"div[data-sokoban-container]",  // Some result types
		"div.MjjYud",                   // New Google result class
	}

	for _, selector := range selectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			result := p.extractResult(s, query, &position)
			if result != nil {
				results = append(results, result)
			}
		})
	}

	// Deduplicate by URL
	results = p.deduplicate(results)

	return results, nil
}

// extractResult extracts a single result from a Google DOM element.
func (p *Provider) extractResult(s *goquery.Selection, query *core.SearchQuery, position *int) *core.Result {
	// Try different title/URL selectors for various Google layouts
	titleSelectors := []string{
		"h3 a",           // Standard result
		"h3",             // Sometimes title is plain h3
		"a[href] h3",     // Nested structure
		".LC20lb",        // Title class
		".DKV0Md",        // Alternative title class
		"a[ping]",        // Tracked links (modern Google)
	}

	var title string
	var href string

	for _, sel := range titleSelectors {
		link := s.Find(sel).First()
		if link.Length() > 0 {
			title = strings.TrimSpace(link.Text())
			// Try to get href from parent or self
			href, _ = link.Attr("href")
			if href == "" {
				// Try parent anchor
				link.ParentsFiltered("a").Each(func(_ int, a *goquery.Selection) {
					if h, exists := a.Attr("href"); exists {
						href = h
					}
				})
			}
			if href != "" {
				break
			}
		}
	}

	// If no title found, try direct link selector
	if href == "" {
		link := s.Find("a[href]").First()
		href, _ = link.Attr("href")
		title = strings.TrimSpace(link.Text())
	}

	if href == "" {
		return nil
	}

	// Skip ads and non-result links
	if isAdLink(href) || isExcludedLink(href) {
		return nil
	}

	// Clean URL
	cleanURL := cleanGoogleURL(href)
	if cleanURL == "" {
		return nil
	}

	// Extract description
	description := p.extractDescription(s)

	host, rootDomain := parser.ParseDomain(cleanURL)

	*position++

	return &core.Result{
		Title:       title,
		URL:         cleanURL,
		Host:        host,
		RootDomain:  rootDomain,
		Description: description,
		SearchPos:   *position,
		Page:        query.Page,
		Timestamp:   time.Now(),
		Engine:      "google",
	}
}

// extractDescription extracts the result description/snippet.
func (p *Provider) extractDescription(s *goquery.Selection) string {
	descSelectors := []string{
		"div.VwiC3b",          // Modern Google snippet
		"span.aCOpRe",         // Older snippet
		".st",                 // Classic snippet
		"div[data-sncf]",      // Knowledge panel description
		".lEBKkf",             // Alternative snippet
		"span[role=text]",     // Text role spans
	}

	for _, sel := range descSelectors {
		desc := s.Find(sel).First()
		if desc.Length() > 0 {
			text := strings.TrimSpace(desc.Text())
			if text != "" {
				return text
			}
		}
	}

	return ""
}

// deduplicate removes duplicate results by URL.
func (p *Provider) deduplicate(results []*core.Result) []*core.Result {
	seen := make(map[string]bool)
	var unique []*core.Result
	for _, r := range results {
		normalized := strings.ToLower(strings.TrimRight(r.URL, "/"))
		if !seen[normalized] {
			seen[normalized] = true
			unique = append(unique, r)
		}
	}
	return unique
}

// cleanGoogleURL extracts the actual URL from Google's redirect wrapper.
func cleanGoogleURL(rawURL string) string {
	// Google wraps URLs: /url?q=ACTUAL_URL&...
	if strings.HasPrefix(rawURL, "/url?") || strings.HasPrefix(rawURL, "/search?") {
		u, err := url.Parse(rawURL)
		if err != nil {
			return rawURL
		}
		if q := u.Query().Get("q"); q != "" {
			return q
		}
		// Check for other redirect params
		if urlStr := u.Query().Get("url"); urlStr != "" {
			return urlStr
		}
		// If it's a /search? URL with no q, it's internal
		if strings.HasPrefix(rawURL, "/search?") {
			return ""
		}
		return rawURL
	}

	// Ensure scheme
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "https://" + rawURL
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	if u.Host == "" {
		return ""
	}

	// Remove tracking fragments
	u.Fragment = ""
	u.RawQuery = stripTrackingParams(u.RawQuery)

	return u.String()
}

// stripTrackingParams removes tracking parameters from URLs.
func stripTrackingParams(rawQuery string) string {
	if rawQuery == "" {
		return ""
	}

	trackingParams := map[string]bool{
		"utm_source":   true,
		"utm_medium":   true,
		"utm_campaign": true,
		"utm_term":     true,
		"utm_content":  true,
		"fbclid":       true,
		"gclid":        true,
		"msclkid":      true,
	}

	params, _ := url.ParseQuery(rawQuery)
	clean := url.Values{}
	for k, v := range params {
		if !trackingParams[strings.ToLower(k)] {
			clean[k] = v
		}
	}
	return clean.Encode()
}

// isAdLink checks if a URL is an advertisement.
func isAdLink(href string) bool {
	adPatterns := []string{
		"//ad.", "//googleads", "doubleclick.net",
		"googleadservices.com", "googlesyndication.com",
		"pagead", "aclk?", "adurl=",
	}
	hrefLower := strings.ToLower(href)
	for _, pattern := range adPatterns {
		if strings.Contains(hrefLower, pattern) {
			return true
		}
	}
	return false
}

// isExcludedLink checks if a URL should be excluded.
func isExcludedLink(href string) bool {
	excluded := []string{
		"javascript:", "mailto:", "tel:", "#",
		"/search?", "/webhp?", "/images?", "/maps?",
		"accounts.google.com", "policies.google.com",
		"support.google.com", "consent.google.com",
	}
	hrefLower := strings.ToLower(href)
	for _, pattern := range excluded {
		if strings.HasPrefix(hrefLower, pattern) {
			return true
		}
	}
	return false
}

// SetClient allows setting a custom client (for testing).
func (p *Provider) SetClient(client *network.Client) {
	p.client = client
}

// GetClient returns the underlying network client.
func (p *Provider) GetClient() *network.Client {
	return p.client
}

// BypassRestrictions attempts to bypass Google restrictions/CAPTCHA.
func (p *Provider) BypassRestrictions(ctx context.Context) error {
	p.log.Info("attempting to bypass Google restrictions")

	// Visit homepage
	_, err := p.client.Get(ctx, "https://www.google.com/")
	if err != nil {
		return fmt.Errorf("homepage visit failed: %w", err)
	}

	// Benign search
	benignQuery := &core.SearchQuery{
		Query: "test",
		Page:  0,
	}
	_, err = p.Search(ctx, benignQuery)
	if err != nil {
		p.log.Warn("benign search failed, trying alternative bypass")
	}

	// Random delay
	delay := time.Duration(2+rand.Intn(4)) * time.Second
	p.log.Debug("bypass delay", logger.LogFields{"delay": delay.String()})
	time.Sleep(delay)

	return nil
}

// Close cleans up provider resources.
func (p *Provider) Close() {
	p.client.Close()
}
