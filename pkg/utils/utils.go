package utils

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// FileExists checks if a file exists
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ResolveURL resolves a relative URL against a base URL
func ResolveURL(base, href string) string {
	if href == "" {
		return ""
	}
	if strings.HasPrefix(href, "//") {
		return "https:" + href
	}
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}
	if base == "" {
		return href
	}
	baseURL, err := url.Parse(base)
	if err != nil {
		return href
	}
	ref, err := url.Parse(href)
	if err != nil {
		return href
	}
	return baseURL.ResolveReference(ref).String()
}

// IsAbsoluteURL checks if a URL is absolute
func IsAbsoluteURL(raw string) bool {
	if raw == "" {
		return false
	}
	u, err := url.Parse(raw)
	return err == nil && u.Scheme != "" && u.Host != ""
}

// FilenameFromURL extracts a filename from a URL
func FilenameFromURL(raw string) string {
	if raw == "" {
		return ""
	}
	if parsed, err := url.Parse(raw); err == nil {
		if name := path.Base(parsed.Path); name != "" && name != "." && name != "/" {
			return name
		}
	}
	name := path.Base(StripQueryFragment(raw))
	name = strings.Trim(name, "/")
	if name == "" {
		return ""
	}
	return name
}

// StripQueryFragment removes query parameters and fragments from a URL
func StripQueryFragment(link string) string {
	if idx := strings.IndexAny(link, "?#"); idx >= 0 {
		return link[:idx]
	}
	return link
}

// BaseName extracts the base name from a path
func BaseName(link string) string {
	clean := StripQueryFragment(link)
	name := path.Base(clean)
	if name == "." || name == "/" {
		return ""
	}
	return name
}

// EscapeDirname replaces invalid characters in directory names
func EscapeDirname(name string) string {
	replacer := strings.NewReplacer(
		"~", "_", "#", "_", "%", "_", "&", "_", "*", "_",
		"{", "_", "}", "_", "\\", "_", "<", "_", ">", "_",
		"?", "_", "/", "_", "`", "_", "'", "_", `"`, "_",
		"|", "_", "+", "_", ":", "_",
	)
	if idx := strings.Index(name, ":"); idx > 15 {
		name = name[:idx]
	}
	return replacer.Replace(name)
}

// ZipDirectory creates a zip file from a directory
func ZipDirectory(srcDir, destZip string) error {
	out, err := os.Create(destZip)
	if err != nil {
		return err
	}
	defer out.Close()

	zipWriter := zip.NewWriter(out)
	defer zipWriter.Close()

	return filepath.WalkDir(srcDir, func(pathname string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if pathname == srcDir {
			return nil
		}

		rel, err := filepath.Rel(srcDir, pathname)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		if d.IsDir() {
			_, err := zipWriter.Create(rel + "/")
			return err
		}

		file, err := os.Open(pathname)
		if err != nil {
			return err
		}
		defer file.Close()

		writer, err := zipWriter.Create(rel)
		if err != nil {
			return err
		}

		if _, err := io.Copy(writer, file); err != nil {
			return err
		}

		return nil
	})
}

// J2TeamCookie represents a cookie in J2Team Cookies format
type J2TeamCookie struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Domain   string `json:"domain"`
	Path     string `json:"path"`
	Secure   bool   `json:"secure"`
	HttpOnly bool   `json:"httpOnly"`
	SameSite string `json:"sameSite"`
}

// J2TeamCookiesFile represents the J2Team Cookies export format
type J2TeamCookiesFile struct {
	URL     string         `json:"url"`
	Cookies []J2TeamCookie `json:"cookies"`
}

// BrowserCookie represents a cookie in browser extension export format
type BrowserCookie struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path"`
	Secure   bool    `json:"secure"`
	HttpOnly bool    `json:"httpOnly"`
	SameSite string  `json:"sameSite"`
	HostOnly bool    `json:"hostOnly"`
	Session  bool    `json:"session"`
	StoreID  *string `json:"storeId"`
}

// LoadCookies loads cookies from a JSON file and auto-detects the format
// Supports Cookie-Editor format (flat JSON), J2Team Cookies format, and browser extension export format
func LoadCookies(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Try J2Team format first
	var j2team J2TeamCookiesFile
	if err := json.Unmarshal(data, &j2team); err == nil && len(j2team.Cookies) > 0 {
		// Convert J2Team format to simple map
		cookies := make(map[string]string, len(j2team.Cookies))
		for _, cookie := range j2team.Cookies {
			cookies[cookie.Name] = cookie.Value
		}
		return cookies, nil
	}

	// Try browser extension export format (array of cookie objects)
	var browserCookies []BrowserCookie
	if err := json.Unmarshal(data, &browserCookies); err == nil && len(browserCookies) > 0 {
		// Convert browser format to simple map
		cookies := make(map[string]string, len(browserCookies))
		for _, cookie := range browserCookies {
			cookies[cookie.Name] = cookie.Value
		}
		return cookies, nil
	}

	// Fall back to Cookie-Editor format (flat JSON map)
	var cookies map[string]string
	if err := json.Unmarshal(data, &cookies); err != nil {
		return nil, errors.New("unsupported cookie format: unable to parse as J2Team, browser extension, or Cookie-Editor format")
	}

	if len(cookies) == 0 {
		return nil, errors.New("cookies file is empty")
	}

	return cookies, nil
}
