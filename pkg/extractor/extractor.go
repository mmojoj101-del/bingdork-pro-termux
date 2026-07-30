// Package extractor provides advanced data extraction from search results.
package extractor

import (
	"context"
	"regexp"
	"strings"
	"sync"

	"github.com/bingdork/bingdork/internal/core"
	"github.com/bingdork/bingdork/internal/logger"
	"github.com/bingdork/bingdork/pkg/parser"
)

// Extractor processes result sets to extract enriched data.
type Extractor struct {
	log      *logger.Logger
	filters  []core.Filter
	mu       sync.RWMutex
	extractors []ExtractorFunc
}

// ExtractorFunc is a function that extracts data from results.
type ExtractorFunc func(ctx context.Context, result *core.Result) (map[string]interface{}, error)

// New creates a new Extractor.
func New(log *logger.Logger) *Extractor {
	e := &Extractor{
		log:     log,
		filters: make([]core.Filter, 0),
	}
	e.registerDefaultExtractors()
	return e
}

// WithFilters sets the filters for the extractor.
func (e *Extractor) WithFilters(filters []core.Filter) *Extractor {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.filters = filters
	return e
}

// AddFilter adds a single filter.
func (e *Extractor) AddFilter(filter core.Filter) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.filters = append(e.filters, filter)
}

// RegisterExtractor adds a custom extraction function.
func (e *Extractor) RegisterExtractor(fn ExtractorFunc) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.extractors = append(e.extractors, fn)
}

// Extract processes a result set and returns processed results.
func (e *Extractor) Extract(ctx context.Context, resultSet *core.ResultSet) (*core.ResultSet, error) {
	e.mu.RLock()
	filters := make([]core.Filter, len(e.filters))
	copy(filters, e.filters)
	extractors := make([]ExtractorFunc, len(e.extractors))
	copy(extractors, e.extractors)
	e.mu.RUnlock()

	var processed []*core.Result
	for _, result := range resultSet.Results {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Apply filters
		if !e.applyFilters(result, filters) {
			continue
		}

		// Run extractors
		for _, fn := range extractors {
			data, err := fn(ctx, result)
			if err != nil {
				e.log.Warn("extractor failed", logger.LogFields{
					"url":   result.URL,
					"error": err,
				})
				continue
			}
			// In production, this data would be merged into result metadata
			_ = data
		}

		// Enrich with parsed URL data
		e.enrichResult(result)
		processed = append(processed, result)
	}

	resultSet.Results = processed
	resultSet.Total = len(processed)

	return resultSet, nil
}

// applyFilters checks if a result passes all filters.
func (e *Extractor) applyFilters(result *core.Result, filters []core.Filter) bool {
	for _, f := range filters {
		pass := e.applyFilter(result, f)
		if !pass {
			return false
		}
	}
	return true
}

// applyFilter checks a single filter against a result.
func (e *Extractor) applyFilter(result *core.Result, filter core.Filter) bool {
	switch filter.Type {
	case core.FilterRegexInclude:
		matched, _ := regexp.MatchString(filter.Pattern, result.URL)
		if filter.Negate {
			return !matched // include what does NOT match
		}
		return matched // include what matches

	case core.FilterRegexExclude:
		matched, _ := regexp.MatchString(filter.Pattern, result.URL)
		if filter.Negate {
			return matched // exclude what does NOT match (keep what matches)
		}
		return !matched // exclude what matches (keep what doesn't)

	case core.FilterHostWhitelist:
		matched := strings.EqualFold(result.Host, filter.Pattern)
		if filter.Negate {
			return !matched
		}
		return matched

	case core.FilterHostBlacklist:
		matched := strings.EqualFold(result.Host, filter.Pattern)
		if filter.Negate {
			return matched
		}
		return !matched

	case core.FilterExtension:
		ext := getExtension(result.URL)
		matched := strings.EqualFold(ext, filter.Pattern)
		if filter.Negate {
			return !matched // exclude what matches extension
		}
		return matched // only keep what matches extension

	case core.FilterKeyword:
		text := strings.ToLower(result.Title + " " + result.Description)
		keyword := strings.ToLower(filter.Pattern)
		contains := strings.Contains(text, keyword)
		if filter.Negate {
			return !contains // exclude what contains keyword
		}
		return contains // keep what contains keyword

	case core.FilterDuplicate:
		return true
	default:
		return true
	}
}

// enrichResult adds parsed URL metadata to the result.
func (e *Extractor) enrichResult(result *core.Result) {
	meta, err := parser.ParseURL(result.URL)
	if err != nil {
		return
	}
	_ = meta // In production, merge into result
}

