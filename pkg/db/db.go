package db

import "time"

// Plugin The plugin information.
type Plugin struct {
	ID            string                 `fauna:"id" json:"id,omitempty"`
	Name          string                 `json:"name,omitempty" fauna:"name"`
	DisplayName   string                 `json:"displayName,omitempty" fauna:"displayName"`
	Author        string                 `json:"author,omitempty" fauna:"author"`
	Type          string                 `json:"type,omitempty" fauna:"type"`
	Import        string                 `json:"import,omitempty" fauna:"import"`
	Compatibility string                 `json:"compatibility,omitempty" fauna:"compatibility"`
	Summary       string                 `json:"summary,omitempty" fauna:"summary"`
	IconURL       string                 `json:"iconUrl,omitempty" fauna:"iconUrl"`
	BannerURL     string                 `json:"bannerUrl,omitempty" fauna:"bannerUrl"`
	Readme        string                 `json:"readme,omitempty" fauna:"readme"`
	LatestVersion string                 `json:"latestVersion,omitempty" fauna:"latestVersion"`
	Versions      []string               `json:"versions,omitempty" fauna:"versions"`
	Stars         int                    `json:"stars,omitempty" fauna:"stars"`
	Snippet       map[string]interface{} `json:"snippet,omitempty" fauna:"snippet"`
	CreatedAt     time.Time              `json:"createdAt" fauna:"createdAt"`
}

// PluginHash The plugin hash tuple..
type PluginHash struct {
	Name string `json:"name,omitempty" fauna:"name"`
	Hash string `json:"hash,omitempty" fauna:"hash"`
}

// Pagination holds information for requesting page.
type Pagination struct {
	Start string
	Size  int
}
