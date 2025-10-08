package models

import "encoding/json"

// BookInfo represents the book information from the API
type BookInfo struct {
	Title       string        `json:"title"`
	Description string        `json:"description"`
	WebURL      string        `json:"web_url"`
	Identifier  string        `json:"identifier"`
	ISBN        string        `json:"isbn"`
	Issued      string        `json:"issued"`
	Rights      string        `json:"rights"`
	Cover       string        `json:"cover"`
	Authors     []namedEntity `json:"authors"`
	Publishers  []namedEntity `json:"publishers"`
	Subjects    []namedEntity `json:"subjects"`
}

// namedEntity represents a simple name entity
type namedEntity struct {
	Name string `json:"name"`
}

// Chapter represents a book chapter
type Chapter struct {
	Title        string              `json:"title"`
	Filename     string              `json:"filename"`
	Content      string              `json:"content"`
	AssetBaseURL string              `json:"asset_base_url"`
	Images       []string            `json:"images"`
	Stylesheets  []ChapterStylesheet `json:"stylesheets"`
	SiteStyles   []string            `json:"site_styles"`
	Fragment     string              `json:"fragment"`
	ID           string              `json:"id"`
	Depth        json.Number         `json:"depth"`
	Children     []Chapter           `json:"children"`
}

// ChapterStylesheet represents a chapter stylesheet
type ChapterStylesheet struct {
	URL string `json:"url"`
}

// ChapterResponse represents the API response for chapters
type ChapterResponse struct {
	Count   int       `json:"count"`
	Next    *string   `json:"next"`
	Results []Chapter `json:"results"`
}

// TocItem represents a table of contents item
type TocItem struct {
	Fragment string      `json:"fragment"`
	Href     string      `json:"href"`
	Label    string      `json:"label"`
	Depth    json.Number `json:"depth"`
	ID       string      `json:"id"`
	Children []TocItem   `json:"children"`
}
