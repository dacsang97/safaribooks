# Safari Books Downloader

[![CI](https://github.com/dacsang97/safaribooks/actions/workflows/ci.yml/badge.svg)](https://github.com/dacsang97/safaribooks/actions/workflows/ci.yml)

A simple CLI tool to download and generate EPUB files from Safari Books Online.

## Features

- Download books from Safari Books Online by ID
- Generate properly formatted EPUB files
- Simple command-line interface
- Support for Kindle-specific CSS tweaks
- Auto-detect and support multiple cookie formats (Cookie-Editor, J2Team Cookies)

## Installation

```bash
go build -o safaribooks ./cmd/main.go
```

## Exporting Cookies

Before you can download books, you need to export your cookies from Safari Books Online. The tool supports two popular cookie export formats:

### Option 1: Cookie-Editor (Recommended)

1. Install [Cookie-Editor](https://cookie-editor.com/) browser extension
2. Visit [Safari Books Online](https://learning.oreilly.com) and log in
3. Click the Cookie-Editor extension icon
4. Click "Export" → "Export as JSON"
5. Save the file as `cookies.json`

### Option 2: J2TEAM Cookies

1. Install [J2TEAM Cookies](https://chromewebstore.google.com/detail/J2TEAM%20Cookies/okpidcojinmlaakglciglbpcpajaibco) Chrome extension
2. Visit [Safari Books Online](https://learning.oreilly.com) and log in
3. Click the J2TEAM Cookies extension icon
4. Click "Export"
5. Save the file (the tool will auto-detect this format)

**Note:** The CLI automatically detects which format you're using, so you don't need to specify the format.

## Usage

1. Export your cookies using one of the methods above
2. Run the download command:

```bash
./safaribooks download <book-id>
```

### Options

- `--cookies, -c`: Path to cookies file - supports Cookie-Editor and J2Team formats (default: "cookies.json")
- `--output, -o`: Base directory where the Books folder will be created (default: "Books")
- `--kindle`: Enable Kindle-specific CSS tweaks

### Examples

```bash
# Download a book with default settings (uses cookies.json)
./safaribooks download 1234567890

# Download with Cookie-Editor format
./safaribooks download 1234567890 --cookies cookies.json

# Download with J2Team Cookies format
./safaribooks download 1234567890 --cookies learning.oreilly.com.json

# Download with custom output directory
./safaribooks download 1234567890 --output /path/to/output

# Download with Kindle optimizations
./safaribooks download 1234567890 --kindle
```

## Project Structure

```
safaribooks/
├── cmd/
│   └── main.go              # CLI entry point
├── internal/
│   ├── downloader/          # Core download logic
│   ├── epub/                # EPUB generation
│   ├── html/                # HTML processing
│   ├── http/                # HTTP client
│   └── models/              # Data structures
└── pkg/
    └── utils/               # Common utilities
```

## Architecture

The project is organized into several packages with clear responsibilities:

- **cmd**: Command-line interface
- **internal/downloader**: Orchestrates the download process
- **internal/epub**: Generates EPUB files
- **internal/html**: Processes and transforms HTML content
- **internal/http**: Handles HTTP communication with Safari Books API
- **internal/models**: Defines data structures
- **pkg/utils**: Common utility functions

## Dependencies

- [cli/v2](https://github.com/urfave/cli/v2) - Command-line interface
- [goquery](https://github.com/PuerkitoBio/goquery) - HTML parsing and manipulation
- [net/html](https://golang.org/x/net/html) - HTML parsing

## License

This project is for educational purposes only. Please respect the terms of service of Safari Books Online.
