package commit

import (
	"errors"
	"strconv"

	"github.com/jxsl13/module-migration/csv"
)

type CommitConfig struct {
	CSVPath string `koanf:"csv" short:"c" description:"path to csv mapping file"`

	Comma     string `koanf:"separator" short:"s" description:"column separator character in csv"`
	OldColumn string `koanf:"old" short:"o" description:"column name of index (starting with 0) containing the old [git] url"`
	NewColumn string `koanf:"new" short:"n" description:"column name of index (starting with 0) containing the new [git] url"`

	RemoteName string `koanf:"remote" short:"r" description:"name of the remote url"`
	BranchName string `koanf:"branch" short:"b" description:"name of the branch that should be crated for the changes, if empty no branch migration will be executed with git"`

	comma rune

	oldIdx int
	newIdx int
}

func (c *CommitConfig) Validate() error {
	if len(c.CSVPath) == 0 {
		return errors.New("csv file path is empty")
	}
	comma := ([]rune(c.Comma))
	if len(comma) == 0 {
		return errors.New("column separator is empty")
	}
	c.comma = comma[0]

	oldIdx, errOld := strconv.Atoi(c.OldColumn)
	newIdx, errNew := strconv.Atoi(c.NewColumn)

	if errOld != nil || errNew != nil {
		header, err := csv.Header(c.CSVPath, c.comma)
		if err != nil {
			return err
		}

		for idx, col := range header {
			if col == c.OldColumn {
				oldIdx = idx
			}

			if col == c.NewColumn {
				newIdx = idx
			}
		}
	}

	c.oldIdx = oldIdx
	c.newIdx = newIdx

	return nil
}

func (c *CommitConfig) CommaRune() rune {
	return c.comma
}

func (c *CommitConfig) OldColumnIndex() int {
	return c.oldIdx
}

func (c *CommitConfig) NewColumnIndex() int {
	return c.newIdx
}
