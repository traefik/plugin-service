package handlers

import (
    "errors"
    "io"
    "net/http"
    "net/http/httptest"
    "net/url"
    "strings"
    "testing"
    "time"

    "github.com/google/go-github/v57/github"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
    "github.com/stretchr/testify/require"
    "github.com/traefik/plugin-service/pkg/db"
    "golang.org/x/mod/modfile"
    "golang.org/x/mod/module"
)

func Test_cleanModuleName(t *testing.T) {
    testCases := []struct {
        name     string
        expected string
    }{
        {
            name:     "/powpow/",
            expected: "powpow",
        },
        {
            name:     "/powpow/v2",
            expected: "powpow/v2",
        },
        {
            name:     "powpow/v2",
            expected: "powpow/v2",
        },

        {
            name:     "powpow",
            expected: "powpow",
        },

        {
            name:     "powpow/v2/",
            expected: "powpow/v2",
        },
    }

    for _, test := range testCases {
        t.Run(test.name, func(t *testing.T) {
            t.Parallel()

            name := cleanModuleName(test.name)
            assert.Equal(t, test.expected, name)
        })
    }
}

func Test_extractPluginInfo(t *testing.T) {
    type expected struct {
        moduleName string
        version    string
    }

    testCases := []struct {
        desc     string
        url      string
        sep      string
        expected expected
    }{
        {
            desc: "public URL",
            url:  "https://plugins.traefik.io/public/download/github.com/tomMoulard/fail2ban/v0.6.6",
            sep:  "/download/",
            expected: expected{
                moduleName: "github.com/tomMoulard/fail2ban",
                version:    "v0.6.6",
            },
        },
        {
            desc: "internal URL",
            url:  "https://plugins.traefik.io/download/github.com/tomMoulard/fail2ban/v0.6.6",
            sep:  "/download/",
            expected: expected{
                moduleName: "github.com/tomMoulard/fail2ban",
                version:    "v0.6.6",
            },
        },
        {
            desc: "with extra slash",
            url:  "https://plugins.traefik.io/public/download/github.com/tomMoulard/fail2ban/v0.6.6/",
            sep:  "/download/",
            expected: expected{
                moduleName: "github.com/tomMoulard/fail2ban",
                version:    "v0.6.6",
            },
        },
    }

    for _, test := range testCases {
        t.Run(test.desc, func(t *testing.T) {
            t.Parallel()

            endpoint, err := url.Parse(test.url)
            require.NoError(t, err)

            moduleName, version := extractPluginInfo(endpoint, test.sep)

            assert.Equal(t, test.expected.moduleName, moduleName)
            assert.Equal(t, test.expected.version, version)
        })
    }
}

func Test_Download(t *testing.T) {
    data := db.Plugin{
        Author:        "traefik",
        Compatibility: "v2",
        CreatedAt:     time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC),
        DisplayName:   "Demo Plugin",
        ID:            "276809780784267776",
        Import:        "github.com/traefik/plugindemo",
        LatestVersion: "v0.2.1",
        Name:          "github.com/traefik/plugindemo",
        Readme:        "README",
        Snippet:       map[string]interface{}{"toml": "toml", "yaml": "yaml"},
        Stars:         22,
        Summary:       "[Demo] Add Request Header",
        Type:          "middleware",
        Versions:      []string{"v0.2.1", "v0.2.0", "v0.1.0"},
    }

    testDB := NewPluginStorerMock(t).OnGetByName("github.com/traefik/plugindemo", false, false).Once().TypedReturns(data, nil).
        OnGetHashByName("github.com/traefik/plugindemo", "v0.2.1").TypedReturns(db.PluginHash{}, nil).Once().
        Parent

    goproxyMock := NewGoproxyPluginClientMock(t).
        OnGetModFile("github.com/traefik/plugindemo", "v0.2.1").TypedReturns(&modfile.File{}, nil).
        OnDownloadSources("github.com/traefik/plugindemo", "v0.2.1").TypedReturns(io.NopCloser(strings.NewReader("test")), nil).
        Parent
    githubMock := NewGithubPluginClientMock(t)

    rw := httptest.NewRecorder()

    req := httptest.NewRequest(http.MethodGet, "/public/download/github.com/traefik/plugindemo/v0.2.1", http.NoBody)

    New(testDB, goproxyMock, githubMock, 10*time.Second).Download(rw, req)

    assert.Equal(t, http.StatusOK, rw.Code)
    assert.Equal(t, "test", rw.Body.String())
    assert.Equal(t, "max-age=10,s-maxage=10", rw.Header().Get(cacheControlHeader))
}

