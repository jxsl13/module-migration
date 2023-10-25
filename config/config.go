package config

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/jxsl13/module-migration/csv"
)

type Config struct {
	RemoteName string `koanf:"remote" short:"r" description:"name of the remote url"`
	BranchName string `koanf:"branch" short:"b" description:"name of the branch that should be crated for the changes, if empty no branch migration will be executed with git"`
	CSVPath    string `koanf:"csv" short:"c" description:"path to csv mapping file"`
	Include    string `koanf:"include" short:"i" description:"';' separated list of include file paths matching regular expression"`
	Exclude    string `koanf:"exclude" short:"e" description:"';' separated list of exclude file paths matching regular expression"`
	Comma      string `koanf:"separator" short:"s" description:"column separator character in csv"`
	OldColumn  string `koanf:"old" short:"o" description:"column name of index (starting with 0) containing the old [git] url"`
	NewColumn  string `koanf:"new" short:"n" description:"column name of index (starting with 0) containing the new [git] url"`

	comma   rune
	include []*regexp.Regexp
	exclude []*regexp.Regexp

	oldIdx int
	newIdx int
}

func (c *Config) CommaRune() rune {
	return c.comma
}

func (c *Config) IncludeRegex() []*regexp.Regexp {
	return c.include
}

func (c *Config) ExcludeRegex() []*regexp.Regexp {
	return c.exclude
}

func (c *Config) OldColumnIndex() int {
	return c.oldIdx
}

func (c *Config) NewColumnIndex() int {
	return c.newIdx
}

func (c *Config) Validate() error {
	if len(c.CSVPath) == 0 {
		return errors.New("csv file path is empty")
	}
	comma := ([]rune(c.Comma))
	if len(comma) == 0 {
		return errors.New("column separator is empty")
	}
	c.comma = comma[0]

	ss := strings.Split(c.Include, ";")
	c.include = make([]*regexp.Regexp, 0, len(ss))
	for _, s := range ss {
		r, err := regexp.Compile(s)
		if err != nil {
			return fmt.Errorf("invalid include regex: %q: %w", s, err)
		}
		c.include = append(c.include, r)
	}

	ss = strings.Split(c.Exclude, ";")
	c.exclude = make([]*regexp.Regexp, 0, len(ss))
	for _, s := range ss {
		r, err := regexp.Compile(s)
		if err != nil {
			return fmt.Errorf("invalid exclude regex: %q: %w", s, err)
		}
		c.exclude = append(c.exclude, r)
	}

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
