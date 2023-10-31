package utils

import "strings"

func NewReplacer(m map[string]string) *strings.Replacer {
	replace := make([]string, 0, len(m)*2)
	for k, v := range m {
		replace = append(replace, k, v)
	}

	return strings.NewReplacer(replace...)
}