func Test_Download_withDifferentHash(t *testing.T) {
    data := db.Plugin{
        Author:        "traefik",
        Compatibility: "v2",
        CreatedAt:     time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC),
        DisplayName:   "Demo Plugin",
        ID:            "276809780784267776",
        Import:        "github.com/traefik/plugindemo",
        LatestVersion: "v0.2.1",
        Name:          "github.com/traefik/plugindemo",
        Readme:        "README",
        Snippet:       map[string]interface{}{"toml": "toml", "yaml": "yaml"},
        Stars:         22,
        Summary:       "[Demo] Add Request Header",
        Type:          "middleware",
        Versions:      []string{"v0.2.1", "v0.2.0", "v0.1.0"},
    }

    testDB := NewPluginStorerMock(t).OnGetByName("github.com/traefik/plugindemo", false, false).Once().TypedReturns(data, nil).
        OnGetHashByName("github.com/traefik/plugindemo", "v0.2.1").Once().TypedReturns(db.PluginHash{Hash: "yy"}, nil).Twice().
        Parent

    goproxyMock := NewGoproxyPluginClientMock(t).OnGetModFile("github.com/traefik/plugindemo", "v0.2.1").Once().TypedReturns(&modfile.File{}, nil).
        OnDownloadSources("github.com/traefik/plugindemo", "v0.2.1").Once().TypedReturns(io.NopCloser(strings.NewReader("test")), nil).
        Parent
    githubMock := NewGithubPluginClientMock(t)

    rw := httptest.NewRecorder()

    req := httptest.NewRequest(http.MethodGet, "/public/download/github.com/traefik/plugindemo/v0.2.1", http.NoBody)
    req.Header.Set(hashHeader, "xx")

    New(testDB, goproxyMock, githubMock, 10*time.Second).Download(rw, req)

    assert.Equal(t, http.StatusOK, rw.Code)
    assert.Equal(t, "test", rw.Body.String())
    assert.Equal(t, "max-age=10,s-maxage=10", rw.Header().Get(cacheControlHeader))
}

func Test_Download_handle_GetByName_Error(t *testing.T) {
    testDB := NewPluginStorerMock(t).OnGetByName("github.com/traefik/plugindemo", false, false).Once().TypedReturns(db.Plugin{}, errors.New("test")).Parent

    goproxyMock := NewGoproxyPluginClientMock(t)
    githubMock := NewGithubPluginClientMock(t)

    rw := httptest.NewRecorder()

    req := httptest.NewRequest(http.MethodGet, "/public/download/github.com/traefik/plugindemo/v0.2.1", http.NoBody)

    New(testDB, goproxyMock, githubMock, 10*time.Second).Download(rw, req)

    assert.Equal(t, http.StatusInternalServerError, rw.Code)
    assert.Equal(t, `{"error":"Failed to get plugin github.com/traefik/plugindemo@v0.2.1"}`+"\n", rw.Body.String())
    assert.Equal(t, "no-cache", rw.Header().Get(cacheControlHeader))
}

func Test_Download_handle_GetByName_Error_dbNotFoundError(t *testing.T) {
    testDB := NewPluginStorerMock(t).OnGetByName("github.com/traefik/plugindemo", false, false).Once().TypedReturns(db.Plugin{}, db.NotFoundError{Err: errors.New("test")}).Parent

    goproxyMock := NewGoproxyPluginClientMock(t)
    githubMock := NewGithubPluginClientMock(t)

    rw := httptest.NewRecorder()

    req := httptest.NewRequest(http.MethodGet, "/public/download/github.com/traefik/plugindemo/v0.2.1", http.NoBody)

    New(testDB, goproxyMock, githubMock, 10*time.Second).Download(rw, req)

    assert.Equal(t, http.StatusNotFound, rw.Code)
    assert.Equal(t, `{"error":"Unknown plugin: github.com/traefik/plugindemo@v0.2.1"}`+"\n", rw.Body.String())
    assert.Equal(t, "no-cache", rw.Header().Get(cacheControlHeader))
}

