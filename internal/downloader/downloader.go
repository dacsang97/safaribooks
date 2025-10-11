package downloader

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/dacsang97/safaribooks/internal/html"
	safarihttp "github.com/dacsang97/safaribooks/internal/http"
	"github.com/dacsang97/safaribooks/internal/models"
	"github.com/dacsang97/safaribooks/pkg/utils"
)

const (
	defaultCookiesFile = "cookies.json"
	defaultBooksDir    = "Books"
	maxWorkers         = 5 // Simple concurrency limit
)

type Downloader struct {
	bookID      string
	cookiesPath string
	booksDir    string
	kindleMode  bool
	siteURL     string
	client      *safarihttp.Client
}

func NewDownloader(bookID, cookiesPath, booksDir string, kindleMode bool, siteURL string) (*Downloader, error) {
	if cookiesPath == "" {
		cookiesPath = defaultCookiesFile
	}
	if booksDir == "" {
		booksDir = defaultBooksDir
	}

	if err := os.MkdirAll(booksDir, 0755); err != nil {
		return nil, fmt.Errorf("create books directory: %w", err)
	}

	client, err := safarihttp.NewClient(cookiesPath, siteURL)
	if err != nil {
		return nil, fmt.Errorf("create HTTP client: %w", err)
	}

	return &Downloader{
		bookID:      bookID,
		cookiesPath: cookiesPath,
		booksDir:    booksDir,
		kindleMode:  kindleMode,
		siteURL:     siteURL,
		client:      client,
	}, nil
}

func (d *Downloader) Run() error {
	fmt.Printf("[*] Retrieving book info...\n")
	bookInfo, err := d.client.GetBookInfo(d.bookID)
	if err != nil {
		return err
	}

	fmt.Printf("[*] Retrieving book chapters...\n")
	chapters, err := d.client.GetBookChapters(d.bookID)
	if err != nil {
		return err
	}

	bookPath, err := d.createBookDirectory(bookInfo)
	if err != nil {
		return err
	}

	fmt.Printf("[*] Downloading %d chapters...\n", len(chapters))
	if err := d.downloadChapters(bookPath, chapters); err != nil {
		return err
	}

	fmt.Printf("[*] Creating EPUB file...\n")
	if err := d.generateEPUB(bookInfo, chapters, bookPath); err != nil {
		return err
	}

	epubPath := filepath.Join(bookPath, filepath.Base(bookPath)+".epub")
	fmt.Printf("[*] Done: %s\n", epubPath)
	return nil
}

func (d *Downloader) createBookDirectory(bookInfo models.BookInfo) (string, error) {
	title := utils.EscapeDirname(bookInfo.Title)
	if title == "" {
		title = d.bookID
	}

	cleanTitle := strings.Split(title, ",")[0]
	dirName := fmt.Sprintf("%s (%s)", cleanTitle, d.bookID)
	bookPath := filepath.Join(d.booksDir, dirName)

	dirs := []string{
		bookPath,
		filepath.Join(bookPath, "OEBPS"),
		filepath.Join(bookPath, "OEBPS", "Styles"),
		filepath.Join(bookPath, "OEBPS", "Images"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", fmt.Errorf("create directory %s: %w", dir, err)
		}
	}

	return bookPath, nil
}

func (d *Downloader) downloadChapters(bookPath string, chapters []models.Chapter) error {
	oebpsPath := filepath.Join(bookPath, "OEBPS")

	// Use simple worker pool for concurrency
	sem := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstError error

	for idx := range chapters {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}        // Acquire
			defer func() { <-sem }() // Release

			// Create parser per goroutine to avoid race conditions
			parser := html.NewParser("https://"+d.siteURL, d.kindleMode)

			if err := d.downloadChapter(oebpsPath, &chapters[i], i == 0, parser, bookPath); err != nil {
				mu.Lock()
				if firstError == nil {
					firstError = err
				}
				mu.Unlock()
				fmt.Printf("[-] Failed chapter %s: %v\n", chapters[i].Title, err)
			}
		}(idx)
	}

	wg.Wait()
	return firstError
}

func (d *Downloader) downloadChapter(oebpsPath string, chapter *models.Chapter, isFirst bool, parser *html.Parser, bookPath string) error {
	// Download chapter content
	resp, err := d.client.Get(chapter.Content)
	if err != nil {
		return fmt.Errorf("download chapter: %w", err)
	}
	if !resp.IsSuccess() {
		return fmt.Errorf("status %d for chapter %s", resp.StatusCode(), chapter.Title)
	}

	chapter.Content = string(resp.Body())

	// Parse chapter HTML
	_, pageHTML, err := parser.ParseChapter(*chapter, isFirst)
	if err != nil {
		return fmt.Errorf("parse chapter: %w", err)
	}

	// Save chapter file
	filename := strings.ReplaceAll(chapter.Filename, ".html", ".xhtml")
	chapter.Filename = filename
	outputPath := filepath.Join(oebpsPath, filename)
	if err := os.WriteFile(outputPath, []byte(pageHTML), 0644); err != nil {
		return fmt.Errorf("write chapter: %w", err)
	}

	// Download chapter assets (CSS/images)
	d.downloadAssets(chapter, bookPath)
	return nil
}

