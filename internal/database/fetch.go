package database

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

func FetchAvailableVersions(engine Engine) ([]Release, error) {
	driver, err := driverFor(engine)
	if err != nil {
		return nil, err
	}
	return driver.FetchAvailableVersions()
}

func archiveAvailable(archiveURL string) (bool, error) {
	req, err := http.NewRequest(http.MethodHead, archiveURL, nil)
	if err != nil {
		return false, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return false, err
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, fmt.Errorf("server merespons HTTP %d untuk %s", resp.StatusCode, archiveURL)
	}
	return true, nil
}

func fetchText(source string) (string, error) {
	body, err := fetchBytes(source)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func fetchBytes(source string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, source, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "gacor database manager")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("server merespons HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func fetchXML(source string, target any) error {
	body, err := fetchBytes(source)
	if err != nil {
		return err
	}
	if err := xml.Unmarshal(body, target); err != nil {
		return fmt.Errorf("metadata XML tidak valid: %w", err)
	}
	return nil
}

func fetchJSON(source string, target any) error {
	body, err := fetchText(source)
	if err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(body), target); err != nil {
		return fmt.Errorf("metadata JSON tidak valid: %w", err)
	}
	return nil
}

func sortReleases(releases []Release) {
	sort.Slice(releases, func(i, j int) bool {
		return compareVersion(releases[i].Version, releases[j].Version) > 0
	})
}

func latestPerSeries(releases []Release) []Release {
	latest := make(map[string]Release, len(releases))
	for _, release := range releases {
		parts := strings.Split(release.Version, ".")
		if len(parts) < 2 {
			continue
		}
		series := strings.Join(parts[:2], ".")
		current, exists := latest[series]
		if !exists || compareVersion(release.Version, current.Version) > 0 {
			latest[series] = release
		}
	}
	result := make([]Release, 0, len(latest))
	for _, release := range latest {
		result = append(result, release)
	}
	sortReleases(result)
	return result
}

func compareVersion(a, b string) int {
	partA, partB := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(partA) || i < len(partB); i++ {
		var nA, nB int
		if i < len(partA) {
			nA, _ = strconv.Atoi(partA[i])
		}
		if i < len(partB) {
			nB, _ = strconv.Atoi(partB[i])
		}
		if nA > nB {
			return 1
		}
		if nA < nB {
			return -1
		}
	}
	return 0
}