func Test_Download_handle_GetHashByName_Error(t *testing.T) {
    data := db.Plugin{
        Author:        "traefik",
        Compatibility: "v2",
        CreatedAt:     time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC),
        DisplayName:   "Demo Plugin",
        ID:            "276809780784267776",
        Import:        "github.com/traefik/plugindemo",
        LatestVersion: "v0.2.1",
        Name:          "github.com/traefik/plugindemo",
        Readme:        "README",
        Snippet:       map[string]interface{}{"toml": "toml", "yaml": "yaml"},
        Stars:         22,
        Summary:       "[Demo] Add Request Header",
        Type:          "middleware",
        Versions:      []string{"v0.2.1", "v0.2.0", "v0.1.0"},
    }

    testDB := NewPluginStorerMock(t).OnGetByName("github.com/traefik/plugindemo", false, false).Once().TypedReturns(data, nil).
        OnGetHashByName("github.com/traefik/plugindemo", "v0.2.1").TypedReturns(db.PluginHash{}, errors.New("error")).Once().
        Parent

    goproxyMock := NewGoproxyPluginClientMock(t)
    githubMock := NewGithubPluginClientMock(t)

    rw := httptest.NewRecorder()

    req := httptest.NewRequest(http.MethodGet, "/public/download/github.com/traefik/plugindemo/v0.2.1", http.NoBody)
    req.Header.Set(hashHeader, "xx")

    New(testDB, goproxyMock, githubMock, 10*time.Second).Download(rw, req)

    assert.Equal(t, http.StatusInternalServerError, rw.Code)
    assert.Equal(t, `{"error":"Failed to get plugin github.com/traefik/plugindemo@v0.2.1"}`+"\n", rw.Body.String())
    assert.Equal(t, "no-cache", rw.Header().Get(cacheControlHeader))
}

func Test_Download_handle_GetModFile_Error(t *testing.T) {
    data := db.Plugin{
        Author:        "traefik",
        Compatibility: "v2",
        CreatedAt:     time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC),
        DisplayName:   "Demo Plugin",
        ID:            "276809780784267776",
        Import:        "github.com/traefik/plugindemo",
        LatestVersion: "v0.2.1",
        Name:          "github.com/traefik/plugindemo",
        Readme:        "README",
        Snippet:       map[string]interface{}{"toml": "toml", "yaml": "yaml"},
        Stars:         22,
        Summary:       "[Demo] Add Request Header",
        Type:          "middleware",
        Versions:      []string{"v0.2.1", "v0.2.0", "v0.1.0"},
    }

    testDB := NewPluginStorerMock(t).OnGetByName("github.com/traefik/plugindemo", false, false).Once().TypedReturns(data, nil).Parent

    goproxyMock := NewGoproxyPluginClientMock(t).
        OnGetModFile("github.com/traefik/plugindemo", "v0.2.1").TypedReturns(&modfile.File{}, errors.New("error")).Parent
    githubMock := NewGithubPluginClientMock(t)

    rw := httptest.NewRecorder()

    req := httptest.NewRequest(http.MethodGet, "/public/download/github.com/traefik/plugindemo/v0.2.1", http.NoBody)

    New(testDB, goproxyMock, githubMock, 10*time.Second).Download(rw, req)

    assert.Equal(t, http.StatusInternalServerError, rw.Code)
    assert.Equal(t, `{"error":"Failed to get plugin github.com/traefik/plugindemo@v0.2.1"}`+"\n", rw.Body.String())
    assert.Equal(t, "no-cache", rw.Header().Get(cacheControlHeader))
}

func Test_Download_unhandledMethod(t *testing.T) {
    testDB := NewPluginStorerMock(t)

    goproxyMock := NewGoproxyPluginClientMock(t)
    githubMock := NewGithubPluginClientMock(t)

    rw := httptest.NewRecorder()

    req := httptest.NewRequest(http.MethodHead, "/public/download/github.com/traefik/plugindemo/v0.2.1", http.NoBody)

    New(testDB, goproxyMock, githubMock, 10*time.Second).Download(rw, req)

    assert.Equal(t, http.StatusMethodNotAllowed, rw.Code)
}

