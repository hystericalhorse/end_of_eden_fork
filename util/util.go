package util

import (
	"sort"
	"strings"
)

func Min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

func Max(x, y int) int {
	if x > y {
		return x
	}
	return y
}

func CopyMap[K, V comparable](m map[K]V) map[K]V {
	result := make(map[K]V)
	for k, v := range m {
		result[k] = v
	}
	return result
}

func InsertString(s string, insert string, n int) string {
	return s[:n] + insert + s[n:]
}

func RemoveAnsiReset(s string) string {
	return strings.Replace(s, "\x1b[0m", "", 1)
}

func SortStringsStable(s []string) []string {
	sort.Strings(s)
	return s
}
