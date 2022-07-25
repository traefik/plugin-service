package db

import "time"

// Plugin The plugin information.
type Plugin struct {
	ID            string                 `bson:"id"`
	Name          string                 `bson:"name"`
	DisplayName   string                 `bson:"displayName"`
	Author        string                 `bson:"author"`
	Type          string                 `bson:"type"`
	Compatibility string                 `bson:"compatibility"`
	Summary       string                 `bson:"summary"`
	IconURL       string                 `bson:"iconUrl"`
	BannerURL     string                 `bson:"bannerUrl"`
	Readme        string                 `bson:"readme"`
	LatestVersion string                 `bson:"latestVersion"`
	Versions      []PluginVersion        `bson:"versions"`
	Stars         int                    `bson:"stars"`
	Snippet       map[string]interface{} `bson:"snippet"`
	CreatedAt     time.Time              `bson:"createdAt"`
}

// PluginVersion is a plugin version with the correct import.
type PluginVersion struct {
	Name   string `json:"name" bson:"name"`
	Import string `json:"import" bson:"import"`
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