func Test_Download_wasm(t *testing.T) {
    data := db.Plugin{
        Author:        "traefik",
        Compatibility: "v2",
        CreatedAt:     time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC),
        DisplayName:   "Demo Plugin",
        ID:            "276809780784267776",
        Import:        "github.com/traefik/plugindemowasm",
        LatestVersion: "v0.0.1",
        Name:          "github.com/traefik/plugindemowasm",
        Readme:        "README",
        Runtime:       "wasm",
        Snippet:       map[string]interface{}{"toml": "toml", "yaml": "yaml"},
        Stars:         22,
        Summary:       "Demo Plugin WASM",
        Type:          "middleware",
        Versions:      []string{"v0.0.1"},
    }

    testDB := NewPluginStorerMock(t).OnGetByName("github.com/traefik/plugindemowasm", false, false).Once().TypedReturns(data, nil).
        OnGetHashByName("github.com/traefik/plugindemowasm", "v0.0.1").TypedReturns(db.PluginHash{}, nil).Once().
        Parent

    goproxyMock := NewGoproxyPluginClientMock(t)

    rw := httptest.NewRecorder()
    link := &url.URL{Scheme: "https", Host: "api.github.com", Path: "/repos/traefik/plugindemowasm/releases/assets/138238821"}

    githubMock := NewGithubPluginClientMock(t).
        OnGetReleaseByTag("traefik", "plugindemowasm", "v0.0.1").TypedReturns(&github.RepositoryRelease{
        Assets: []*github.ReleaseAsset{
            {Name: github.String("plugindemowasm_0.0.1_checksums.txt"), URL: github.String("https://api.github.com/repos/traefik/plugindemowasm/releases/assets/138238820")},
            {Name: github.String("plugindemowasm_v0.0.1.zip"), URL: github.String("https://api.github.com/repos/traefik/plugindemowasm/releases/assets/138238821")},
        }}, nil, nil).Once().
        OnDoRaw(mock.MatchedBy(func(req *http.Request) bool {
            if req.URL.String() != link.String() {
                return false
            }
            rw.WriteHeader(http.StatusOK)
            _, _ = rw.Write([]byte("test"))

            return true
        }), rw).TypedReturns(nil, nil).
        Parent

    req := httptest.NewRequest(http.MethodGet, "/public/download/github.com/traefik/plugindemowasm/v0.0.1", http.NoBody)

    New(testDB, goproxyMock, githubMock, 10*time.Second).Download(rw, req)

    assert.Equal(t, http.StatusOK, rw.Code)
    assert.Equal(t, "test", rw.Body.String())
    assert.Equal(t, "max-age=10,s-maxage=10", rw.Header().Get(cacheControlHeader))
}

func Test_Download_withHash(t *testing.T) {
    data := db.Plugin{
        Author:        "traefik",
        Compatibility: "v2",
        CreatedAt:     time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC),
        DisplayName:   "Demo Plugin",
        ID:            "276809780784267776",
        Import:        "github.com/traefik/plugindemo",
        LatestVersion: "v0.2.1",
        Name:          "github.com/traefik/plugindemo",
        Readme:        "README",
        Snippet:       map[string]interface{}{"toml": "toml", "yaml": "yaml"},
        Stars:         22,
        Summary:       "[Demo] Add Request Header",
        Type:          "middleware",
        Versions:      []string{"v0.2.1", "v0.2.0", "v0.1.0"},
    }

    testDB := NewPluginStorerMock(t).OnGetByName("github.com/traefik/plugindemo", false, false).Once().TypedReturns(data, nil).
        OnGetHashByName("github.com/traefik/plugindemo", "v0.2.1").TypedReturns(db.PluginHash{Hash: "xx"}, nil).Once().
        Parent

    goproxyMock := NewGoproxyPluginClientMock(t)
    githubMock := NewGithubPluginClientMock(t)

    rw := httptest.NewRecorder()

    req := httptest.NewRequest(http.MethodGet, "/public/download/github.com/traefik/plugindemo/v0.2.1", http.NoBody)
    req.Header.Set(hashHeader, "xx")

    New(testDB, goproxyMock, githubMock, 10*time.Second).Download(rw, req)

    assert.Equal(t, http.StatusNotModified, rw.Code)
    assert.Equal(t, "", rw.Body.String())
    assert.Equal(t, "max-age=10,s-maxage=10", rw.Header().Get(cacheControlHeader))
}

