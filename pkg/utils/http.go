package utils

import (
	"encoding/json"
	"fmt"

	"github.com/go-resty/resty/v2"
)

// HandleJSONResponse handles JSON HTTP responses
func HandleJSONResponse(resp *resty.Response, target interface{}, errorMsg string) error {
	if !resp.IsSuccess() {
		return fmt.Errorf("%s: unexpected status %d: %s", errorMsg, resp.StatusCode(), resp.String())
	}
	if err := json.Unmarshal(resp.Body(), target); err != nil {
		return fmt.Errorf("%s: invalid response: %w", errorMsg, err)
	}
	return nil
}

// HandleJSONResponseWithClient uses resty client directly to get JSON response
func HandleJSONResponseWithClient(client *resty.Client, url string, target interface{}, errorMsg string) error {
	resp, err := client.R().Get(url)
	if err != nil {
		return fmt.Errorf("%s: request failed: %w", errorMsg, err)
	}
	return HandleJSONResponse(resp, target, errorMsg)
}

// WrapError wraps an error with a consistent message format
func WrapError(err error, operation string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("unable to %s: %w", operation, err)
}