func (d *Downloader) downloadAssets(chapter *models.Chapter, basePath string) {
	imagesPath := filepath.Join(basePath, "OEBPS", "Images")

	if len(chapter.Images) > 0 {
		fmt.Printf("[*] Chapter '%s' has %d images\n", chapter.Title, len(chapter.Images))
	}

	// Download images
	for _, imgURL := range chapter.Images {
		url := d.resolveImageURL(chapter, imgURL)
		if url == "" {
			fmt.Printf("[-] Skipping empty image URL from: %s\n", imgURL)
			continue
		}
		filename := utils.FilenameFromURL(url)
		if filename == "" {
			fmt.Printf("[-] Could not get filename from URL: %s\n", url)
			continue
		}
		fmt.Printf("[*] Downloading image: %s -> %s\n", url, filename)
		d.downloadFile(url, filepath.Join(imagesPath, filename))
	}
}

func (d *Downloader) downloadFile(url, path string) {
	if utils.FileExists(path) {
		fmt.Printf("[+] Image already exists: %s\n", filepath.Base(path))
		return
	}

	resp, err := d.client.Get(url)
	if err != nil {
		fmt.Printf("[-] Failed to download %s: %v\n", url, err)
		return
	}
	if !resp.IsSuccess() {
		fmt.Printf("[-] Failed to download %s: status %d\n", url, resp.StatusCode())
		return
	}

	if err := os.WriteFile(path, resp.Body(), 0644); err != nil {
		fmt.Printf("[-] Failed to save %s: %v\n", filepath.Base(path), err)
		return
	}
	fmt.Printf("[+] Downloaded image: %s\n", filepath.Base(path))
}

func (d *Downloader) resolveImageURL(chapter *models.Chapter, img string) string {
	chapterBase := chapter.AssetBaseURL
	apiV2 := strings.Contains(chapter.Content, "/api/v2/")

	if apiV2 {
		chapterBase = fmt.Sprintf("https://%s/api/v2/epubs/urn:orm:book:%s/files", d.siteURL, d.bookID)
		return strings.TrimSuffix(chapterBase, "/") + "/" + strings.TrimPrefix(img, "/")
	}

	return utils.ResolveURL(chapter.AssetBaseURL, img)
}

