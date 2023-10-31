package utils

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"regexp"
	"sort"
	"strings"
)

func ReplaceInDir(rootPath string, exclude, include []*regexp.Regexp, replacer *strings.Replacer) ([]string, error) {
	touchedFiles := make([]string, 0, 512)
	err := WalkMatching(rootPath, exclude, include, func(path string, info fs.FileInfo, e error) error {
		if e != nil {
			return fmt.Errorf("%s: %w", path, e)
		}

		f, err := os.OpenFile(path, os.O_RDWR, 0)
		if err != nil {
			return err
		}
		defer f.Close()

		data, err := io.ReadAll(f)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", path, err)
		}
		err = f.Truncate(0)
		if err != nil {
			return fmt.Errorf("failed to truncate: %s: %w", path, err)
		}
		_, err = f.Seek(0, 0)
		if err != nil {
			return fmt.Errorf("failed to reset file cursor to 0: %s: %w", path, err)
		}

		_, err = replacer.WriteString(f, string(data))
		if err != nil {
			return fmt.Errorf("failed to write replaced file content of %s: %w", path, err)
		}

		touchedFiles = append(touchedFiles, path)
		return nil
	})

	sort.Sort(byPathSeparators(touchedFiles))
	return touchedFiles, err
}

func sortedKeys[V any](m map[string]V) []string {
	result := make([]string, 0, len(m))
	for k := range m {
		result = append(result, k)
	}

	sort.Strings(result)
	return result
}