func Test_Download_withRequirements(t *testing.T) {
    data := db.Plugin{
        Author:        "maxlerebourg",
        Compatibility: "",
        CreatedAt:     time.Date(2020, 9, 29, 6, 0, 12, 517000, time.UTC),
        DisplayName:   "Crowdsec Bouncer Traefik Plugin",
        ID:            "6335346ca4caa9ddeffda116",
        Import:        "github.com/maxlerebourg/crowdsec-bouncer-traefik-plugin",
        LatestVersion: "v1.3.5",
        Name:          "github.com/maxlerebourg/crowdsec-bouncer-traefik-plugin",
        Readme:        "![GitHub](https://img.shields.io/github/license/maxlerebourg/crowdsec-bouncer-traefik-plugin)\n...",
        Snippet:       map[string]interface{}{"toml": "toml", "yaml": "yaml"},
        Stars:         22,
        Summary:       "Middleware plugin which forwards the request IP to local Crowdsec agent, which can be used to allow/deny the request",
        Type:          "middleware",
        Versions:      []string{"v1.3.5", "v1.3.4"},
    }

    testDB := NewPluginStorerMock(t).OnGetByName("github.com/maxlerebourg/crowdsec-bouncer-traefik-plugin", false, false).Once().TypedReturns(data, nil).
        OnGetHashByName("github.com/maxlerebourg/crowdsec-bouncer-traefik-plugin", "v1.3.5").TypedReturns(db.PluginHash{}, nil).Once().
        Parent

    goproxyMock := NewGoproxyPluginClientMock(t).
        OnGetModFile("github.com/maxlerebourg/crowdsec-bouncer-traefik-plugin", "v1.3.5").TypedReturns(
        &modfile.File{
            Require: []*modfile.Require{
                {Mod: module.Version{Path: "github.com/leprosus/golang-ttl-map", Version: "v1.1.7"}, Indirect: false},
                {Mod: module.Version{Path: "github.com/maxlerebourg/simpleredis", Version: "v1.0.11"}, Indirect: false},
            },
        }, nil).
        Parent

    rw := httptest.NewRecorder()
    link := &url.URL{Scheme: "https", Host: "codeload.github.com", Path: "/maxlerebourg/crowdsec-bouncer-traefik-plugin/legacy.zip/refs/tags/v1.3.5"}
    githubMock := NewGithubPluginClientMock(t).OnGetArchiveLink("maxlerebourg", "crowdsec-bouncer-traefik-plugin", "zipball", &github.RepositoryContentGetOptions{Ref: "v1.3.5"}, 3).TypedReturns(link, &github.Response{Response: &http.Response{StatusCode: http.StatusFound, Header: map[string][]string{"location": {link.String()}}}}, nil).Once().
        OnDoRaw(mock.MatchedBy(func(req *http.Request) bool {
            if req.URL.String() != link.String() {
                return false
            }
            rw.WriteHeader(http.StatusOK)
            _, _ = rw.Write([]byte("test"))

            return true
        }), rw).TypedReturns(nil, nil).Parent

    req := httptest.NewRequest(http.MethodGet, "/public/download/github.com/maxlerebourg/crowdsec-bouncer-traefik-plugin/v1.3.5", http.NoBody)

    New(testDB, goproxyMock, githubMock, 10*time.Second).Download(rw, req)

    assert.Equal(t, http.StatusOK, rw.Code)
    assert.Equal(t, "test", rw.Body.String())
    assert.Equal(t, "max-age=10,s-maxage=10", rw.Header().Get(cacheControlHeader))
}

