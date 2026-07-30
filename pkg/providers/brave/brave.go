// Package brave implements the Brave Search provider.
package brave

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

// Provider implements the Brave Search engine provider.
type Provider struct {
	client       *network.Client
	log          *logger.Logger
	cfg          *core.ProviderConfig
	mu           sync.Mutex
	requestCount int
	lastReqTime  time.Time
	baseURL      string
}

// New creates a new Brave Search provider.
func New(cfg *core.ProviderConfig, netCfg *core.NetworkConfig, log *logger.Logger) (*Provider, error) {
	client, err := network.NewClient(netCfg, log)
	if err != nil {
		return nil, fmt.Errorf("creating brave client: %w", err)
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://search.brave.com/search"
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
	return core.ProviderBrave
}

// Search executes a search query against Brave Search.
func (p *Provider) Search(ctx context.Context, query *core.SearchQuery) (*core.ResultSet, error) {
	log := p.log.FromContext(ctx).With(logger.LogFields{
		"provider": "brave",
		"query":    query.Query,
		"page":     query.Page,
	})

	log.Debug("starting brave search")

	searchURL := p.buildSearchURL(query)
	log.Debug("request URL", logger.LogFields{"url": searchURL})

	resp, err := p.client.Get(ctx, searchURL)
	if err != nil {
		return nil, fmt.Errorf("brave request failed: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("brave returned status %d", resp.StatusCode())
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
		return nil, fmt.Errorf("parsing brave results: %w", err)
	}

	log.Info("brave search completed", logger.LogFields{
		"results": len(results),
	})

	return &core.ResultSet{
		Results:   results,
		Query:     query.Query,
		Provider:  core.ProviderBrave,
		Timestamp: time.Now(),
	}, nil
}

// NextPage fetches the next page of results.
func (p *Provider) NextPage(ctx context.Context, query *core.SearchQuery) (*core.ResultSet, error) {
	query.Page++
	return p.Search(ctx, query)
}

// Health checks if Brave Search is reachable.
func (p *Provider) Health(ctx context.Context) *core.ProviderHealth {
	start := time.Now()
	_, err := p.client.Get(ctx, "https://search.brave.com/")
	latency := time.Since(start)

	health := &core.ProviderHealth{
		Provider:  core.ProviderBrave,
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

// Capabilities returns what Brave Search supports.
func (p *Provider) Capabilities() *core.ProviderCapabilities {
	return &core.ProviderCapabilities{
		Pagination: true,
		SafeSearch: true,
		Language:   true,
		Region:     true,
		Advanced: []string{
			"site:", "intitle:", "inurl:", "filetype:",
			"ext:", "link:", "related:",
		},
		MaxPages:  50,
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
		"provider":     "brave",
		"base_url":     p.baseURL,
		"version":      "1.0",
		"capabilities": p.Capabilities(),
	}
}

// buildSearchURL constructs the Brave Search URL.
func (p *Provider) buildSearchURL(query *core.SearchQuery) string {
	params := url.Values{}
	params.Set("q", query.Query)

	// Page: Brave uses 'offset' parameter
	if query.Page > 0 {
		params.Set("offset", fmt.Sprintf("%d", query.Page*10))
	}

	// SafeSearch
	if p.cfg.Options["safe_search"] == "off" {
		params.Set("safesearch", "off")
	} else {
		params.Set("safesearch", "on")
	}

	// Language
	if lang, ok := query.Options["language"]; ok {
		params.Set("ui", lang)
	} else if p.cfg.Options["language"] != "" {
		params.Set("ui", p.cfg.Options["language"])
	}

	// Country/source
	if country, ok := query.Options["country"]; ok {
		params.Set("country", country)
	} else if p.cfg.Options["country"] != "" {
		params.Set("country", p.cfg.Options["country"])
	}

	// Pass through additional options
	for k, v := range query.Options {
		switch k {
		case "language", "country", "count", "raw_html", "safe_search":
			continue
		default:
			params.Set(k, v)
		}
	}

	return fmt.Sprintf("%s?%s", p.baseURL, params.Encode())
}

// parseResults extracts search results from Brave Search HTML response.
func (p *Provider) parseResults(body string, query *core.SearchQuery) ([]*core.Result, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parsing HTML: %w", err)
	}

	var results []*core.Result
	position := 0

	// Brave Search result selectors
	selectors := []string{
		"div.snippet",            // Main result container
		"div[data-type=web]",     // Web results
		"div.result-item",        // Result items
		"div.search-result",      // Search results
		"div.fdb",                // Result blocks
		"div.card",               // Card-based results
	}

	for _, selector := range selectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			result := p.extractResult(s, query, &position)
			if result != nil {
				results = append(results, result)
			}
		})
	}

	results = p.deduplicate(results)

	return results, nil
}

// extractResult extracts a single result from a Brave Search DOM element.
func (p *Provider) extractResult(s *goquery.Selection, query *core.SearchQuery, position *int) *core.Result {
	titleSelectors := []string{
		"a.title",               // Brave title link
		"span.title a",          // Title in span
		"a[href] span",          // Link with span
		"h2 a",                  // Heading link
		"a.result-header",       // Result header link
		"div.card-header a",     // Card header link
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

	// Fallback
	if href == "" && title == "" {
		s.Find("a[href]").Each(func(_ int, a *goquery.Selection) {
			if h, exists := a.Attr("href"); exists {
				h = strings.TrimSpace(h)
				if strings.HasPrefix(h, "http") && !strings.Contains(h, "search.brave.com") {
					href = h
					title = strings.TrimSpace(a.Text())
				}
			}
		})
	}

	if href == "" {
		return nil
	}

	// Skip ads and internal links
	if isAdLink(href) || isExcludedLink(href) {
		return nil
	}

	// Clean URL
	cleanURL := cleanBraveURL(href)
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
		Engine:      "brave",
	}
}

// extractDescription extracts the result description/snippet.
func (p *Provider) extractDescription(s *goquery.Selection) string {
	descSelectors := []string{
		"p.snippet-description",   // Brave description
		"div.snippet-description", // Description div
		"span.snippet-description", // Description span
		".description",            // Generic description
		"div.result-description",  // Result description
		"p.text",                  // Text paragraph
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

// cleanBraveURL cleans a Brave Search result URL.
func cleanBraveURL(rawURL string) string {
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
		"search.brave.com/search", "search.brave.com/settings",
		"search.brave.com/about", "search.brave.com/privacy",
		"search.brave.com/goggles",
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

// BypassRestrictions attempts to bypass Brave Search restrictions.
func (p *Provider) BypassRestrictions(ctx context.Context) error {
	p.log.Info("attempting to bypass Brave Search restrictions")

	_, err := p.client.Get(ctx, "https://search.brave.com/")
	if err != nil {
		return fmt.Errorf("homepage visit failed: %w", err)
	}

	delay := time.Duration(1+rand.Intn(3)) * time.Second
	p.log.Debug("bypass delay", logger.LogFields{"delay": delay.String()})
	time.Sleep(delay)

	return nil
}

// Close cleans up provider resources.
func (p *Provider) Close() {
	p.client.Close()
}
