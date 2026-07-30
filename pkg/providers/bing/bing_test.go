package bing

import (
	"context"
	"testing"
	"time"

	"github.com/bingdork/bingdork/internal/core"
	"github.com/bingdork/bingdork/internal/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanBingURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "https://www.example.com/page",
			expected: "https://www.example.com/page",
		},
		{
			input:    "/url?q=https://www.example.com&src=hp",
			expected: "https://www.example.com",
		},
		{
			input:    "",
			expected: "",
		},
		{
			input:    "javascript:void(0)",
			expected: "https://javascript:void(0)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := cleanBingURL(tt.input)
			if tt.expected == "" {
				assert.Empty(t, result)
			} else {
				assert.Contains(t, result, tt.expected)
			}
		})
	}
}

func TestIsAdLink(t *testing.T) {
	assert.True(t, isAdLink("//ad.example.com"))
	assert.True(t, isAdLink("//www.bing.com/aclick?param=value"))
	assert.True(t, isAdLink("https://doubleclick.net/ad"))
	assert.False(t, isAdLink("https://www.example.com/page"))
	assert.False(t, isAdLink(""))
}

func TestIsExcludedLink(t *testing.T) {
	assert.True(t, isExcludedLink("javascript:void(0)"))
	assert.True(t, isExcludedLink("mailto:user@example.com"))
	assert.True(t, isExcludedLink("#"))
	assert.False(t, isExcludedLink("https://example.com"))
}

func TestNewProvider(t *testing.T) {
	log := logger.NewNopLogger()
	cfg := &core.ProviderConfig{
		Enabled:   true,
		BaseURL:   "https://www.bing.com/search",
		RateLimit: 10,
	}
	netCfg := &core.NetworkConfig{
		Timeout:   30 * time.Second,
		RateLimit: 10,
	}

	provider, err := New(cfg, netCfg, log)
	require.NoError(t, err)
	require.NotNil(t, provider)
	assert.Equal(t, core.ProviderBing, provider.ID())
}

func TestBuildSearchURL(t *testing.T) {
	log := logger.NewNopLogger()
	cfg := &core.ProviderConfig{
		Enabled: true,
		BaseURL: "https://www.bing.com/search",
		Options: map[string]string{"safe_search": "off"},
	}
	netCfg := &core.NetworkConfig{}
	provider, err := New(cfg, netCfg, log)
	require.NoError(t, err)

	query := &core.SearchQuery{
		Query:    "test query",
		Page:     0,
		Options:  map[string]string{},
	}

	url := provider.buildSearchURL(query)
	assert.Contains(t, url, "q=test+query")
	assert.Contains(t, url, "adlt=off")
	assert.Contains(t, url, "setlang=en-US")

	// Test with page
	query.Page = 2
	url = provider.buildSearchURL(query)
	assert.Contains(t, url, "first=21")
}

func TestCapabilities(t *testing.T) {
	log := logger.NewNopLogger()
	cfg := &core.ProviderConfig{
		Enabled:   true,
		RateLimit: 10,
	}
	netCfg := &core.NetworkConfig{
		RateLimit: 10,
	}
	provider, err := New(cfg, netCfg, log)
	require.NoError(t, err)

	caps := provider.Capabilities()
	assert.True(t, caps.Pagination)
	assert.True(t, caps.SafeSearch)
	assert.Greater(t, len(caps.Advanced), 0)
}

func TestParseResultCount(t *testing.T) {
	tests := []struct {
		html     string
		expected int
	}{
		{`<span class="sb_count">1,234 results</span>`, 1234},
		{`<span class="sb_count">500 results</span>`, 500},
		{`No results found`, 0},
		{``, 0},
	}

	for _, tt := range tests {
		result := ParseResultCount(tt.html)
		assert.Equal(t, tt.expected, result)
	}
}

func TestHealth(t *testing.T) {
	log := logger.NewNopLogger()
	cfg := &core.ProviderConfig{Enabled: true}
	netCfg := &core.NetworkConfig{}
	provider, err := New(cfg, netCfg, log)
	require.NoError(t, err)

	health := provider.Health(context.Background())
	assert.NotNil(t, health)
	assert.Equal(t, core.ProviderBing, health.Provider)
}

func TestMetadata(t *testing.T) {
	log := logger.NewNopLogger()
	cfg := &core.ProviderConfig{Enabled: true, BaseURL: "https://www.bing.com/search"}
	netCfg := &core.NetworkConfig{}
	provider, err := New(cfg, netCfg, log)
	require.NoError(t, err)

	meta := provider.Metadata(context.Background())
	assert.Equal(t, "bing", meta["provider"])
	assert.Equal(t, "https://www.bing.com/search", meta["base_url"])
}

func BenchmarkCleanBingURL(b *testing.B) {
	urls := []string{
		"https://www.example.com/page?param=value",
		"/url?q=https://www.example.com&src=hp&ei=test",
		"https://subdomain.example.com/path/to/page.html",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, u := range urls {
			cleanBingURL(u)
		}
	}
}

func BenchmarkBuildSearchURL(b *testing.B) {
	log := logger.NewNopLogger()
	cfg := &core.ProviderConfig{Enabled: true}
	netCfg := &core.NetworkConfig{}
	provider, _ := New(cfg, netCfg, log)
	query := &core.SearchQuery{
		Query:   "test search query",
		Page:    0,
		Options: map[string]string{"language": "en", "region": "US"},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provider.buildSearchURL(query)
	}
}
