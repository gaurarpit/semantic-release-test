package provider

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/go-semantic-release/semantic-release/v2/pkg/provider"
	"github.com/go-semantic-release/semantic-release/v2/pkg/semrel"
	"github.com/google/go-github/v66/github"
	"golang.org/x/oauth2"
)

const providerVersion = "0.1.0"

var chartNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

type GitHubProvider struct {
	owner     string
	repo      string
	chartName string
	client    *github.Client
}

func (p *GitHubProvider) Init(config map[string]string) error {
	p.chartName = config["chart_name"]
	if !chartNamePattern.MatchString(p.chartName) {
		return fmt.Errorf("invalid chart_name %q", p.chartName)
	}

	slug := config["slug"]
	if slug == "" {
		slug = os.Getenv("GITHUB_REPOSITORY")
	}
	owner, repo, found := strings.Cut(slug, "/")
	if !found || owner == "" || repo == "" || strings.Contains(repo, "/") {
		return fmt.Errorf("invalid GitHub repository slug %q", slug)
	}

	token := config["token"]
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	if token == "" {
		token = os.Getenv("GH_TOKEN")
	}
	if token == "" {
		return errors.New("GitHub token missing")
	}

	p.owner = owner
	p.repo = repo
	p.client = github.NewClient(oauth2.NewClient(context.Background(), oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})))
	return nil
}

func (p *GitHubProvider) Name() string {
	return "chart-github"
}

func (p *GitHubProvider) Version() string {
	return providerVersion
}

func (p *GitHubProvider) GetInfo() (*provider.RepositoryInfo, error) {
	repository, _, err := p.client.Repositories.Get(context.Background(), p.owner, p.repo)
	if err != nil {
		return nil, err
	}
	return &provider.RepositoryInfo{
		Owner:         repository.GetOwner().GetLogin(),
		Repo:          repository.GetName(),
		DefaultBranch: repository.GetDefaultBranch(),
		Private:       repository.GetPrivate(),
	}, nil
}

func (p *GitHubProvider) GetCommits(fromSha, toSha string) ([]*semrel.RawCommit, error) {
	commits := make([]*semrel.RawCommit, 0)
	options := &github.CommitsListOptions{
		SHA:  toSha,
		Path: path.Join("charts", p.chartName),
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	for {
		pageCommits, response, err := p.client.Repositories.ListCommits(context.Background(), p.owner, p.repo, options)
		if err != nil {
			return nil, err
		}

		foundFromSha := false
		for _, commit := range pageCommits {
			sha := commit.GetSHA()
			if fromSha != "" && sha == fromSha {
				foundFromSha = true
				break
			}
			commits = append(commits, &semrel.RawCommit{
				SHA:        sha,
				RawMessage: commit.Commit.GetMessage(),
			})
		}

		if foundFromSha || response == nil || response.NextPage == 0 {
			break
		}
		options.Page = response.NextPage
	}

	return commits, nil
}

func (p *GitHubProvider) GetReleases(rawRegexp string) ([]*semrel.Release, error) {
	var match *regexp.Regexp
	if rawRegexp != "" {
		var err error
		match, err = regexp.Compile(rawRegexp)
		if err != nil {
			return nil, err
		}
	}

	prefix := chartTagPrefix(p.chartName)
	releases := make([]*semrel.Release, 0)
	options := &github.ReferenceListOptions{
		Ref: "tags",
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	for {
		references, response, err := p.client.Git.ListMatchingRefs(context.Background(), p.owner, p.repo, options)
		if err != nil {
			return nil, err
		}

		for _, reference := range references {
			tag := strings.TrimPrefix(reference.GetRef(), "refs/tags/")
			if !strings.HasPrefix(tag, prefix) || (match != nil && !match.MatchString(tag)) {
				continue
			}

			version, err := semver.NewVersion(strings.TrimPrefix(tag, prefix))
			if err != nil {
				continue
			}

			sha := reference.GetObject().GetSHA()
			if reference.GetObject().GetType() == "tag" {
				annotatedTag, _, err := p.client.Git.GetTag(context.Background(), p.owner, p.repo, sha)
				if err != nil {
					return nil, err
				}
				sha = annotatedTag.GetObject().GetSHA()
			}
			releases = append(releases, &semrel.Release{SHA: sha, Version: version.String()})
		}

		if response == nil || response.NextPage == 0 {
			break
		}
		options.Page = response.NextPage
	}

	return releases, nil
}

func (p *GitHubProvider) CreateRelease(release *provider.CreateReleaseConfig) error {
	tag := chartTag(p.chartName, release.NewVersion)
	ref := "refs/tags/" + tag
	tagReference := &github.Reference{
		Ref: &ref,
		Object: &github.GitObject{
			SHA: &release.SHA,
		},
	}
	if _, _, err := p.client.Git.CreateRef(context.Background(), p.owner, p.repo, tagReference); err != nil {
		return err
	}

	name := fmt.Sprintf("%s v%s", p.chartName, release.NewVersion)
	_, _, err := p.client.Repositories.CreateRelease(context.Background(), p.owner, p.repo, &github.RepositoryRelease{
		TagName:         &tag,
		Name:            &name,
		TargetCommitish: &release.Branch,
		Body:            &release.Changelog,
		Prerelease:      &release.Prerelease,
	})
	return err
}

func chartTagPrefix(chartName string) string {
	return "charts/" + chartName + "/v"
}

func chartTag(chartName, version string) string {
	return chartTagPrefix(chartName) + version
}
