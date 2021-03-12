package db

import "time"

// Plugin The plugin information.
type Plugin struct {
	ID            string                 `fauna:"id" json:"id,omitempty" bson:"id"`
	Name          string                 `json:"name,omitempty" fauna:"name" bson:"name"`
	DisplayName   string                 `json:"displayName,omitempty" fauna:"displayName" bson:"displayName"`
	Author        string                 `json:"author,omitempty" fauna:"author" bson:"author"`
	Type          string                 `json:"type,omitempty" fauna:"type" bson:"type"`
	Import        string                 `json:"import,omitempty" fauna:"import" bson:"import"`
	Compatibility string                 `json:"compatibility,omitempty" fauna:"compatibility" bson:"compatibility"`
	Summary       string                 `json:"summary,omitempty" fauna:"summary" bson:"summary"`
	IconURL       string                 `json:"iconUrl,omitempty" fauna:"iconUrl" bson:"iconUrl"`
	BannerURL     string                 `json:"bannerUrl,omitempty" fauna:"bannerUrl" bson:"bannerUrl"`
	Readme        string                 `json:"readme,omitempty" fauna:"readme" bson:"readme"`
	LatestVersion string                 `json:"latestVersion,omitempty" fauna:"latestVersion" bson:"latestVersion"`
	Versions      []string               `json:"versions,omitempty" fauna:"versions" bson:"versions"`
	Stars         int                    `json:"stars,omitempty" fauna:"stars" bson:"stars"`
	Snippet       map[string]interface{} `json:"snippet,omitempty" fauna:"snippet" bson:"snippet"`
	CreatedAt     time.Time              `json:"createdAt" fauna:"createdAt" bson:"createdAt"`
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
