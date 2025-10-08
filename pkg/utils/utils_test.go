package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCookies_CookieEditorFormat(t *testing.T) {
	// Create a temporary file with Cookie-Editor format
	tmpDir := t.TempDir()
	cookiePath := filepath.Join(tmpDir, "cookies.json")

	cookieEditorJSON := `{
		"_abck": "test_value_1",
		"orm-jwt": "test_value_2",
		"orm-rt": "test_value_3"
	}`

	if err := os.WriteFile(cookiePath, []byte(cookieEditorJSON), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	cookies, err := LoadCookies(cookiePath)
	if err != nil {
		t.Fatalf("LoadCookies failed: %v", err)
	}

	if len(cookies) != 3 {
		t.Errorf("Expected 3 cookies, got %d", len(cookies))
	}

	if cookies["_abck"] != "test_value_1" {
		t.Errorf("Expected _abck=test_value_1, got %s", cookies["_abck"])
	}

	if cookies["orm-jwt"] != "test_value_2" {
		t.Errorf("Expected orm-jwt=test_value_2, got %s", cookies["orm-jwt"])
	}
}

func TestLoadCookies_J2TeamFormat(t *testing.T) {
	// Create a temporary file with J2Team format
	tmpDir := t.TempDir()
	cookiePath := filepath.Join(tmpDir, "j2team_cookies.json")

	j2teamJSON := `{
		"url": "https://learning.oreilly.com",
		"cookies": [
			{
				"name": "_abck",
				"value": "test_value_1",
				"domain": "learning.oreilly.com",
				"path": "/",
				"secure": true,
				"httpOnly": true,
				"sameSite": "no_restriction"
			},
			{
				"name": "orm-jwt",
				"value": "test_value_2",
				"domain": ".oreilly.com",
				"path": "/",
				"secure": true,
				"httpOnly": false,
				"sameSite": "lax"
			}
		]
	}`

	if err := os.WriteFile(cookiePath, []byte(j2teamJSON), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	cookies, err := LoadCookies(cookiePath)
	if err != nil {
		t.Fatalf("LoadCookies failed: %v", err)
	}

	if len(cookies) != 2 {
		t.Errorf("Expected 2 cookies, got %d", len(cookies))
	}

	if cookies["_abck"] != "test_value_1" {
		t.Errorf("Expected _abck=test_value_1, got %s", cookies["_abck"])
	}

	if cookies["orm-jwt"] != "test_value_2" {
		t.Errorf("Expected orm-jwt=test_value_2, got %s", cookies["orm-jwt"])
	}
}

func TestLoadCookies_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	cookiePath := filepath.Join(tmpDir, "empty.json")

	if err := os.WriteFile(cookiePath, []byte("{}"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	_, err := LoadCookies(cookiePath)
	if err == nil {
		t.Error("Expected error for empty cookies file, got nil")
	}
}

func TestLoadCookies_InvalidFormat(t *testing.T) {
	tmpDir := t.TempDir()
	cookiePath := filepath.Join(tmpDir, "invalid.json")

	if err := os.WriteFile(cookiePath, []byte("invalid json"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	_, err := LoadCookies(cookiePath)
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}
