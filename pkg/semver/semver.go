package semver

import (
	"strconv"
	"strings"
)

// Compare compares two semver strings. Returns -1, 0, or 1.
func Compare(a, b string) int {
	ap := parseParts(a)
	bp := parseParts(b)
	for i := 0; i < 3; i++ {
		if ap[i] < bp[i] {
			return -1
		}
		if ap[i] > bp[i] {
			return 1
		}
	}
	return 0
}

func parseParts(v string) [3]int {
	if idx := strings.IndexAny(v, "-+"); idx != -1 {
		v = v[:idx]
	}
	parts := strings.SplitN(v, ".", 3)
	var result [3]int
	for i := 0; i < 3 && i < len(parts); i++ {
		result[i], _ = strconv.Atoi(parts[i])
	}
	return result
}
