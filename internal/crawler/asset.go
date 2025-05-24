package crawler

import (
	"io"
	"log"
	"net/url"
	"strings"

	"monsterinc/internal/models"

	"github.com/PuerkitoBio/goquery"
)

// URLSource is now defined in internal/models/asset.go
// type URLSource struct {
// 	Tag       string
// 	Attribute string
// }

// ExtractedAsset is now defined in internal/models/asset.go
// type ExtractedAsset struct {
// 	AbsoluteURL string
// 	SourceTag   string // e.g., "a"
// 	SourceAttr  string // e.g., "href"
// 	// ContextText string // Optional: text content of the link, or surrounding text
// }

// tagsToExtract defines which HTML tags and attributes to check for URLs.
// Task 3.1: Define tags and attributes for URL extraction.
var tagsToExtract = map[string]string{
	"a":      "href",
	"link":   "href",   // For stylesheets, favicons, etc.
	"script": "src",    // For JavaScript files
	"img":    "src",    // For images
	"iframe": "src",    // For embedded frames
	"form":   "action", // For form submission URLs
	"object": "data",   // For embedded objects
	"embed":  "src",    // For embedded content
	// TODO: Add more tags/attributes if necessary based on common practices or specific needs
	// e.g. <source src="...">, <video poster="...">, <area href="...">
	// srcset for img/source can contain multiple URLs, needs special handling.
}

// ExtractAssetsFromHTML parses an HTML document and extracts all relevant asset URLs.
// It resolves relative URLs against the provided basePageURL.
// Task 3.1: Implement URL extraction from HTML tags.
// Task 3.2 (partially): This function collects asset URLs.
func ExtractAssetsFromHTML(htmlBody io.Reader, basePageURL *url.URL, crawlerInstance *Crawler) ([]models.ExtractedAsset, error) {
	if basePageURL == nil {
		log.Println("[WARN] AssetExtractor: Base page URL is nil, cannot resolve relative URLs effectively.")
	}

	doc, err := goquery.NewDocumentFromReader(htmlBody)
	if err != nil {
		return nil, err
	}

	var extractedAssets []models.ExtractedAsset

	for tag, attribute := range tagsToExtract {
		doc.Find(tag).Each(func(i int, s *goquery.Selection) {
			attrValue, exists := s.Attr(attribute)
			if !exists || strings.TrimSpace(attrValue) == "" {
				return
			}

			urlsInAttr := []string{attrValue}
			if attribute == "srcset" {
				urlsInAttr = parseSrcset(attrValue)
			}

			for _, rawURL := range urlsInAttr {
				trimmedRawURL := strings.TrimSpace(rawURL)
				if trimmedRawURL == "" || strings.HasPrefix(trimmedRawURL, "data:") || strings.HasPrefix(trimmedRawURL, "mailto:") || strings.HasPrefix(trimmedRawURL, "tel:") || strings.HasPrefix(trimmedRawURL, "javascript:") {
					continue
				}

				var absoluteURL string

				if basePageURL != nil {
					resolved, errRes := basePageURL.Parse(trimmedRawURL)
					if errRes != nil {
						// log.Printf("[DEBUG] AssetExtractor: Failed to resolve URL '%s' against base '%s': %v", trimmedRawURL, basePageURL.String(), errRes)
						continue
					}
					absoluteURL = resolved.String()
				} else {
					parsedRaw, errParse := url.Parse(trimmedRawURL)
					if errParse != nil || !parsedRaw.IsAbs() {
						// log.Printf("[DEBUG] AssetExtractor: URL '%s' is relative but no base URL provided, or unparsable. Skipping.", trimmedRawURL)
						continue
					}
					absoluteURL = parsedRaw.String()
				}

				asset := models.ExtractedAsset{
					AbsoluteURL: absoluteURL,
					SourceTag:   tag,
					SourceAttr:  attribute,
				}
				extractedAssets = append(extractedAssets, asset)

				if crawlerInstance != nil {
					crawlerInstance.DiscoverURL(absoluteURL, basePageURL)
				}
			}
		})
	}
	return extractedAssets, nil
}

func parseSrcset(srcset string) []string {
	var urls []string
	parts := strings.Split(srcset, ",")
	for _, part := range parts {
		trimmedPart := strings.TrimSpace(part)
		if trimmedPart == "" {
			continue
		}
		urlAndDescriptor := strings.Fields(trimmedPart)
		if len(urlAndDescriptor) > 0 {
			urls = append(urls, urlAndDescriptor[0])
		}
	}
	return urls
}
