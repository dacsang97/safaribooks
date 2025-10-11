package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dacsang97/safaribooks/internal/downloader"
	"github.com/urfave/cli/v2"
)

var version = "dev"

func main() {
	app := &cli.App{
		Name:    "safaribooks",
		Usage:   "Download and generate an EPUB of your favorite Safari Books Online titles.",
		Version: version,
		Commands: []*cli.Command{
			{
				Name:      "download",
				Usage:     "Download a book by its numeric identifier (requires cookies).",
				ArgsUsage: "<book-id>",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "cookies",
						Aliases: []string{"c"},
						Usage:   "Path to cookies file (supports Cookie-Editor and J2Team formats).",
						Value:   "cookies.json",
					},
					&cli.StringFlag{
						Name:    "output",
						Aliases: []string{"o"},
						Usage:   "Base directory where the Books folder will be created.",
						Value:   "Books",
					},
					&cli.BoolFlag{
						Name:  "kindle",
						Usage: "Enable Kindle-specific CSS tweaks.",
					},
					&cli.StringFlag{
						Name:    "site-url",
						Aliases: []string{"s"},
						Usage:   "O'Reilly library site URL (e.g., learning-oreilly-com.dclibrary.idm.oclc.org).",
						Value:   "learning.oreilly.com",
					},
				},
				Action: runDownloadAction,
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func runDownloadAction(ctx *cli.Context) error {
	if ctx.Args().Len() != 1 {
		return cli.Exit("book identifier is required", 1)
	}

	bookID := ctx.Args().First()
	if bookID == "" {
		return cli.Exit("book identifier cannot be empty", 1)
	}

	cookiesPath := ctx.String("cookies")
	if cookiesPath == "" {
		cookiesPath = "cookies.json"
	}

	// Check if cookies file exists
	if !filepath.IsAbs(cookiesPath) {
		if wd, err := os.Getwd(); err == nil {
			cookiesPath = filepath.Join(wd, cookiesPath)
		}
	}

	if _, err := os.Stat(cookiesPath); os.IsNotExist(err) {
		return cli.Exit(fmt.Sprintf("cookies file not found at %s", cookiesPath), 1)
	}

	outputDir := ctx.String("output")
	if outputDir == "" {
		outputDir = "Books"
	}

	// Create output directory if it doesn't exist
	if !filepath.IsAbs(outputDir) {
		if wd, err := os.Getwd(); err == nil {
			outputDir = filepath.Join(wd, outputDir)
		}
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return cli.Exit(fmt.Sprintf("unable to create output directory: %v", err), 1)
	}

	kindleMode := ctx.Bool("kindle")
	siteURL := ctx.String("site-url")
	if siteURL == "" {
		siteURL = "learning.oreilly.com"
	}

	// Create downloader
	dl, err := downloader.NewDownloader(bookID, cookiesPath, outputDir, kindleMode, siteURL)
	if err != nil {
		return cli.Exit(fmt.Sprintf("unable to create downloader: %v", err), 1)
	}

	// Run download
	if err := dl.Run(); err != nil {
		return cli.Exit(fmt.Sprintf("download failed: %v", err), 1)
	}

	return nil
}