func (d *Downloader) generateEPUB(bookInfo models.BookInfo, chapters []models.Chapter, bookPath string) error {
	oebpsPath := filepath.Join(bookPath, "OEBPS")
	imagesPath := filepath.Join(oebpsPath, "Images")

	// Download cover image - try to get the largest version
	var coverFilename string
	if bookInfo.Cover != "" {
		coverFilename = d.downloadLargestCover(bookInfo.Cover, imagesPath)
	} else {
		fmt.Printf("[-] No cover URL in book info, checking chapters...\n")
		// Try to find cover in first few chapters
		coverFilename = d.findCoverInChapters(chapters, imagesPath)
	}

	// Create cover page (cover.xhtml)
	if coverFilename != "" {
		coverPage := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml">
<head>
<title>Cover</title>
<style type="text/css">
img { max-width: 100%%; height: auto; }
</style>
</head>
<body>
<div style="text-align:center;">
<img src="Images/%s" alt="Cover"/>
</div>
</body>
</html>`, coverFilename)
		os.WriteFile(filepath.Join(oebpsPath, "cover.xhtml"), []byte(coverPage), 0644)
	}

	// Create mimetype
	os.WriteFile(filepath.Join(bookPath, "mimetype"), []byte("application/epub+zip"), 0644)

	// Create META-INF/container.xml
	metaInf := filepath.Join(bookPath, "META-INF")
	os.MkdirAll(metaInf, 0755)
	containerXML := `<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
<rootfiles>
<rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml" />
</rootfiles>
</container>`
	os.WriteFile(filepath.Join(metaInf, "container.xml"), []byte(containerXML), 0644)

	// Create content.opf and toc.ncx
	if err := d.writeEPUBMetadata(bookInfo, chapters, oebpsPath, coverFilename); err != nil {
		return err
	}

	// Zip to EPUB
	zipPath := bookPath + ".zip"
	if err := utils.ZipDirectory(bookPath, zipPath); err != nil {
		return fmt.Errorf("create zip: %w", err)
	}

	epubPath := filepath.Join(bookPath, filepath.Base(bookPath)+".epub")
	return os.Rename(zipPath, epubPath)
}

func (d *Downloader) writeEPUBMetadata(bookInfo models.BookInfo, chapters []models.Chapter, oebpsPath string, coverFilename string) error {
	// Print metadata info
	fmt.Printf("[*] Book: %s\n", bookInfo.Title)
	if len(bookInfo.Authors) > 0 {
		fmt.Printf("[*] Authors: ")
		for i, author := range bookInfo.Authors {
			if i > 0 {
				fmt.Printf(", ")
			}
			fmt.Printf("%s", author.Name)
		}
		fmt.Printf("\n")
	} else {
		fmt.Printf("[*] Authors: Unknown (no author data from API)\n")
	}
	if len(bookInfo.Publishers) > 0 {
		fmt.Printf("[*] Publisher: %s\n", bookInfo.Publishers[0].Name)
	}

	// Build chapter manifest and spine
	manifest := ""
	spine := ""

	// Add cover page first if we have a cover
	if coverFilename != "" {
		manifest += `<item id="cover" href="cover.xhtml" media-type="application/xhtml+xml" />
`
		spine += `<itemref idref="cover"/>
`
	}

	for i, ch := range chapters {
		id := fmt.Sprintf("ch%d", i)
		manifest += fmt.Sprintf(`<item id="%s" href="%s" media-type="application/xhtml+xml" />
`, id, ch.Filename)
		spine += fmt.Sprintf(`<itemref idref="%s"/>
`, id)
	}

	// Add images to manifest
	imagesPath := filepath.Join(oebpsPath, "Images")
	hasCover := false
	if entries, err := os.ReadDir(imagesPath); err == nil {
		for idx, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			ext := strings.ToLower(filepath.Ext(name))
			mediaType := getImageMediaType(ext)

			// Mark cover image specially
			if name == coverFilename {
				manifest += fmt.Sprintf(`<item id="cover-image" href="Images/%s" media-type="%s" />
`, name, mediaType)
				hasCover = true
			} else {
				manifest += fmt.Sprintf(`<item id="img%d" href="Images/%s" media-type="%s" />
`, idx, name, mediaType)
			}
		}
	}

	// Build authors metadata
	authors := ""
	for _, author := range bookInfo.Authors {
		authors += fmt.Sprintf(`<dc:creator>%s</dc:creator>
`, escapeXML(author.Name))
	}
	if authors == "" {
		authors = `<dc:creator>Unknown</dc:creator>
`
	}

	// Build publishers
	publishers := ""
	for _, pub := range bookInfo.Publishers {
		if pub.Name != "" {
			publishers += escapeXML(pub.Name)
			break
		}
	}
	if publishers == "" {
		publishers = "Unknown"
	}

	// Build description
	description := escapeXML(bookInfo.Description)
	if description == "" {
		description = "No description available"
	}

	// Add cover metadata if we have a cover
	coverMeta := ""
	if hasCover {
		coverMeta = `<meta name="cover" content="cover-image"/>
`
	}

	contentOPF := fmt.Sprintf(`<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="2.0" unique-identifier="bookid">
<metadata xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:opf="http://www.idpf.org/2007/opf">
<dc:title>%s</dc:title>
%s<dc:publisher>%s</dc:publisher>
<dc:description>%s</dc:description>
<dc:language>en</dc:language>
<dc:identifier id="bookid">%s</dc:identifier>
<dc:date>%s</dc:date>
%s</metadata>
<manifest>
<item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml" />
%s</manifest>
<spine toc="ncx">%s</spine>
</package>`, escapeXML(bookInfo.Title), authors, publishers, description,
		firstNonEmpty(bookInfo.ISBN, bookInfo.Identifier, d.bookID),
		escapeXML(bookInfo.Issued), coverMeta, manifest, spine)

	// Build authors for TOC
	tocAuthors := ""
	for _, author := range bookInfo.Authors {
		tocAuthors += escapeXML(author.Name) + ", "
	}
	tocAuthors = strings.TrimSuffix(tocAuthors, ", ")
	if tocAuthors == "" {
		tocAuthors = "Unknown"
	}

	// Simple toc.ncx
	navMap := ""
	for i, ch := range chapters {
		navMap += fmt.Sprintf(`<navPoint id="ch%d" playOrder="%d">
<navLabel><text>%s</text></navLabel>
<content src="%s"/>
</navPoint>
`, i, i+1, escapeXML(ch.Title), ch.Filename)
	}

	tocNCX := fmt.Sprintf(`<?xml version="1.0"?>
<!DOCTYPE ncx PUBLIC "-//NISO//DTD ncx 2005-1//EN" "http://www.daisy.org/z3986/2005/ncx-2005-1.dtd">
<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1">
<head>
<meta name="dtb:uid" content="%s"/>
</head>
<docTitle><text>%s</text></docTitle>
<docAuthor><text>%s</text></docAuthor>
<navMap>
%s</navMap>
</ncx>`, firstNonEmpty(bookInfo.ISBN, d.bookID), escapeXML(bookInfo.Title), tocAuthors, navMap)

	os.WriteFile(filepath.Join(oebpsPath, "content.opf"), []byte(contentOPF), 0644)
	os.WriteFile(filepath.Join(oebpsPath, "toc.ncx"), []byte(tocNCX), 0644)
	return nil
}

func getImageMediaType(ext string) string {
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".webp":
		return "image/webp"
	default:
		return "image/jpeg"
	}
}

func (d *Downloader) findCoverInChapters(chapters []models.Chapter, imagesPath string) string {
	// Look for cover in first 3 chapters
	for i := 0; i < len(chapters) && i < 3; i++ {
		ch := &chapters[i]
		if strings.Contains(strings.ToLower(ch.Title), "cover") ||
			strings.Contains(strings.ToLower(ch.Filename), "cover") {
			fmt.Printf("[*] Found cover chapter: %s\n", ch.Title)

			// Download chapter content to get images
			resp, err := d.client.Get(ch.Content)
			if err == nil && resp.IsSuccess() {
				ch.Content = string(resp.Body())

				// If chapter has multiple images, find the largest
				if len(ch.Images) > 0 {
					fmt.Printf("[*] Cover chapter has %d images, finding largest...\n", len(ch.Images))
					return d.findLargestImageFromList(ch, ch.Images, imagesPath)
				}
			}
		}
	}
	return ""
}

func (d *Downloader) findLargestImageFromList(chapter *models.Chapter, imageURLs []string, imagesPath string) string {
	// Try first image with 600w variant
	for _, imgURL := range imageURLs {
		url := d.resolveImageURL(chapter, imgURL)
		if url == "" {
			continue
		}

		// Try to download 600w variant
		variants := d.generateCoverURLVariants(url)
		for _, variantURL := range variants {
			resp, err := d.client.Get(variantURL)
			if err != nil || !resp.IsSuccess() {
				continue
			}

			data := resp.Body()
			size := len(data)

			// Save first successful download
			ext := ".jpg"
			if strings.Contains(variantURL, ".png") {
				ext = ".png"
			}

			coverFilename := "cover" + ext
			coverFile := filepath.Join(imagesPath, coverFilename)
			if err := os.WriteFile(coverFile, data, 0644); err != nil {
				continue
			}

			fmt.Printf("[+] Saved cover (%d KB): %s\n", size/1024, coverFilename)
			return coverFilename
		}
	}

	return ""
}

func (d *Downloader) generateCoverURLVariants(coverURL string) []string {
	// Always use 600w - best quality/size balance
	sizeVariants := []string{
		"1200w", "800w", "600w", "500w", "400w", "200w",
		"large", "medium", "small", "thumb",
	}

	// Try replacing any existing size with 600w
	for _, oldSize := range sizeVariants {
		if strings.Contains(coverURL, oldSize) {
			url600w := strings.ReplaceAll(coverURL, oldSize, "600w")
			return []string{url600w, coverURL} // Try 600w first, then original
		}
	}

	// If no size found in URL, try appending /600w/
	baseURL := strings.TrimSuffix(coverURL, "/")
	return []string{
		baseURL + "/600w/",
		coverURL,
	}
}

func (d *Downloader) downloadLargestCover(coverURL, imagesPath string) string {
	fmt.Printf("[*] Original cover URL: %s\n", coverURL)

	// Generate possible cover URLs (prefer 600w)
	possibleURLs := d.generateCoverURLVariants(coverURL)

	// Try downloading in order (600w first)
	for _, url := range possibleURLs {
		resp, err := d.client.Get(url)
		if err != nil || !resp.IsSuccess() {
			continue
		}

		data := resp.Body()
		size := len(data)

		// Detect image type
		ext := ".jpg"
		if strings.Contains(url, ".png") {
			ext = ".png"
		}

		coverFilename := "cover" + ext
		coverFile := filepath.Join(imagesPath, coverFilename)
		if err := os.WriteFile(coverFile, data, 0644); err != nil {
			fmt.Printf("[-] Failed to save cover: %v\n", err)
			continue
		}

		fmt.Printf("[+] Saved cover (%d KB): %s\n", size/1024, coverFilename)
		return coverFilename
	}

	fmt.Printf("[-] Failed to download cover from any variant\n")
	return ""
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}

func firstNonEmpty(strs ...string) string {
	for _, s := range strs {
		if s != "" {
			return s
		}
	}
	return ""
}
