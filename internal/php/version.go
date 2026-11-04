package php

import "strings"

// MajorMinor extracts "8.4" from "8.4.26".
func MajorMinor(fullVersion string) string {
	parts := strings.SplitN(fullVersion, ".", 3)
	if len(parts) < 2 {
		return fullVersion
	}
	return parts[0] + "." + parts[1]
}

// LatestFor finds the latest full version for a specific major.minor in `versions`.
func LatestFor(versions map[string]string, majorMinor string) (string, bool) {
	var latest string
	for full := range versions {
		if MajorMinor(full) == majorMinor {
			if latest == "" || compareVer(full, latest) > 0 {
				latest = full
			}
		}
	}
	return latest, latest != ""
}

func compareVer(a, b string) int {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			if len(pa[i]) != len(pb[i]) {
				return len(pa[i]) - len(pb[i])
			}
			if pa[i] > pb[i] {
				return 1
			}
			return -1
		}
	}
	return 0
}
