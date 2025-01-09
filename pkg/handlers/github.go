package handlers

import (
    "context"
    "net/http"
    "net/url"

    "github.com/google/go-github/v57/github"
    "golang.org/x/oauth2"
)

type GithubClient struct {
    client *github.Client
}

type GithubPluginClient interface {
    Do(ctx context.Context, req *http.Request, v interface{}) (*github.Response, error)
    GetArchiveLink(ctx context.Context, owner, repo string, archiveformat github.ArchiveFormat, opts *github.RepositoryContentGetOptions, maxRedirects int) (*url.URL, *github.Response, error)
    GetReleaseByTag(ctx context.Context, owner, repo, tag string) (*github.RepositoryRelease, *github.Response, error)
}

// NewGithubClient create a new GitHub client.
func NewGithubClient(ctx context.Context, token string) *GithubClient {
    if token == "" {
        return &GithubClient{github.NewClient(nil)}
    }

    ts := oauth2.StaticTokenSource(
        &oauth2.Token{AccessToken: token},
    )

    return &GithubClient{github.NewClient(oauth2.NewClient(ctx, ts))}
}

func (c GithubClient) Do(ctx context.Context, req *http.Request, v interface{}) (*github.Response, error) {
    return c.client.Do(ctx, req, v)
}

func (c GithubClient) GetArchiveLink(ctx context.Context, owner, repo string, archiveformat github.ArchiveFormat, opts *github.RepositoryContentGetOptions, maxRedirects int) (*url.URL, *github.Response, error) {
    return c.client.Repositories.GetArchiveLink(ctx, owner, repo, archiveformat, opts, maxRedirects)
}

func (c GithubClient) GetReleaseByTag(ctx context.Context, owner, repo, tag string) (*github.RepositoryRelease, *github.Response, error) {
    return c.client.Repositories.GetReleaseByTag(ctx, owner, repo, tag)
}
