// Package bing implements the Bing search provider.
package bing

import (
	"context"
	"fmt"
	"math/rand"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/bingdork/bingdork/internal/core"
	"github.com/bingdork/bingdork/internal/logger"
	"github.com/bingdork/bingdork/internal/network"
	"github.com/bingdork/bingdork/pkg/parser"
)

// Provider implements the Bing search engine provider.
type Provider struct {
	client       *network.Client
	log          *logger.Logger
	cfg          *core.ProviderConfig
	mu           sync.Mutex
	requestCount int
	lastReqTime  time.Time
	baseURL      string
}

// New creates a new Bing provider.
func New(cfg *core.ProviderConfig, netCfg *core.NetworkConfig, log *logger.Logger) (*Provider, error) {
	client, err := network.NewClient(netCfg, log)
	if err != nil {
		return nil, fmt.Errorf("creating bing client: %w", err)
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://www.bing.com/search"
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
	return core.ProviderBing
}

// Search executes a search query against Bing.
func (p *Provider) Search(ctx context.Context, query *core.SearchQuery) (*core.ResultSet, error) {
	log := p.log.FromContext(ctx).With(logger.LogFields{
		"provider": "bing",
		"query":    query.Query,
		"page":     query.Page,
	})

	log.Debug("starting bing search")

	// Build the search URL
	searchURL := p.buildSearchURL(query)
	log.Debug("request URL", logger.LogFields{"url": searchURL})

	// Execute the request
	resp, err := p.client.Get(ctx, searchURL)
	if err != nil {
		return nil, fmt.Errorf("bing request failed: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("bing returned status %d", resp.StatusCode())
	}

	body := resp.String()

	// Log raw HTML if configured
	if p.cfg.Options["raw_html"] == "true" {
		log.Trace("raw HTML response", logger.LogFields{
			"html": body,
			"size": len(body),
		})
	}

	// Parse results
	results, err := p.parseResults(body, query)
	if err != nil {
		return nil, fmt.Errorf("parsing bing results: %w", err)
	}

	log.Info("bing search completed", logger.LogFields{
		"results": len(results),
	})

	return &core.ResultSet{
		Results:   results,
		Query:     query.Query,
		Provider:  core.ProviderBing,
		Timestamp: time.Now(),
	}, nil
}

// NextPage fetches the next page of results.
func (p *Provider) NextPage(ctx context.Context, query *core.SearchQuery) (*core.ResultSet, error) {
	query.Page++
	return p.Search(ctx, query)
}

// Health checks if Bing is reachable.
func (p *Provider) Health(ctx context.Context) *core.ProviderHealth {
	start := time.Now()
	_, err := p.client.Get(ctx, "https://www.bing.com/")
	latency := time.Since(start)

	health := &core.ProviderHealth{
		Provider:  core.ProviderBing,
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

// Capabilities returns what Bing supports.
func (p *Provider) Capabilities() *core.ProviderCapabilities {
	return &core.ProviderCapabilities{
		Pagination: true,
		SafeSearch: true,
		Language:   true,
		Region:     true,
		Advanced: []string{
			"site:", "intitle:", "inurl:", "filetype:",
			"ext:", "link:", "related:", "inanchor:",
			"loc:", "ip:", "language:", "feed:",
			"hasfeed:", "sitecontains:", "contains:",
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
		"provider":       "bing",
		"base_url":       p.baseURL,
		"version":        "1.0",
		"capabilities":   p.Capabilities(),
	}
}

// buildSearchURL constructs the Bing search URL with query parameters.
func (p *Provider) buildSearchURL(query *core.SearchQuery) string {
	params := url.Values{}
	params.Set("q", query.Query)
	params.Set("setlang", "en-US")
	params.Set("cc", "US")

	// Page handling: Bing uses 'first' parameter (1-based, increments by 10)
	if query.Page > 0 {
		params.Set("first", fmt.Sprintf("%d", query.Page*10+1))
	}

	// SafeSearch
	if p.cfg.Options["safe_search"] == "off" {
		params.Set("adlt", "off")
	} else {
		params.Set("adlt", "strict")
	}

	// Language
	if lang, ok := query.Options["language"]; ok {
		params.Set("setlang", lang)
	} else if p.cfg.Options["language"] != "" {
		params.Set("setlang", p.cfg.Options["language"])
	}

	// Region
	if region, ok := query.Options["region"]; ok {
		params.Set("cc", region)
	} else if p.cfg.Options["region"] != "" {
		params.Set("cc", p.cfg.Options["region"])
	}

	// Count
	if count, ok := query.Options["count"]; ok {
		params.Set("count", count)
	} else {
		params.Set("count", "10")
	}

	// Market
	if market, ok := query.Options["mkt"]; ok {
		params.Set("mkt", market)
	}

	// Additional options
	for k, v := range query.Options {
		switch k {
		case "language", "region", "count", "mkt", "raw_html", "safe_search":
			continue
		default:
			params.Set(k, v)
		}
	}

	return fmt.Sprintf("%s?%s", p.baseURL, params.Encode())
}

// parseResults extracts search results from Bing HTML response.
func (p *Provider) parseResults(body string, query *core.SearchQuery) ([]*core.Result, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parsing HTML: %w", err)
	}

	var results []*core.Result
	position := 0

	// Bing main result selectors (modern + legacy)
	selectors := []string{
		"li.b_algo",              // Main results (original)
		"#b_results > li",        // All result list items
		"li.b_algoExp",           // Expanded results
		"ol#b_results > li",       // Ordered list results
		"div.b_algo",              // Div-based results
		"div.b_caption",           // Caption-based results
		"div.b_title",             // Title-based results
		"li.sb_add",               // Additional results
		"li.b_ans",                // Answer boxes
		"li.b_ans0",               // Top answer
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

// extractResult extracts a single result from a Bing DOM element.
func (p *Provider) extractResult(s *goquery.Selection, query *core.SearchQuery, position *int) *core.Result {
	// Try different title selectors (modern + legacy)
	titleSelectors := []string{
		"h2 a",                    // Standard
		"h2",                      // Plain h2
		"a[href]",                 // Any linked result
		".b_algoSlug a",           // Result slug
		"a.b_lt",                  // Large title
		"a[data-bm]",              // Data-bound links
		".b_title a",              // Title class
		".b_caption a[href]",      // Caption links
		"#b_results h2 a",         // Within results container
		".b_algo h2 a",            // Algorithm results
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

	if href == "" {
		return nil
	}

	// Skip ads and non-result links
	if isAdLink(href) || isExcludedLink(href) {
		return nil
	}

	// Clean URL
	cleanURL := cleanBingURL(href)
	if cleanURL == "" {
		return nil
	}

	// Extract description
	description := p.extractDescription(s)

	// Parse host and root domain
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
		Engine:      "bing",
	}
}

// extractDescription extracts the result description/snippet.
func (p *Provider) extractDescription(s *goquery.Selection) string {
	descSelectors := []string{
		"p.b_lineclamp2",           // Clamped description
		"p.b_foregroundtext",       // Foreground text
		"div.b_caption p",          // Caption paragraph
		"div.b_caption",            // Caption div
		".b_caption .b_snippet",    // Snippet
		".b_snippet",               // Generic snippet
		"p.b_snippet",              // Snippet paragraph
		".b_algoSlug",              // Result slug
		"span.b_more_text",         // More text
		".b_caption div",           // Caption div
		".b_secondaryText",         // Secondary text
		"div.b_newwrapper",         // New wrapper
		"p.b_newwrapper",           // New wrapper paragraph
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

// cleanBingURL extracts the actual URL from Bing's redirect wrapper.
func cleanBingURL(rawURL string) string {
	// Bing wraps URLs in redirect: /url?q=ACTUAL_URL&...
	if strings.HasPrefix(rawURL, "/url?") || strings.HasPrefix(rawURL, "/redirect?") {
		u, err := url.Parse(rawURL)
		if err != nil {
			return rawURL
		}
		if q := u.Query().Get("q"); q != "" {
			return q
		}
		if ru := u.Query().Get("ru"); ru != "" {
			return ru
		}
		if l := u.Query().Get("link"); l != "" {
			return l
		}
		return rawURL
	}

	// Ensure the URL has a scheme
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "https://" + rawURL
	}

	// Parse and validate
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
		"ref":          true,
		"spm":          true,
		"sc_channel":   true,
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
		"//ad.", "//www.bing.com/aclick", "//adservice",
		"//ads.", "/aclk?", "doubleclick.net",
		"googleads", "adurl=", "adword",
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
	}
	hrefLower := strings.ToLower(href)
	for _, pattern := range excluded {
		if strings.HasPrefix(hrefLower, pattern) {
			return true
		}
	}
	return false
}

// Bing specific result count regex
var resultCountRegex = regexp.MustCompile(`([0-9,]+)\s+results`)

// ParseResultCount extracts the total result count from Bing's result stats.
func ParseResultCount(body string) int {
	matches := resultCountRegex.FindStringSubmatch(body)
	if len(matches) > 1 {
		countStr := strings.ReplaceAll(matches[1], ",", "")
		var count int
		fmt.Sscanf(countStr, "%d", &count)
		return count
	}
	return 0
}

// BypassRestrictions attempts to bypass Bing's CAPTCHA or restrictions.
func (p *Provider) BypassRestrictions(ctx context.Context) error {
	p.log.Info("attempting to bypass Bing restrictions")

	// Method 1: Visit homepage first to establish session
	resp, err := p.client.Get(ctx, "https://www.bing.com/")
	if err != nil {
		return fmt.Errorf("homepage visit failed: %w", err)
	}

	// Method 2: Search with a benign query first
	benignQuery := &core.SearchQuery{
		Query: "test",
		Page:  0,
	}
	_, err = p.Search(ctx, benignQuery)
	if err != nil {
		p.log.Warn("benign search failed, trying alternative bypass")
	}

	// Method 3: Random delay between 3-7 seconds
	delay := time.Duration(3+rand.Intn(5)) * time.Second
	p.log.Debug("bypass delay", logger.LogFields{"delay": delay.String()})
	time.Sleep(delay)

	// Method 4: Change user agent
	_ = resp // session established

	return nil
}

// GetClient returns the underlying network client.
func (p *Provider) GetClient() *network.Client {
	return p.client
}

// Close cleans up provider resources.
func (p *Provider) Close() {
	p.client.Close()
}
