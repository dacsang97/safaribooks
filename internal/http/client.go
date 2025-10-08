package http

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/dacsang97/safaribooks/internal/models"
	"github.com/dacsang97/safaribooks/pkg/utils"
	"github.com/go-resty/resty/v2"
	"github.com/samber/lo"
)

const (
	safariBaseURL    = "https://learning.oreilly.com"
	profileURL       = safariBaseURL + "/profile/"
	defaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 " +
		"(KHTML, like Gecko) Chrome/90.0.4430.212 Safari/537.36"
)

// Client represents an HTTP client for Safari Books API
type Client struct {
	client *resty.Client
}

// NewClient creates a new HTTP client with authentication
func NewClient(cookiesPath string) (*Client, error) {
	cookies, err := utils.LoadCookies(cookiesPath)
	if err != nil {
		return nil, utils.WrapError(err, "load cookies")
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, utils.WrapError(err, "create cookie jar")
	}

	// Create resty client
	client := resty.New().
		SetTimeout(60 * time.Second).
		SetRedirectPolicy(resty.FlexibleRedirectPolicy(10))

	// Set cookies
	base, _ := url.Parse(safariBaseURL)
	var cookieSet []*http.Cookie
	for name, value := range cookies {
		cookieSet = append(cookieSet, &http.Cookie{
			Name:   name,
			Value:  value,
			Path:   "/",
			Domain: base.Host,
		})
	}

	// Resty doesn't have SetJar, we need to set cookies manually
	client.SetCookies(cookieSet)

	// Store the jar for potential future use
	_ = jar // Keep the jar to avoid unused variable warning

	// Set default headers
	client.SetHeaders(map[string]string{
		"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8",
		"Referer":                   safariBaseURL + "/login/unified/?next=/home/",
		"Upgrade-Insecure-Requests": "1",
		"User-Agent":                defaultUserAgent,
	})

	// Check authentication
	if err := ensureAuthenticated(client); err != nil {
		return nil, err
	}

	return &Client{
		client: client,
	}, nil
}

// Get performs a GET request
func (c *Client) Get(url string) (*resty.Response, error) {
	return c.client.R().Get(url)
}

// GetBookInfo fetches book information from the API
func (c *Client) GetBookInfo(bookID string) (models.BookInfo, error) {
	apiURL := fmt.Sprintf("%s/api/v1/book/%s/", safariBaseURL, bookID)

	var info models.BookInfo
	if err := utils.HandleJSONResponseWithClient(c.client, apiURL, &info, "API: unable to retrieve book info"); err != nil {
		return models.BookInfo{}, err
	}

	return info, nil
}

// GetBookChapters fetches all chapters for a book
func (c *Client) GetBookChapters(bookID string) ([]models.Chapter, error) {
	apiURL := fmt.Sprintf("%s/api/v1/book/%s/", safariBaseURL, bookID)
	var all []models.Chapter
	pageURL := apiURL + "chapter/?page=1"

	for pageURL != "" {
		var payload models.ChapterResponse
		resp, err := c.client.R().Get(pageURL)
		if err != nil {
			return nil, utils.WrapError(err, "API: retrieve book chapters")
		}

		if err := utils.HandleJSONResponse(resp, &payload, "API: unable to retrieve book chapters"); err != nil {
			return nil, err
		}

		if len(payload.Results) == 0 {
			return nil, errors.New("API: unable to retrieve book chapters")
		}

		// Use samber/lo to filter chapters
		covers := lo.Filter(payload.Results, func(chapter models.Chapter, index int) bool {
			return strings.Contains(strings.ToLower(chapter.Filename), "cover") ||
				strings.Contains(strings.ToLower(chapter.Title), "cover")
		})

		remaining := lo.Filter(payload.Results, func(chapter models.Chapter, index int) bool {
			return !strings.Contains(strings.ToLower(chapter.Filename), "cover") &&
				!strings.Contains(strings.ToLower(chapter.Title), "cover")
		})

		all = append(all, covers...)
		all = append(all, remaining...)

		if payload.Next != nil && *payload.Next != "" {
			pageURL = *payload.Next
		} else {
			pageURL = ""
		}
	}

	return all, nil
}

// ensureAuthenticated checks if the client is authenticated
func ensureAuthenticated(client *resty.Client) error {
	resp, err := client.R().
		SetHeader("User-Agent", defaultUserAgent).
		Get(profileURL)
	if err != nil {
		return utils.WrapError(err, "authentication check failed")
	}

	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("authentication issue: expected 200, got %d", resp.StatusCode())
	}

	if strings.Contains(resp.String(), `user_type":"Expired"`) {
		return errors.New("authentication issue: account subscription expired")
	}

	return nil
}
