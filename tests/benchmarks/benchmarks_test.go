package benchmarks

import (
	"context"
	"strings"
	"testing"

	"github.com/bingdork/bingdork/internal/core"
	"github.com/bingdork/bingdork/internal/logger"
	"github.com/bingdork/bingdork/pkg/engine"
	"github.com/bingdork/bingdork/pkg/extractor"
	"github.com/bingdork/bingdork/pkg/output"
	"github.com/bingdork/bingdork/pkg/parser"
)

// mockProvider for benchmarks
type benchProvider struct {
	id core.ProviderID
}

func (m *benchProvider) ID() core.ProviderID                                       { return m.id }
func (m *benchProvider) Search(ctx context.Context, q *core.SearchQuery) (*core.ResultSet, error) {
	results := make([]*core.Result, 100)
	for i := range results {
		results[i] = &core.Result{
			Title:       "Benchmark Result " + strings.Repeat("x", 50),
			URL:         "https://example.com/page/" + strings.Repeat("x", 20),
			Host:        "example.com",
			RootDomain:  "example.com",
			Description: strings.Repeat("This is a benchmark description. ", 10),
			SearchPos:   i + 1,
			Engine:      string(m.id),
		}
	}
	return &core.ResultSet{Results: results, Query: q.Query, Provider: m.id}, nil
}
func (m *benchProvider) NextPage(ctx context.Context, q *core.SearchQuery) (*core.ResultSet, error) {
	return m.Search(ctx, q)
}
func (m *benchProvider) Health(ctx context.Context) *core.ProviderHealth {
	return &core.ProviderHealth{Provider: m.id, Healthy: true}
}
func (m *benchProvider) Capabilities() *core.ProviderCapabilities {
	return &core.ProviderCapabilities{Pagination: true, MaxPages: 100}
}
func (m *benchProvider) RateLimit(ctx context.Context) *core.RateLimitInfo {
	return &core.RateLimitInfo{RequestsPerMinute: 10}
}
func (m *benchProvider) Metadata(ctx context.Context) map[string]interface{} {
	return map[string]interface{}{"provider": string(m.id)}
}

func BenchmarkEngine_Search(b *testing.B) {
	log := logger.NewNopLogger()
	provider := &benchProvider{id: "bench"}
	eng := engine.New(log, provider)

	ctx := context.Background()
	query := &core.SearchQuery{Query: "benchmark query"}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		eng.Search(ctx, query)
	}
}

func BenchmarkEngine_SearchAll(b *testing.B) {
	log := logger.NewNopLogger()
	providers := make([]core.SearchProvider, 5)
	for i := range providers {
		providers[i] = &benchProvider{id: core.ProviderID(string(rune('A' + i)))}
	}
	eng := engine.New(log, providers...)

	ctx := context.Background()
	query := &core.SearchQuery{Query: "benchmark"}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		eng.SearchAll(ctx, query)
	}
}

func BenchmarkExtractor_Extract(b *testing.B) {
	log := logger.NewNopLogger()
	ex := extractor.New(log)

	results := make([]*core.Result, 1000)
	for i := range results {
		results[i] = &core.Result{
			Title:       "Result " + strings.Repeat("x", 50),
			URL:         "https://example.com/path/" + strings.Repeat("x", 30),
			Host:        "sub.example.com",
			RootDomain:  "example.com",
			Description: strings.Repeat("Description text. ", 20),
			SearchPos:   i + 1,
		}
	}
	rs := &core.ResultSet{Results: results, Query: "test"}

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ex.Extract(ctx, rs)
	}
}

func BenchmarkParser_ParseDomain(b *testing.B) {
	urls := []string{
		"https://www.example.com/path/to/page",
		"https://sub.domain.example.co.uk/resource?query=value",
		"https://192.168.1.1:8080/admin",
		"https://very.deep.subdomain.example.com/long/path/here",
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, u := range urls {
			parser.ParseDomain(u)
		}
	}
}

func BenchmarkParser_NormalizeURL(b *testing.B) {
	urls := []string{
		"https://example.com/page?a=3&b=2&a=1#fragment",
		"http://sub.example.com:8080/path?q=search&lang=en&page=1",
		"https://www.example.com/",
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, u := range urls {
			parser.NormalizeURL(u)
		}
	}
}

func BenchmarkOutput_JSON(b *testing.B) {
	log := logger.NewNopLogger()
	cfg := &core.OutputConfig{PrettyPrint: false}
	exporter, _ := output.NewJSONExporter(cfg, log)

	results := make([]*core.Result, 100)
	for i := range results {
		results[i] = &core.Result{
			Title:       "Result",
			URL:         "https://example.com/page",
			Host:        "example.com",
			RootDomain:  "example.com",
			Description: "Description",
			SearchPos:   i + 1,
		}
	}
	rs := &core.ResultSet{Results: results, Query: "test"}

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		exporter.Export(ctx, rs)
	}
}

func BenchmarkOutput_CSV(b *testing.B) {
	log := logger.NewNopLogger()
	cfg := &core.OutputConfig{}
	exporter, _ := output.NewCSVExporter(cfg, log)

	results := make([]*core.Result, 100)
	for i := range results {
		results[i] = &core.Result{
			Title:       "Result",
			URL:         "https://example.com/page",
			Host:        "example.com",
			RootDomain:  "example.com",
			Description: "Description",
			SearchPos:   i + 1,
		}
	}
	rs := &core.ResultSet{Results: results, Query: "test"}

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		exporter.Export(ctx, rs)
	}
}

func BenchmarkResultSetProcessing(b *testing.B) {
	// Simulates a full pipeline without network
	log := logger.NewNopLogger()
	ex := extractor.New(log)

	rs := &core.ResultSet{
		Query:   "benchmark",
		Results: make([]*core.Result, 1000),
	}
	for i := range rs.Results {
		rs.Results[i] = &core.Result{
			Title:       "Result " + strings.Repeat("x", 40),
			URL:         "https://example.com/path/" + strings.Repeat("x", 20),
			Host:        "example.com",
			RootDomain:  "example.com",
			Description: strings.Repeat("Description. ", 15),
			SearchPos:   i + 1,
			Engine:      "bing",
		}
	}

	ctx := context.Background()
	filters := []core.Filter{
		{Type: core.FilterExtension, Pattern: ".pdf", Negate: true},
	}
	ex.WithFilters(filters)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ex.Extract(ctx, rs)
	}
}
