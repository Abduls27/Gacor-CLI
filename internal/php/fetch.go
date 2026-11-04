package php

import (
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// FetchAvailableVersions scrapes windows.php.net and returns a map of versions to URLs.
func FetchAvailableVersions() (map[string]string, error) {
	baseURL := "https://windows.php.net/downloads/releases/"

	resp, err := http.Get(baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch: %w", err)
	}
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}

	versions := map[string]string{}
	re := regexp.MustCompile(`php-(\d+)\.(\d+)\.(\d+)(?:-\d+)?-nts-Win32-vs\d+-x64\.zip$`)

	doc.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		matches := re.FindStringSubmatch(href)
		if len(matches) < 4 {
			return
		}
		minor := matches[1] + "." + matches[2] + "." + matches[3]
		url := href
		if !strings.HasPrefix(url, "http") {
			url = baseURL + filepath.Base(href)
		}
		if _, exists := versions[minor]; !exists {
			versions[minor] = url
		}
	})

	return versions, nil
}

// FetchPrereleaseVersions scrapes pre-release (RC/dev) versions from windows.php.net/downloads/qa/.
func FetchPrereleaseVersions() (map[string]string, error) {
	baseURL := "https://windows.php.net/downloads/qa/"

	resp, err := http.Get(baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch: %w", err)
	}
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}

	versions := map[string]string{}
	// Capture the "RC4" or "-dev" suffix before "-nts-Win32-...".
	re := regexp.MustCompile(`php-(\d+\.\d+\.\d+(?:RC\d+|-dev|alpha\d+|beta\d+))-nts-Win32-vs\d+-x64\.zip$`)

	doc.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		matches := re.FindStringSubmatch(href)
		if len(matches) < 2 {
			return
		}
		version := matches[1] // e.g. "8.6.0RC4" or "8.3.34-dev"
		url := href
		if !strings.HasPrefix(url, "http") {
			url = baseURL + filepath.Base(href)
		}
		if _, exists := versions[version]; !exists {
			versions[version] = url
		}
	})

	return versions, nil
}

func SortedVersions(versions map[string]string) []string {
	keys := make([]string, 0, len(versions))
	for k := range versions {
		keys = append(keys, k)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(keys)))
	return keys
}
