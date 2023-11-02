package utils

import (
	"sort"
	"strings"
)

func NewReplacer(m map[string]string) *strings.Replacer {
	replace := make([]string, 0, len(m)*2)

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	sort.Sort(byLen(keys))

	for _, key := range keys {
		replace = append(replace, key, m[key])
	}

	// we want to replace longer names before shorter names
	return strings.NewReplacer(replace...)
}

type byLen []string

func (a byLen) Len() int      { return len(a) }
func (a byLen) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a byLen) Less(i, j int) bool {
	ai := len(a[i])
	aj := len(a[j])

	if ai == aj {
		return a[i] < a[j]
	}

	return ai > aj
}
