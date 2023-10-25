package main

import (
	"path/filepath"
	"strings"
)

const sep = string(filepath.Separator)

type byPathSeparators []string

func (a byPathSeparators) Len() int      { return len(a) }
func (a byPathSeparators) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a byPathSeparators) Less(i, j int) bool {
	li := strings.Count(a[i], sep)
	lj := strings.Count(a[j], sep)

	if li == lj {
		return a[i] < a[j]
	}

	return li < lj
}
