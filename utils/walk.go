package utils

import (
	"io/fs"
	"path/filepath"
	"regexp"

	"github.com/jxsl13/cwalk"
)

func WalkMatching(rootPath string, exclude, include []*regexp.Regexp, walk filepath.WalkFunc) error {
	return cwalk.Walk(rootPath, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return walk(path, nil, err)
		}
		for _, re := range exclude {
			if re.MatchString(path) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		keep := false
		for _, re := range include {
			if re.MatchString(path) {
				keep = true
				break
			}
		}
		if !keep {
			return nil
		}

		return walk(path, info, err)
	})
}
