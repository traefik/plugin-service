package db

import "time"

// Plugin The plugin information.
type Plugin struct {
	ID            string                 `json:"id,omitempty" bson:"id"`
	Name          string                 `json:"name,omitempty" bson:"name"`
	DisplayName   string                 `json:"displayName,omitempty" bson:"displayName"`
	Author        string                 `json:"author,omitempty" bson:"author"`
	Type          string                 `json:"type,omitempty" bson:"type"`
	Import        string                 `json:"import,omitempty" bson:"import"`
	Compatibility string                 `json:"compatibility,omitempty" bson:"compatibility"`
	Summary       string                 `json:"summary,omitempty" bson:"summary"`
	IconURL       string                 `json:"iconUrl,omitempty" bson:"iconUrl"`
	BannerURL     string                 `json:"bannerUrl,omitempty" bson:"bannerUrl"`
	Readme        string                 `json:"readme,omitempty" bson:"readme"`
	LatestVersion string                 `json:"latestVersion,omitempty" bson:"latestVersion"`
	Versions      []string               `json:"versions,omitempty" bson:"versions"`
	Stars         int                    `json:"stars,omitempty" bson:"stars"`
	Snippet       map[string]interface{} `json:"snippet,omitempty" bson:"snippet"`
	CreatedAt     time.Time              `json:"createdAt" bson:"createdAt"`
}

// PluginHash The plugin hash tuple..
type PluginHash struct {
	Name string `json:"name,omitempty" bson:"name"`
	Hash string `json:"hash,omitempty" bson:"hash"`
}

// Pagination holds information for requesting page.
type Pagination struct {
	Start string
	Size  int
}

// NextPage represents a pagination header value.
type NextPage struct {
	Name   string `json:"name"`
	NextID string `json:"nextId"`
}
