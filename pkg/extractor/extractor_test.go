package extractor

import (
	"context"
	"testing"

	"github.com/bingdork/bingdork/internal/core"
	"github.com/bingdork/bingdork/internal/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testResultSet() *core.ResultSet {
	return &core.ResultSet{
		Query:    "test",
		Provider: "bing",
		Results: []*core.Result{
			{
				Title:       "Example Page",
				URL:         "https://www.example.com/page",
				Host:        "www.example.com",
				RootDomain:  "example.com",
				Description: "This is a test page with admin@example.com email",
				SearchPos:   1,
				Engine:      "bing",
			},
			{
				Title:       "Admin Panel",
				URL:         "https://admin.example.com/login",
				Host:        "admin.example.com",
				RootDomain:  "example.com",
				Description: "Administrator login panel",
				SearchPos:   2,
				Engine:      "bing",
			},
			{
				Title:       "PDF Document",
				URL:         "https://docs.example.com/report.pdf",
				Host:        "docs.example.com",
				RootDomain:  "example.com",
				Description: "Annual report PDF",
				SearchPos:   3,
				Engine:      "bing",
			},
		},
	}
}

func TestNew(t *testing.T) {
	log := logger.NewNopLogger()
	ex := New(log)
	require.NotNil(t, ex)
}

func TestExtract(t *testing.T) {
	log := logger.NewNopLogger()
	ex := New(log)

	ctx := context.Background()
	resultSet, err := ex.Extract(ctx, testResultSet())
	require.NoError(t, err)
	require.NotNil(t, resultSet)
	assert.Len(t, resultSet.Results, 3)
	assert.Equal(t, "test", resultSet.Query)
}

func TestFilter_RegexInclude(t *testing.T) {
	log := logger.NewNopLogger()
	ex := New(log)

	filter := core.Filter{
		Type:    core.FilterRegexInclude,
		Pattern: `admin\.`,
	}
	ex.AddFilter(filter)

	ctx := context.Background()
	resultSet, err := ex.Extract(ctx, testResultSet())
	require.NoError(t, err)
	assert.Len(t, resultSet.Results, 1)
	assert.Contains(t, resultSet.Results[0].URL, "admin")
}

func TestFilter_RegexExclude(t *testing.T) {
	log := logger.NewNopLogger()
	ex := New(log)

	filter := core.Filter{
		Type:    core.FilterRegexExclude,
		Pattern: `\.pdf$`,
	}
	ex.AddFilter(filter)

	ctx := context.Background()
	resultSet, err := ex.Extract(ctx, testResultSet())
	require.NoError(t, err)
	assert.Len(t, resultSet.Results, 2)
}

func TestFilter_HostWhitelist(t *testing.T) {
	log := logger.NewNopLogger()
	ex := New(log)

	filter := core.Filter{
		Type:    core.FilterHostWhitelist,
		Pattern: "admin.example.com",
	}
	ex.AddFilter(filter)

	ctx := context.Background()
	resultSet, err := ex.Extract(ctx, testResultSet())
	require.NoError(t, err)
	assert.Len(t, resultSet.Results, 1)
	assert.Equal(t, "admin.example.com", resultSet.Results[0].Host)
}

func TestFilter_HostBlacklist(t *testing.T) {
	log := logger.NewNopLogger()
	ex := New(log)

	filter := core.Filter{
		Type:    core.FilterHostBlacklist,
		Pattern: "admin.example.com",
	}
	ex.AddFilter(filter)

	ctx := context.Background()
	resultSet, err := ex.Extract(ctx, testResultSet())
	require.NoError(t, err)
	assert.Len(t, resultSet.Results, 2) // exclude admin.example.com
}

func TestFilter_Extension(t *testing.T) {
	log := logger.NewNopLogger()
	ex := New(log)

	filter := core.Filter{
		Type:    core.FilterExtension,
		Pattern: ".pdf",
		Negate:  true, // exclude PDFs
	}
	ex.AddFilter(filter)

	ctx := context.Background()
	resultSet, err := ex.Extract(ctx, testResultSet())
	require.NoError(t, err)
	assert.Len(t, resultSet.Results, 2)
}

func TestFilter_Keyword(t *testing.T) {
	log := logger.NewNopLogger()
	ex := New(log)

	filter := core.Filter{
		Type:    core.FilterKeyword,
		Pattern: "admin",
	}
	ex.AddFilter(filter)

	ctx := context.Background()
	resultSet, err := ex.Extract(ctx, testResultSet())
	require.NoError(t, err)
	assert.Len(t, resultSet.Results, 2)
	assert.Contains(t, resultSet.Results[0].Title, "Example")
	assert.Contains(t, resultSet.Results[1].Title, "Admin")
}

func TestMultipleFilters(t *testing.T) {
	log := logger.NewNopLogger()
	ex := New(log)

	filters := []core.Filter{
		{Type: core.FilterExtension, Pattern: ".pdf", Negate: true},
		{Type: core.FilterHostBlacklist, Pattern: "admin.example.com"},
	}
	ex.WithFilters(filters)

	ctx := context.Background()
	resultSet, err := ex.Extract(ctx, testResultSet())
	require.NoError(t, err)
	assert.Len(t, resultSet.Results, 1) // only www.example.com remains
}

func TestFilterManager(t *testing.T) {
	log := logger.NewNopLogger()
	fm := NewFilterManager(log)

	set := &FilterSet{
		Name: "test-set",
		Filters: []core.Filter{
			{Type: core.FilterExtension, Pattern: ".pdf", Negate: true},
		},
	}
	fm.AddSet(set)

	got, ok := fm.GetSet("test-set")
	assert.True(t, ok)
	assert.Equal(t, "test-set", got.Name)

	sets := fm.ListSets()
	assert.Contains(t, sets, "test-set")

	fm.DeleteSet("test-set")
	_, ok = fm.GetSet("test-set")
	assert.False(t, ok)
}

func TestDefaultFilterSets(t *testing.T) {
	sets := DefaultFilterSets()
	assert.Greater(t, len(sets), 0)
	assert.Equal(t, "common-web", sets[0].Name)
	assert.Equal(t, "no-social", sets[1].Name)
	assert.Equal(t, "bug-bounty", sets[2].Name)
}

func TestContextCancellation(t *testing.T) {
	log := logger.NewNopLogger()
	ex := New(log)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediately cancel

	_, err := ex.Extract(ctx, testResultSet())
	assert.Error(t, err)
}

func TestRegisterCustomExtractor(t *testing.T) {
	log := logger.NewNopLogger()
	ex := New(log)

	called := false
	customFn := func(ctx context.Context, result *core.Result) (map[string]interface{}, error) {
		called = true
		return map[string]interface{}{"custom": "value"}, nil
	}
	ex.RegisterExtractor(customFn)

	ctx := context.Background()
	_, err := ex.Extract(ctx, testResultSet())
	require.NoError(t, err)
	assert.True(t, called)
}

func BenchmarkExtract(b *testing.B) {
	log := logger.NewNopLogger()
	ex := New(log)

	// Create a large result set
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
	rs := &core.ResultSet{
		Query:   "test",
		Results: results,
	}

	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ex.Extract(ctx, rs)
	}
}
