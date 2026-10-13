package provider

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	semrelprovider "github.com/go-semantic-release/semantic-release/v2/pkg/provider"
	"github.com/google/go-github/v66/github"
	"github.com/stretchr/testify/require"
)

func TestGetCommitsScopesHistoryToChartDirectory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, "/repos/acme/charts/commits", request.URL.Path)
		require.Equal(t, "charts/api", request.URL.Query().Get("path"))
		require.Equal(t, "current-sha", request.URL.Query().Get("sha"))
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`[
			{"sha":"api-change","commit":{"message":"feat: api improvement"}},
			{"sha":"api-base","commit":{"message":"chore: api baseline"}}
		]`))
	}))
	defer server.Close()

	client := github.NewClient(nil)
	client.BaseURL, _ = client.BaseURL.Parse(server.URL + "/")
	provider := &GitHubProvider{owner: "acme", repo: "charts", chartName: "api", client: client}

	commits, err := provider.GetCommits("api-base", "current-sha")

	require.NoError(t, err)
	require.Len(t, commits, 1)
	require.Equal(t, "api-change", commits[0].SHA)
	require.Equal(t, "feat: api improvement", commits[0].RawMessage)
}

func TestGetReleasesScopesTagsToChart(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, "/repos/acme/charts/git/matching-refs/tags", request.URL.Path)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`[
			{"ref":"refs/tags/charts/api/v1.2.3","object":{"type":"commit","sha":"api-release"}},
			{"ref":"refs/tags/charts/worker/v9.9.9","object":{"type":"commit","sha":"worker-release"}},
			{"ref":"refs/tags/charts/api/not-semver","object":{"type":"commit","sha":"invalid"}}
		]`))
	}))
	defer server.Close()

	client := github.NewClient(nil)
	client.BaseURL, _ = client.BaseURL.Parse(server.URL + "/")
	provider := &GitHubProvider{owner: "acme", repo: "charts", chartName: "api", client: client}

	releases, err := provider.GetReleases("")

	require.NoError(t, err)
	require.Len(t, releases, 1)
	require.Equal(t, "api-release", releases[0].SHA)
	require.Equal(t, "1.2.3", releases[0].Version)
}

func TestChartTagUsesUniqueChartNamespace(t *testing.T) {
	require.Equal(t, "charts/api/v1.2.3", chartTag("api", "1.2.3"))
	require.Equal(t, "charts/worker/v1.2.3", chartTag("worker", "1.2.3"))
}

func TestCreateReleaseUsesChartScopedTag(t *testing.T) {
	type apiCall struct {
		path string
		body map[string]any
	}
	calls := make(chan apiCall, 2)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body := make(map[string]any)
		if request.Body != nil {
			defer request.Body.Close()
			data, _ := io.ReadAll(request.Body)
			_ = json.Unmarshal(data, &body)
		}
		calls <- apiCall{path: request.URL.Path, body: body}
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusCreated)
		_, _ = writer.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := github.NewClient(nil)
	client.BaseURL, _ = client.BaseURL.Parse(server.URL + "/")
	provider := &GitHubProvider{owner: "acme", repo: "charts", chartName: "api", client: client}

	err := provider.CreateRelease(&semrelprovider.CreateReleaseConfig{
		NewVersion: "1.2.3",
		Changelog:  "API improvements",
		Branch:     "main",
		SHA:        "release-sha",
	})

	require.NoError(t, err)
	tagCall := <-calls
	require.Equal(t, "/repos/acme/charts/git/refs", tagCall.path)
	require.Equal(t, "refs/tags/charts/api/v1.2.3", tagCall.body["ref"])
	require.Equal(t, "release-sha", tagCall.body["sha"])
	releaseCall := <-calls
	require.Equal(t, "/repos/acme/charts/releases", releaseCall.path)
	require.Equal(t, "charts/api/v1.2.3", releaseCall.body["tag_name"])
	require.Equal(t, "api v1.2.3", releaseCall.body["name"])
	require.Equal(t, "API improvements", releaseCall.body["body"])
}
