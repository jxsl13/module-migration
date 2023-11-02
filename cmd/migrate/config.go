package migrate

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/jxsl13/module-migration/csv"
	"github.com/jxsl13/module-migration/defaults"
	"github.com/jxsl13/module-migration/utils"
)

type MigrateConfig struct {
	// shared flags
	CSVPath string `koanf:"csv" short:"c" description:"path to csv mapping file"`

	Comma     string `koanf:"separator" short:"s" description:"column separator character in csv"`
	OldColumn string `koanf:"old" short:"o" description:"column name or index (starting with 0) containing the old [git] url"`
	NewColumn string `koanf:"new" short:"n" description:"column name or index (starting with 0) containing the new [git] url"`

	RemoteName string `koanf:"remote" short:"r" description:"name of the remote url"`
	BranchName string `koanf:"branch" short:"b" description:"name of the branch that should be crated for the changes, if empty no branch migration will be executed with git"`

	comma rune

	oldIdx int
	newIdx int

	// subcommand specific flags
	Include         string `koanf:"include" short:"i" description:"',' separated list of include file paths matching regular expression"`
	Exclude         string `koanf:"exclude" short:"e" description:"',' separated list of exclude file paths matching regular expression"`
	AdditionalFiles string `koanf:"copy" description:"moves specified files or directories into your repository (, separated)"`

	include    []*regexp.Regexp
	exclude    []*regexp.Regexp
	additional []string
}

func (c *MigrateConfig) Validate() error {
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

	ss := strings.Split(c.Include, defaults.ListSeparator)
	c.include = make([]*regexp.Regexp, 0, len(ss))
	for _, s := range ss {
		r, err := regexp.Compile(s)
		if err != nil {
			return fmt.Errorf("invalid include regex: %q: %w", s, err)
		}
		c.include = append(c.include, r)
	}

	ss = strings.Split(c.Exclude, defaults.ListSeparator)
	c.exclude = make([]*regexp.Regexp, 0, len(ss))
	for _, s := range ss {
		r, err := regexp.Compile(s)
		if err != nil {
			return fmt.Errorf("invalid exclude regex: %q: %w", s, err)
		}
		c.exclude = append(c.exclude, r)
	}

	c.additional = strings.Split(c.AdditionalFiles, defaults.ListSeparator)

	for _, filename := range c.additional {
		_, found, err := utils.Exists(filename)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("file or directory not found: %s", filename)
		}
	}
	return nil
}

func (c *MigrateConfig) IncludeRegex() []*regexp.Regexp {
	return c.include
}

func (c *MigrateConfig) ExcludeRegex() []*regexp.Regexp {
	return c.exclude
}

func (c *MigrateConfig) Additional() []string {
	return c.additional
}

func (c *MigrateConfig) CommaRune() rune {
	return c.comma
}

func (c *MigrateConfig) OldColumnIndex() int {
	return c.oldIdx
}

func (c *MigrateConfig) NewColumnIndex() int {
	return c.newIdx
}
