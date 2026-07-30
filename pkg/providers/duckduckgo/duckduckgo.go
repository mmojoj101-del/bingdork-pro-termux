// Package duckduckgo implements the DuckDuckGo search provider.
package duckduckgo

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

// Provider implements the DuckDuckGo search engine provider.
type Provider struct {
	client       *network.Client
	log          *logger.Logger
	cfg          *core.ProviderConfig
	mu           sync.Mutex
	requestCount int
	lastReqTime  time.Time
	baseURL      string
	htmlURL      string
}

// New creates a new DuckDuckGo provider.
func New(cfg *core.ProviderConfig, netCfg *core.NetworkConfig, log *logger.Logger) (*Provider, error) {
	client, err := network.NewClient(netCfg, log)
	if err != nil {
		return nil, fmt.Errorf("creating duckduckgo client: %w", err)
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://duckduckgo.com/"
	}

	return &Provider{
		client:  client,
		log:     log,
		cfg:     cfg,
		baseURL: baseURL,
		htmlURL: "https://html.duckduckgo.com/html/",
	}, nil
}

// ID returns the provider identifier.
func (p *Provider) ID() core.ProviderID {
	return core.ProviderDuckDuckGo
}

// Search executes a search query against DuckDuckGo.
func (p *Provider) Search(ctx context.Context, query *core.SearchQuery) (*core.ResultSet, error) {
	log := p.log.FromContext(ctx).With(logger.LogFields{
		"provider": "duckduckgo",
		"query":    query.Query,
		"page":     query.Page,
	})

	log.Debug("starting duckduckgo search")

	searchURL := p.buildSearchURL(query)
	log.Debug("request URL", logger.LogFields{"url": searchURL})

	resp, err := p.client.Get(ctx, searchURL)
	if err != nil {
		return nil, fmt.Errorf("duckduckgo request failed: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("duckduckgo returned status %d", resp.StatusCode())
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
		return nil, fmt.Errorf("parsing duckduckgo results: %w", err)
	}

	log.Info("duckduckgo search completed", logger.LogFields{
		"results": len(results),
	})

	return &core.ResultSet{
		Results:   results,
		Query:     query.Query,
		Provider:  core.ProviderDuckDuckGo,
		Timestamp: time.Now(),
	}, nil
}

// NextPage fetches the next page of results.
func (p *Provider) NextPage(ctx context.Context, query *core.SearchQuery) (*core.ResultSet, error) {
	query.Page++
	return p.Search(ctx, query)
}

// Health checks if DuckDuckGo is reachable.
func (p *Provider) Health(ctx context.Context) *core.ProviderHealth {
	start := time.Now()
	_, err := p.client.Get(ctx, "https://duckduckgo.com/")
	latency := time.Since(start)

	health := &core.ProviderHealth{
		Provider:  core.ProviderDuckDuckGo,
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

// Capabilities returns what DuckDuckGo supports.
func (p *Provider) Capabilities() *core.ProviderCapabilities {
	return &core.ProviderCapabilities{
		Pagination: false, // DDG HTML doesn't support traditional pagination
		SafeSearch: true,
		Language:   true,
		Region:     true,
		Advanced: []string{
			"site:", "intitle:", "inurl:", "filetype:",
			"ext:", "link:", "related:",
		},
		MaxPages:  1,
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
		"provider":     "duckduckgo",
		"base_url":     p.baseURL,
		"html_url":     p.htmlURL,
		"version":      "1.0",
		"capabilities": p.Capabilities(),
	}
}

// buildSearchURL constructs the DuckDuckGo search URL.
// Uses the lite HTML version for reliable parsing.
func (p *Provider) buildSearchURL(query *core.SearchQuery) string {
	// Use the HTML (non-JS) version for reliable scraping
	base := p.htmlURL

	params := url.Values{}
	params.Set("q", query.Query)

	// Page handling
	if query.Page > 0 {
		params.Set("s", fmt.Sprintf("%d", query.Page*25))
	}

	// Language
	if lang, ok := query.Options["language"]; ok {
		params.Set("kl", lang)
	} else if p.cfg.Options["language"] != "" {
		params.Set("kl", p.cfg.Options["language"])
	} else {
		params.Set("kl", "us-en")
	}

	// SafeSearch
	if p.cfg.Options["safe_search"] == "off" {
		params.Set("kp", "-2")
	} else {
		params.Set("kp", "1")
	}

	// Region
	if region, ok := query.Options["region"]; ok {
		params.Set("k", region)
	} else if p.cfg.Options["region"] != "" {
		params.Set("k", p.cfg.Options["region"])
	}

	// Pass through additional options
	for k, v := range query.Options {
		switch k {
		case "language", "region", "count", "raw_html", "safe_search":
			continue
		default:
			params.Set(k, v)
		}
	}

	return fmt.Sprintf("%s?%s", base, params.Encode())
}

// parseResults extracts search results from DuckDuckGo HTML response.
func (p *Provider) parseResults(body string, query *core.SearchQuery) ([]*core.Result, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parsing HTML: %w", err)
	}

	var results []*core.Result
	position := 0

	// DuckDuckGo HTML version selectors
	selectors := []string{
		"div.result",            // Main result container (HTML version)
		"div.web-result",        // Alternative result container
		"article",               // Some DDG layouts use article
		"li.result",             // List-based results
		".results_links",        // Result links
		"div.links_main",        // Link containers
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

// extractResult extracts a single result from a DuckDuckGo DOM element.
func (p *Provider) extractResult(s *goquery.Selection, query *core.SearchQuery, position *int) *core.Result {
	titleSelectors := []string{
		"a.result__a",          // HTML version link
		"h2 a",                 // Standard heading link
		"a.result-link",        // Alternative link class
		"a[data-testid=result-title-a]", // New DDG layout
		"a.tracked",            // Tracked links
		"h2.result-title a",    // Title in result
	}

	var title string
	var href string

	for _, sel := range titleSelectors {
		link := s.Find(sel).First()
		if link.Length() > 0 {
			title = strings.TrimSpace(link.Text())
			href, _ = link.Attr("href")
			if href != "" {
				break
			}
		}
	}

	// Fallback: look for any link with proper URL
	if href == "" {
		s.Find("a[href]").Each(func(_ int, a *goquery.Selection) {
			if h, exists := a.Attr("href"); exists {
				if strings.HasPrefix(h, "http") || strings.HasPrefix(h, "//") {
					href = h
					title = strings.TrimSpace(a.Text())
				}
			}
		})
	}

	if href == "" {
		return nil
	}

	// Skip ads and non-result links
	if isAdLink(href) || isExcludedLink(href) {
		return nil
	}

	// Clean URL
	cleanURL := cleanDDGURL(href)
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
		Engine:      "duckduckgo",
	}
}

// extractDescription extracts the result description/snippet.
func (p *Provider) extractDescription(s *goquery.Selection) string {
	descSelectors := []string{
		"a.result__snippet",     // DDG HTML version snippet
		"span.result__snippet",  // Snippet in span
		".result__snippet",      // Generic snippet
		"div.snippet",           // Snippet div
		"p.result-description",  // Description paragraph
		".result__body",         // Result body
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

// cleanDDGURL extracts the actual URL from DuckDuckGo's redirect.
func cleanDDGURL(rawURL string) string {
	// DDG redirect: //duckduckgo.com/l/?uddg=ACTUAL_URL&...
	if strings.Contains(rawURL, "duckduckgo.com/l/") || strings.Contains(rawURL, "uddg=") {
		u, err := url.Parse(rawURL)
		if err != nil {
			return rawURL
		}
		if q := u.Query().Get("uddg"); q != "" {
			decoded, err := url.QueryUnescape(q)
			if err == nil {
				return decoded
			}
			return q
		}
		return rawURL
	}

	// Handle // prefix (protocol-relative)
	if strings.HasPrefix(rawURL, "//") {
		rawURL = "https:" + rawURL
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
		"//ad.", "//ads.", "doubleclick.net",
		"adservice", "pagead", "aclk?",
		"adurl=", "adword",
		"//duckduckgo.com/y.js", // DDG ad tracking
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
		"/html/", "duckduckgo.com/settings",
		"duckduckgo.com/about", "duckduckgo.com/privacy",
	}
	hrefLower := strings.ToLower(href)
	for _, pattern := range excluded {
		if strings.Contains(hrefLower, pattern) {
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

// BypassRestrictions attempts to bypass DuckDuckGo restrictions.
func (p *Provider) BypassRestrictions(ctx context.Context) error {
	p.log.Info("attempting to bypass DuckDuckGo restrictions")

	_, err := p.client.Get(ctx, "https://duckduckgo.com/")
	if err != nil {
		return fmt.Errorf("homepage visit failed: %w", err)
	}

	// Random delay
	delay := time.Duration(1+rand.Intn(3)) * time.Second
	p.log.Debug("bypass delay", logger.LogFields{"delay": delay.String()})
	time.Sleep(delay)

	return nil
}

// Close cleans up provider resources.
func (p *Provider) Close() {
	p.client.Close()
}