func Test_Download_withRequirements_handle_Do_Error(t *testing.T) {
    data := db.Plugin{
        Author:        "maxlerebourg",
        Compatibility: "",
        CreatedAt:     time.Date(2020, 9, 29, 6, 0, 12, 517000, time.UTC),
        DisplayName:   "Crowdsec Bouncer Traefik Plugin",
        ID:            "6335346ca4caa9ddeffda116",
        Import:        "github.com/maxlerebourg/crowdsec-bouncer-traefik-plugin",
        LatestVersion: "v1.3.5",
        Name:          "github.com/maxlerebourg/crowdsec-bouncer-traefik-plugin",
        Readme:        "![GitHub](https://img.shields.io/github/license/maxlerebourg/crowdsec-bouncer-traefik-plugin)\n...",
        Snippet:       map[string]interface{}{"toml": "toml", "yaml": "yaml"},
        Stars:         22,
        Summary:       "Middleware plugin which forwards the request IP to local Crowdsec agent, which can be used to allow/deny the request",
        Type:          "middleware",
        Versions:      []string{"v1.3.5", "v1.3.4"},
    }

    testDB := NewPluginStorerMock(t).OnGetByName("github.com/maxlerebourg/crowdsec-bouncer-traefik-plugin", false, false).Once().TypedReturns(data, nil).
        OnGetHashByName("github.com/maxlerebourg/crowdsec-bouncer-traefik-plugin", "v1.3.5").TypedReturns(db.PluginHash{}, nil).Once().
        Parent

    goproxyMock := NewGoproxyPluginClientMock(t).
        OnGetModFile("github.com/maxlerebourg/crowdsec-bouncer-traefik-plugin", "v1.3.5").TypedReturns(
        &modfile.File{
            Require: []*modfile.Require{
                {Mod: module.Version{Path: "github.com/leprosus/golang-ttl-map", Version: "v1.1.7"}, Indirect: false},
                {Mod: module.Version{Path: "github.com/maxlerebourg/simpleredis", Version: "v1.0.11"}, Indirect: false},
            },
        }, nil).
        Parent

    rw := httptest.NewRecorder()
    link := &url.URL{Scheme: "https", Host: "codeload.github.com", Path: "/maxlerebourg/crowdsec-bouncer-traefik-plugin/legacy.zip/refs/tags/v1.3.5"}
    githubMock := NewGithubPluginClientMock(t).OnGetArchiveLink("maxlerebourg", "crowdsec-bouncer-traefik-plugin", "zipball", &github.RepositoryContentGetOptions{Ref: "v1.3.5"}, 3).TypedReturns(link, &github.Response{Response: &http.Response{StatusCode: http.StatusFound, Header: map[string][]string{"location": {link.String()}}}}, nil).Once().
        OnDoRaw(mock.MatchedBy(func(req *http.Request) bool {
            return req.URL.String() == link.String()
        }), rw).TypedReturns(nil, errors.New("error")).Parent

    req := httptest.NewRequest(http.MethodGet, "/public/download/github.com/maxlerebourg/crowdsec-bouncer-traefik-plugin/v1.3.5", http.NoBody)

    New(testDB, goproxyMock, githubMock, 10*time.Second).Download(rw, req)

    assert.Equal(t, http.StatusInternalServerError, rw.Code)
    assert.Equal(t, `{"error":"Failed to get plugin github.com/maxlerebourg/crowdsec-bouncer-traefik-plugin@v1.3.5"}`+"\n", rw.Body.String())
    assert.Equal(t, "no-cache", rw.Header().Get(cacheControlHeader))
}

func Test_Download_withRequirements_handle_GetArchiveLink_Error(t *testing.T) {
    data := db.Plugin{
        Author:        "maxlerebourg",
        Compatibility: "",
        CreatedAt:     time.Date(2020, 9, 29, 6, 0, 12, 517000, time.UTC),
        DisplayName:   "Crowdsec Bouncer Traefik Plugin",
        ID:            "6335346ca4caa9ddeffda116",
        Import:        "github.com/maxlerebourg/crowdsec-bouncer-traefik-plugin",
        LatestVersion: "v1.3.5",
        Name:          "github.com/maxlerebourg/crowdsec-bouncer-traefik-plugin",
        Readme:        "![GitHub](https://img.shields.io/github/license/maxlerebourg/crowdsec-bouncer-traefik-plugin)\n...",
        Snippet:       map[string]interface{}{"toml": "toml", "yaml": "yaml"},
        Stars:         22,
        Summary:       "Middleware plugin which forwards the request IP to local Crowdsec agent, which can be used to allow/deny the request",
        Type:          "middleware",
        Versions:      []string{"v1.3.5", "v1.3.4"},
    }

    testDB := NewPluginStorerMock(t).OnGetByName("github.com/maxlerebourg/crowdsec-bouncer-traefik-plugin", false, false).Once().TypedReturns(data, nil).Parent

    goproxyMock := NewGoproxyPluginClientMock(t).
        OnGetModFile("github.com/maxlerebourg/crowdsec-bouncer-traefik-plugin", "v1.3.5").TypedReturns(
        &modfile.File{
            Require: []*modfile.Require{
                {Mod: module.Version{Path: "github.com/leprosus/golang-ttl-map", Version: "v1.1.7"}, Indirect: false},
                {Mod: module.Version{Path: "github.com/maxlerebourg/simpleredis", Version: "v1.0.11"}, Indirect: false},
            },
        }, nil).
        Parent

    rw := httptest.NewRecorder()
    githubMock := NewGithubPluginClientMock(t).OnGetArchiveLink("maxlerebourg", "crowdsec-bouncer-traefik-plugin", "zipball", &github.RepositoryContentGetOptions{Ref: "v1.3.5"}, 3).TypedReturns(nil, nil, errors.New("error")).Once().Parent

    req := httptest.NewRequest(http.MethodGet, "/public/download/github.com/maxlerebourg/crowdsec-bouncer-traefik-plugin/v1.3.5", http.NoBody)

    New(testDB, goproxyMock, githubMock, 10*time.Second).Download(rw, req)

    assert.Equal(t, http.StatusInternalServerError, rw.Code)
    assert.Equal(t, `{"error":"Failed to get plugin github.com/maxlerebourg/crowdsec-bouncer-traefik-plugin@v1.3.5"}`+"\n", rw.Body.String())
    assert.Equal(t, "no-cache", rw.Header().Get(cacheControlHeader))
}