// registerDefaultExtractors registers the built-in extraction functions.
func (e *Extractor) registerDefaultExtractors() {
	e.extractors = append(e.extractors,
		e.extractEmails,
		e.extractIPs,
		e.extractTechStacks,
		e.extractForms,
	)
}

// extractEmails extracts email addresses from result content.
func (e *Extractor) extractEmails(ctx context.Context, result *core.Result) (map[string]interface{}, error) {
	emails := parser.ExtractEmails(result.Description)
	if len(emails) > 0 {
		return map[string]interface{}{"emails": emails}, nil
	}
	return nil, nil
}

// extractIPs extracts IP addresses from result content.
func (e *Extractor) extractIPs(ctx context.Context, result *core.Result) (map[string]interface{}, error) {
	ips := parser.ExtractIPs(result.Description)
	if len(ips) > 0 {
		return map[string]interface{}{"ips": ips}, nil
	}
	return nil, nil
}

// extractTechStacks detects technology stacks from URL/content.
func (e *Extractor) extractTechStacks(ctx context.Context, result *core.Result) (map[string]interface{}, error) {
	// Placeholder for tech stack detection (Wappalyzer-like)
	return nil, nil
}

// extractForms detects form elements in content.
func (e *Extractor) extractForms(ctx context.Context, result *core.Result) (map[string]interface{}, error) {
	// Placeholder for form extraction
	return nil, nil
}

// getExtension returns the file extension from a URL path.
func getExtension(rawURL string) string {
	parts := strings.Split(rawURL, "?")
	path := parts[0]
	idx := strings.LastIndex(path, ".")
	if idx < 0 {
		return ""
	}
	return path[idx:]
}

// FilterSet is a collection of named filters for reuse.
type FilterSet struct {
	Name    string        `json:"name"`
	Filters []core.Filter `json:"filters"`
}

// FilterManager manages named filter sets.
type FilterManager struct {
	mu     sync.RWMutex
	sets   map[string]*FilterSet
	log    *logger.Logger
}

// NewFilterManager creates a new filter manager.
func NewFilterManager(log *logger.Logger) *FilterManager {
	return &FilterManager{
		sets: make(map[string]*FilterSet),
		log:  log,
	}
}

// AddSet adds a named filter set.
func (fm *FilterManager) AddSet(set *FilterSet) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	fm.sets[set.Name] = set
}

// GetSet retrieves a named filter set.
func (fm *FilterManager) GetSet(name string) (*FilterSet, bool) {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	s, ok := fm.sets[name]
	return s, ok
}

// DeleteSet removes a named filter set.
func (fm *FilterManager) DeleteSet(name string) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	delete(fm.sets, name)
}

// ListSets returns all filter set names.
func (fm *FilterManager) ListSets() []string {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	names := make([]string, 0, len(fm.sets))
	for name := range fm.sets {
		names = append(names, name)
	}
	return names
}

// DefaultFilterSets returns commonly used filter sets.
func DefaultFilterSets() []*FilterSet {
	return []*FilterSet{
		{
			Name: "common-web",
			Filters: []core.Filter{
				{Type: core.FilterExtension, Pattern: ".pdf", Negate: true},
				{Type: core.FilterExtension, Pattern: ".jpg", Negate: true},
				{Type: core.FilterExtension, Pattern: ".png", Negate: true},
				{Type: core.FilterExtension, Pattern: ".gif", Negate: true},
				{Type: core.FilterExtension, Pattern: ".svg", Negate: true},
			},
		},
		{
			Name: "no-social",
			Filters: []core.Filter{
				{Type: core.FilterHostBlacklist, Pattern: "facebook.com"},
				{Type: core.FilterHostBlacklist, Pattern: "twitter.com"},
				{Type: core.FilterHostBlacklist, Pattern: "linkedin.com"},
				{Type: core.FilterHostBlacklist, Pattern: "instagram.com"},
				{Type: core.FilterHostBlacklist, Pattern: "youtube.com"},
			},
		},
		{
			Name: "bug-bounty",
			Filters: []core.Filter{
				{Type: core.FilterExtension, Pattern: ".pdf", Negate: true},
				{Type: core.FilterHostBlacklist, Pattern: "facebook.com"},
				{Type: core.FilterHostBlacklist, Pattern: "twitter.com"},
				{Type: core.FilterRegexInclude, Pattern: `\.(com|org|net|io|app|dev|gov|edu)\/`},
			},
		},
	}
}
