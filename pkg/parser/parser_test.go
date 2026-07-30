package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDomain(t *testing.T) {
	tests := []struct {
		url        string
		host       string
		rootDomain string
	}{
		{"https://www.example.com/path", "www.example.com", "example.com"},
		{"https://sub.domain.example.co.uk/path", "sub.domain.example.co.uk", "example.co.uk"},
		{"https://example.com", "example.com", "example.com"},
		{"http://192.168.1.1:8080", "192.168.1.1", "1.1"},
		{"invalid-url", "invalid-url", "invalid-url"},
		{"", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			host, domain := ParseDomain(tt.url)
			assert.Equal(t, tt.host, host)
			assert.Equal(t, tt.rootDomain, domain)
		})
	}
}

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			"https://example.com/page?b=2&a=1#fragment",
			"https://example.com/page?a=1&b=2",
		},
		{
			"http://example.com/",
			"http://example.com/",
		},
		{
			"example.com",
			"https://example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := NormalizeURL(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCanonicalURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			"https://www.Example.com/Path/?query=value#frag",
			"https://www.example.com/Path",
		},
		{
			"https://example.com/",
			"https://example.com/",
		},
		{
			"http://EXAMPLE.COM:8080",
			"http://example.com:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := CanonicalURL(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsSubdomain(t *testing.T) {
	assert.True(t, IsSubdomain("sub.example.com", "example.com"))
	assert.True(t, IsSubdomain("deep.sub.example.com", "example.com"))
	assert.False(t, IsSubdomain("example.com", "example.com"))
	assert.False(t, IsSubdomain("other.com", "example.com"))
}

func TestExtractURLs(t *testing.T) {
	body := `Visit https://example.com and http://test.org/path for more info.`
	urls := ExtractURLs(body)
	assert.Len(t, urls, 2)
	assert.Contains(t, urls[0], "example.com")
}

func TestExtractEmails(t *testing.T) {
	body := "Contact admin@example.com or support@test.org for help."
	emails := ExtractEmails(body)
	assert.Len(t, emails, 2)
	assert.Contains(t, emails, "admin@example.com")
}

func TestParseURL(t *testing.T) {
	meta, err := ParseURL("https://sub.example.com:8080/path/to/page?q=1#section")
	require.NoError(t, err)
	assert.Equal(t, "https", meta.Scheme)
	assert.Equal(t, "sub.example.com", meta.Host)
	assert.Equal(t, "example.com", meta.RootDomain)
	assert.Equal(t, "8080", meta.Port)
	assert.Equal(t, "/path/to/page", meta.Path)
	assert.Equal(t, "section", meta.Fragment)
	assert.Equal(t, "com", meta.TLD)
	assert.False(t, meta.IsIP)
	assert.True(t, meta.IsSSL)
	assert.Equal(t, 3, meta.PathDepth)
}

func TestIsValidURL(t *testing.T) {
	assert.True(t, IsValidURL("https://example.com"))
	assert.True(t, IsValidURL("http://example.com/path?q=1"))
	assert.False(t, IsValidURL("not-a-url"))
	assert.False(t, IsValidURL(""))
}

func TestGetSubdomainParts(t *testing.T) {
	assert.Equal(t, []string{"sub"}, GetSubdomainParts("sub.example.com"))
	assert.Equal(t, []string{"deep", "sub"}, GetSubdomainParts("deep.sub.example.com"))
	assert.Nil(t, GetSubdomainParts("example.com"))
	assert.Nil(t, GetSubdomainParts("localhost"))
}

func TestContainsGoogleAnalytics(t *testing.T) {
	assert.True(t, ContainsGoogleAnalytics("https://example.com?utm_source=test"))
	assert.False(t, ContainsGoogleAnalytics("https://example.com?param=value"))
}

func TestExtractURLs_LargeInput(t *testing.T) {
	body := ""
	for i := 0; i < 1000; i++ {
		body += " visit https://example.com/page" + string(rune('0'+i%10)) + " "
	}
	urls := ExtractURLs(body)
	assert.Greater(t, len(urls), 0)
}

func BenchmarkParseDomain(b *testing.B) {
	urls := []string{
		"https://www.example.com/path/to/page",
		"https://sub.domain.example.co.uk/resource",
		"https://192.168.1.1:8080/admin",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, u := range urls {
			ParseDomain(u)
		}
	}
}

func BenchmarkNormalizeURL(b *testing.B) {
	urls := []string{
		"https://example.com/page?b=2&a=1&c=3#fragment",
		"http://sub.example.com:8080/path?q=search&lang=en",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, u := range urls {
			NormalizeURL(u)
		}
	}
}
