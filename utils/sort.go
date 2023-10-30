package utils

import (
	"path/filepath"
	"strings"
)

const Separator = string(filepath.Separator)

type byPathSeparators []string

func (a byPathSeparators) Len() int      { return len(a) }
func (a byPathSeparators) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a byPathSeparators) Less(i, j int) bool {
	li := strings.Count(a[i], Separator)
	lj := strings.Count(a[j], Separator)

	if li == lj {
		return a[i] < a[j]
	}

	return li < lj
}
