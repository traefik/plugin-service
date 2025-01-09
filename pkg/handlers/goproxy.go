package handlers

import (
    "io"

    "github.com/ldez/grignotin/goproxy"
    "golang.org/x/mod/modfile"
)

type GoproxyClient struct {
    client *goproxy.Client
}

type GoproxyPluginClient interface {
    DownloadSources(moduleName, version string) (io.ReadCloser, error)
    GetModFile(moduleName, version string) (*modfile.File, error)
}

// NewGoproxyClient creates a new Goproxy client.
func NewGoproxyClient(url, username, password string) (*GoproxyClient, error) {
    gpClient := goproxy.NewClient(url)

    if url != "" && username != "" && password != "" {
        tr, err := goproxy.NewBasicAuthTransport(username, password)
        if err != nil {
            return nil, err
        }

        gpClient.HTTPClient = tr.Client()
    }

    return &GoproxyClient{client: gpClient}, nil
}

func (c GoproxyClient) DownloadSources(moduleName, version string) (io.ReadCloser, error) {
    return c.client.DownloadSources(moduleName, version)
}

func (c GoproxyClient) GetModFile(moduleName, version string) (*modfile.File, error) {
    return c.client.GetModFile(moduleName, version)
}
