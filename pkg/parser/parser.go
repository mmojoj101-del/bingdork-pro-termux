// Package parser provides URL, domain, and HTML parsing utilities.
package parser

import (
	"net/url"
	"regexp"
	"strings"
	"sync"

	"mvdan.cc/xurls/v2"
)

var (
	// urlRegex matches URLs in text (relaxed)
	urlRegex = xurls.Strict()

	// domainRegex matches domain names
	domainRegex = regexp.MustCompile(`(?:[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}`)

	// ipRegex matches IP addresses
	ipRegex = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)

	// emailRegex matches email addresses
	emailRegex = regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)

	// wordToSaltPool for string building
	pool = sync.Pool{
		New: func() interface{} {
			return new(strings.Builder)
		},
	}
)

// ParseDomain extracts host and root domain from a URL string.
func ParseDomain(rawURL string) (host string, rootDomain string) {
	// Ensure scheme
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "https://" + rawURL
	}

	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return "", ""
	}

	host = u.Host
	// Strip port
	if idx := strings.LastIndex(host, ":"); idx > 0 {
		host = host[:idx]
	}

	rootDomain = extractRootDomain(host)
	return host, rootDomain
}

// extractRootDomain extracts the root domain (e.g., example.com from sub.example.com).
func extractRootDomain(host string) string {
	parts := strings.Split(host, ".")
	if len(parts) < 2 {
		return host
	}

	// Handle common multi-part TLDs
	tldOverrides := map[string]bool{
		"com": true, "org": true, "net": true, "gov": true, "edu": true,
		"co": true, "uk": true, "jp": true, "kr": true, "br": true,
		"au": true, "cn": true, "in": true, "de": true, "fr": true,
		"ru": true, "es": true, "it": true, "nl": true, "se": true,
		"no": true, "fi": true, "dk": true, "pl": true, "at": true,
		"ch": true, "be": true, "ie": true, "nz": true, "za": true,
		"mx": true, "ar": true, "cl": true, "pe": true,
		"us": true, "ca": true, "mil": true, "info": true, "biz": true,
		"tv": true, "me": true, "cc": true, "io": true, "pro": true,
		"name": true, "mobi": true, "asia": true, "tel": true,
	}

	// Check for two-part TLDs (e.g., co.uk, com.au)
	if len(parts) >= 3 {
		lastTwo := parts[len(parts)-2] + "." + parts[len(parts)-1]
		if _, ok := tldOverrides[parts[len(parts)-2]]; ok {
			if len(parts) >= 4 {
				return parts[len(parts)-3] + "." + lastTwo
			}
			return lastTwo
		}
	}

	// Standard: return last two parts
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "." + parts[len(parts)-1]
	}

	return host
}

// ExtractURLs extracts all URLs from a text body.
func ExtractURLs(body string) []string {
	return urlRegex.FindAllString(body, -1)
}

// ExtractDomains extracts all domain names from text.
func ExtractDomains(body string) []string {
	return domainRegex.FindAllString(body, -1)
}

// ExtractIPs extracts IP addresses from text.
func ExtractIPs(body string) []string {
	return ipRegex.FindAllString(body, -1)
}

// ExtractEmails extracts email addresses from text.
func ExtractEmails(body string) []string {
	return emailRegex.FindAllString(body, -1)
}

// NormalizeURL normalizes a URL by removing fragments and sorting query params.
func NormalizeURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	u.Fragment = ""
	u.RawFragment = ""

	// Re-encode query with sorted keys
	if u.RawQuery != "" {
		vals := u.Query()
		u.RawQuery = vals.Encode()
	}

	// Force scheme
	if u.Scheme == "" {
		u.Scheme = "https"
	}

	return u.String()
}

// CanonicalURL produces a canonical form for deduplication.
func CanonicalURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	u.Fragment = ""
	u.RawFragment = ""
	u.RawQuery = ""
	u.ForceQuery = false

	if u.Scheme == "" {
		u.Scheme = "https"
	}

	// Lowercase host
	u.Host = strings.ToLower(u.Host)

	// Remove trailing slash from path (except for root)
	if u.Path != "/" && strings.HasSuffix(u.Path, "/") {
		u.Path = strings.TrimSuffix(u.Path, "/")
	}

	return u.String()
}

