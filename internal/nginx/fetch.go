package nginx

import (
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func FetchAvailableVersions() (map[string]string, error) {
	resp, err := http.Get("https://nginx.org/en/download.html")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch nginx.org: %w", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	versions := map[string]string{}
	re := regexp.MustCompile(`nginx-(\d+\.\d+\.\d+)\.zip$`)

	doc.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		if !strings.HasSuffix(href, ".zip") {
			return
		}
		matches := re.FindStringSubmatch(href)
		if len(matches) < 2 {
			return
		}
		full := matches[1] // "1.8.1"

		url := href
		if !strings.HasPrefix(url, "http") {
			url = "https://nginx.org" + url
		}
		if _, exists := versions[full]; !exists {
			versions[full] = url
		}
	})

	return versions, nil
}

func SortedVersions(versions map[string]string) []string {
	keys := make([]string, 0, len(versions))
	for k := range versions {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return compareVersion(keys[i], keys[j]) > 0
	})
	return keys
}

func compareVersion(a, b string) int {
	partsA := strings.Split(a, ".")
	partsB := strings.Split(b, ".")

	maxLen := len(partsA)
	if len(partsB) > maxLen {
		maxLen = len(partsB)
	}

	for i := 0; i < maxLen; i++ {
		var numA, numB int
		if i < len(partsA) {
			fmt.Sscanf(partsA[i], "%d", &numA)
		}
		if i < len(partsB) {
			fmt.Sscanf(partsB[i], "%d", &numB)
		}
		if numA != numB {
			if numA > numB {
				return 1
			}
			return -1
		}
	}
	return 0
}
