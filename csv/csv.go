package csv

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	giturls "github.com/whilp/git-urls"
)

func Header(filePath string, commaRune rune) ([]string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.Comma = commaRune
	r.ReuseRecord = true

	record, err := r.Read()
	if err != nil {
		return nil, err
	}
	return record, nil
}

// returns source and target urls as map
func NewReplacerFromCSV(filePath string, oldColumn, newColumn int, commaRune rune) (gitUrlMap, moduleMap map[string]string, err error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	maxIndex := oldColumn
	if newColumn > oldColumn {
		maxIndex = newColumn
	}

	r := csv.NewReader(f)
	r.Comma = commaRune
	r.ReuseRecord = true
	gitUrlMap = make(map[string]string, 512)
	moduleMap = make(map[string]string, 512)
	row := 0
	for {
		record, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, nil, err
		}

		if len(record) <= maxIndex {
			return nil, nil, fmt.Errorf("record %v has no index %d", record, maxIndex)
		}

		if row == 0 {
			row++
			continue
		}

		oldUrl := record[oldColumn]
		newUrl := record[newColumn]

		if oldUrl == "" || newUrl == "" {
			continue
		}

		o, err := giturls.Parse(oldUrl)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid old url: %s", oldUrl)
		}

		n, err := giturls.Parse(newUrl)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid new url: %s", newUrl)
		}

		gitUrlOld := strings.TrimLeft(o.String(), "/")
		gitUrlNew := strings.TrimLeft(n.String(), "/")

		gitUrlMap[gitUrlOld] = gitUrlNew

		// remove scheme only for import mapping
		o.Scheme = ""
		o.User = nil
		n.Scheme = ""
		n.User = nil

		oldModuleUrl := strings.TrimSuffix(strings.TrimLeft(o.String(), "/"), ".git")
		newModuleUrl := strings.TrimSuffix(strings.TrimLeft(n.String(), "/"), ".git")
		moduleMap[oldModuleUrl] = newModuleUrl
		row++
	}
	return gitUrlMap, moduleMap, nil
}
