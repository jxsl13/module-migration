package utils

import (
	"fmt"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"io/fs"
	"os"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/tools/go/ast/astutil"
)

func ReplaceInDir(rootPath string, exclude, include []*regexp.Regexp, replacer *strings.Replacer) ([]string, error) {
	touchedFiles := make([]string, 0, 512)
	fset := token.NewFileSet()
	err := WalkMatching(rootPath, exclude, include, func(path string, info fs.FileInfo, e error) (err error) {
		if e != nil {
			return fmt.Errorf("%s: %w", path, e)
		}
		defer func() {
			touchedFiles = append(touchedFiles, path)
		}()

		if !strings.HasSuffix(path, ".go") {
			e = replaceInNonGoFile(path, replacer)
			if e != nil {
				return e
			}
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		f, err := parser.ParseFile(fset, path, data, parser.AllErrors|parser.ParseComments)
		if err != nil {
			return fmt.Errorf("invalid Go file: %s: %w", path, err)
		}

		imports := astutil.Imports(fset, f)

		for _, imps := range imports {
			for _, imp := range imps {
				name := ""
				if imp.Name != nil {
					name = imp.Name.Name
				}
				before := strings.Trim(imp.Path.Value, `"`)
				after := replacer.Replace(before)
				if after != before {

					if name != "" {
						astutil.DeleteNamedImport(fset, f, name, before)
						astutil.AddNamedImport(fset, f, name, after)
					} else {
						astutil.DeleteImport(fset, f, before)
						astutil.AddImport(fset, f, after)
					}

				}
			}

		}

		file, err := os.OpenFile(path, os.O_RDWR|os.O_TRUNC, 0755)
		if err != nil {
			return err
		}
		defer file.Close()

		err = printer.Fprint(file, fset, f)
		if err != nil {
			return err
		}

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

func replaceInNonGoFile(path string, replacer *strings.Replacer) error {
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
	return nil
}