// IsSubdomain checks if host is a subdomain of the given root domain.
func IsSubdomain(host, rootDomain string) bool {
	return strings.HasSuffix(strings.ToLower(host), "."+strings.ToLower(rootDomain))
}

// GetSubdomainParts returns the subdomain parts of a host.
func GetSubdomainParts(host string) []string {
	host = strings.ToLower(host)
	parts := strings.Split(host, ".")
	if len(parts) <= 2 {
		return nil
	}
	return parts[:len(parts)-2]
}

// IsValidURL checks if a string is a valid URL.
func IsValidURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	return err == nil && u.Scheme != "" && u.Host != ""
}

// URLMetadata holds parsed URL metadata.
type URLMetadata struct {
	Scheme     string `json:"scheme"`
	Host       string `json:"host"`
	RootDomain string `json:"root_domain"`
	Port       string `json:"port,omitempty"`
	Path       string `json:"path"`
	Query      string `json:"query"`
	Fragment   string `json:"fragment"`
	TLD        string `json:"tld"`
	IsIP       bool   `json:"is_ip"`
	IsSSL      bool   `json:"is_ssl"`
	PathDepth  int    `json:"path_depth"`
}

// ParseURL parses a URL into its metadata components.
func ParseURL(rawURL string) (*URLMetadata, error) {
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "https://" + rawURL
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	host := u.Host
	if idx := strings.LastIndex(host, ":"); idx > 0 {
		host = host[:idx]
	}

	_, rootDomain := ParseDomain(rawURL)

	// TLD
	parts := strings.Split(host, ".")
	tld := parts[len(parts)-1]

	// Is IP
	isIP := ipRegex.MatchString(host)

	// Path depth
	pathDepth := 0
	cleaned := strings.Trim(u.Path, "/")
	if cleaned != "" {
		pathDepth = len(strings.Split(cleaned, "/"))
	}

	return &URLMetadata{
		Scheme:     u.Scheme,
		Host:       host,
		RootDomain: rootDomain,
		Port:       u.Port(),
		Path:       u.Path,
		Query:      u.RawQuery,
		Fragment:   u.Fragment,
		TLD:        tld,
		IsIP:       isIP,
		IsSSL:      u.Scheme == "https",
		PathDepth:  pathDepth,
	}, nil
}

// StreamParser is a streaming HTML parser that extracts results incrementally.
type StreamParser struct {
	mu      sync.Mutex
	buffer  strings.Builder
	results chan *ParsedElement
	done    chan struct{}
	err     error
}

// ParsedElement represents a single extracted element.
type ParsedElement struct {
	Tag       string
	Class     string
	ID        string
	Text      string
	Href      string
	Src       string
	DataAttrs map[string]string
}

// NewStreamParser creates a new streaming parser.
func NewStreamParser() *StreamParser {
	return &StreamParser{
		results: make(chan *ParsedElement, 1000),
		done:    make(chan struct{}),
	}
}

// Feed feeds HTML data to the streaming parser.
func (sp *StreamParser) Feed(data string) {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	sp.buffer.WriteString(data)
}

// Results returns the results channel.
func (sp *StreamParser) Results() <-chan *ParsedElement {
	return sp.results
}

// Done signals that no more data will be fed.
func (sp *StreamParser) Done() {
	close(sp.done)
}

// Close closes the parser and cleans up.
func (sp *StreamParser) Close() {
	sp.Done()
	close(sp.results)
}

// ContainsGoogleAnalytics checks if a URL contains Google Analytics parameters.
func ContainsGoogleAnalytics(rawURL string) bool {
	params := []string{"utm_source", "utm_medium", "utm_campaign", "utm_term", "utm_content"}
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	q := u.Query()
	for _, p := range params {
		if q.Get(p) != "" {
			return true
		}
	}
	return false
}
