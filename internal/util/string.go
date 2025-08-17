package util

import (
	"strings"
)

func LineCount(s string) uint64 {
	n := strings.Count(s, "\n")
	return uint64(n)
}
