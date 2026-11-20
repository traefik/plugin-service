package db

import "time"

// Plugin The plugin information.
type Plugin struct {
	ID            string         `json:"id,omitempty" bson:"id"`
	Name          string         `json:"name,omitempty" bson:"name"`
	DisplayName   string         `json:"displayName,omitempty" bson:"displayName"`
	Runtime       string         `json:"runtime,omitempty" bson:"runtime"`
	WasmPath      string         `json:"wasmPath,omitempty" bson:"wasmPath"`
	Author        string         `json:"author,omitempty" bson:"author"`
	Type          string         `json:"type,omitempty" bson:"type"`
	Import        string         `json:"import,omitempty" bson:"import"`
	Compatibility string         `json:"compatibility,omitempty" bson:"compatibility"`
	Summary       string         `json:"summary,omitempty" bson:"summary"`
	IconURL       string         `json:"iconUrl,omitempty" bson:"iconUrl"`
	BannerURL     string         `json:"bannerUrl,omitempty" bson:"bannerUrl"`
	Readme        string         `json:"readme,omitempty" bson:"readme"`
	LatestVersion string         `json:"latestVersion,omitempty" bson:"latestVersion"`
	Versions      []string       `json:"versions,omitempty" bson:"versions"`
	Stars         int            `json:"stars,omitempty" bson:"stars"`
	Snippet       map[string]any `json:"snippet,omitempty" bson:"snippet"`
	CreatedAt     time.Time      `json:"createdAt" bson:"createdAt"`
	Disabled      bool           `json:"disabled,omitempty" bson:"disabled"`
	Hidden        bool           `json:"hidden,omitempty" bson:"hidden"`
	UseUnsafe     bool           `json:"useUnsafe,omitempty" bson:"useUnsafe"`
	// TagsETag is the ETag of the last GitHub tags response seen by the scraper.
	// omitempty: an update without the ETag (an older scraper) does not erase it.
	TagsETag string `json:"tagsETag,omitempty" bson:"tagsETag,omitempty"`
}

// PluginState is the part of a plugin the scraper needs to know if a plugin changed.
type PluginState struct {
	ID            string `json:"id" bson:"id"`
	Name          string `json:"name" bson:"name"`
	LatestVersion string `json:"latestVersion" bson:"latestVersion"`
	Stars         int    `json:"stars" bson:"stars"`
	Hidden        bool   `json:"hidden" bson:"hidden"`
	Disabled      bool   `json:"disabled" bson:"disabled"`
	TagsETag      string `json:"tagsETag" bson:"tagsETag"`
}

// PluginPatch holds the plugin fields that can be updated alone.
// A nil field is not updated.
type PluginPatch struct {
	Stars    *int    `json:"stars,omitempty"`
	Hidden   *bool   `json:"hidden,omitempty"`
	TagsETag *string `json:"tagsETag,omitempty"`
}

// IsEmpty returns true when the patch updates nothing.
func (p PluginPatch) IsEmpty() bool {
	return p.Stars == nil && p.Hidden == nil && p.TagsETag == nil
}

// BlacklistEntry represents a repository excluded from the plugin scraping.
// The Repository is the GitHub full name (owner/repo) and is the unique key.
type BlacklistEntry struct {
	Repository string    `json:"repository" bson:"repository"`
	Reason     string    `json:"reason,omitempty" bson:"reason"`
	Author     string    `json:"author,omitempty" bson:"author"`
	CreatedAt  time.Time `json:"createdAt" bson:"createdAt"`
}

// PluginHash The plugin hash tuple.
type PluginHash struct {
	Name     string `json:"name,omitempty" bson:"name"`
	Hash     string `json:"hash,omitempty" bson:"hash"`
	Verified *bool  `json:"verified,omitempty" bson:"verified"`
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