func Test_Download_withRequirements_handle_GetHashByName_Error(t *testing.T) {
    data := db.Plugin{
        Author:        "maxlerebourg",
        Compatibility: "",
        CreatedAt:     time.Date(2020, 9, 29, 6, 0, 12, 517000, time.UTC),
        DisplayName:   "Crowdsec Bouncer Traefik Plugin",
        ID:            "6335346ca4caa9ddeffda116",
        Import:        "github.com/maxlerebourg/crowdsec-bouncer-traefik-plugin",
        LatestVersion: "v1.3.5",
        Name:          "github.com/maxlerebourg/crowdsec-bouncer-traefik-plugin",
        Readme:        "![GitHub](https://img.shields.io/github/license/maxlerebourg/crowdsec-bouncer-traefik-plugin)\n...",
        Snippet:       map[string]interface{}{"toml": "toml", "yaml": "yaml"},
        Stars:         22,
        Summary:       "Middleware plugin which forwards the request IP to local Crowdsec agent, which can be used to allow/deny the request",
        Type:          "middleware",
        Versions:      []string{"v1.3.5", "v1.3.4"},
    }

    testDB := NewPluginStorerMock(t).OnGetByName("github.com/maxlerebourg/crowdsec-bouncer-traefik-plugin", false, false).Once().TypedReturns(data, nil).
        OnGetHashByName("github.com/maxlerebourg/crowdsec-bouncer-traefik-plugin", "v1.3.5").TypedReturns(db.PluginHash{}, errors.New("error")).Once().
        Parent

    goproxyMock := NewGoproxyPluginClientMock(t).
        OnGetModFile("github.com/maxlerebourg/crowdsec-bouncer-traefik-plugin", "v1.3.5").TypedReturns(
        &modfile.File{
            Require: []*modfile.Require{
                {Mod: module.Version{Path: "github.com/leprosus/golang-ttl-map", Version: "v1.1.7"}, Indirect: false},
                {Mod: module.Version{Path: "github.com/maxlerebourg/simpleredis", Version: "v1.0.11"}, Indirect: false},
            },
        }, nil).
        Parent

    rw := httptest.NewRecorder()
    link := &url.URL{Scheme: "https", Host: "codeload.github.com", Path: "/maxlerebourg/crowdsec-bouncer-traefik-plugin/legacy.zip/refs/tags/v1.3.5"}
    githubMock := NewGithubPluginClientMock(t).OnGetArchiveLink("maxlerebourg", "crowdsec-bouncer-traefik-plugin", "zipball", &github.RepositoryContentGetOptions{Ref: "v1.3.5"}, 3).TypedReturns(link, &github.Response{Response: &http.Response{StatusCode: http.StatusFound, Header: map[string][]string{"location": {link.String()}}}}, nil).Once().Parent

    req := httptest.NewRequest(http.MethodGet, "/public/download/github.com/maxlerebourg/crowdsec-bouncer-traefik-plugin/v1.3.5", http.NoBody)

    New(testDB, goproxyMock, githubMock, 10*time.Second).Download(rw, req)

    assert.Equal(t, http.StatusInternalServerError, rw.Code)
    assert.Equal(t, `{"error":"Failed to get plugin github.com/maxlerebourg/crowdsec-bouncer-traefik-plugin@v1.3.5"}`+"\n", rw.Body.String())
    assert.Equal(t, "no-cache", rw.Header().Get(cacheControlHeader))
}
