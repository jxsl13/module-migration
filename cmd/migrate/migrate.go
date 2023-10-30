package migrate

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/jxsl13/module-migration/config"
	"github.com/jxsl13/module-migration/csv"
	"github.com/jxsl13/module-migration/defaults"
	"github.com/jxsl13/module-migration/utils"
	"github.com/spf13/cobra"
)

func NewMigrateCmd() *cobra.Command {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)

	migrateContext := migrateContext{
		Ctx: ctx,
	}

	// cmd represents the run command
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "replace all imports, module names and more using a csv file that contains mapping of old module git paths and new module git paths",
		Args:  cobra.ExactArgs(1),
		RunE:  migrateContext.RunE,
		PostRunE: func(cmd *cobra.Command, args []string) error {

			cancel()
			return nil
		},
	}

	// register flags but defer parsing and validation of the final values
	cmd.PreRunE = migrateContext.PreRunE(cmd)

	return cmd
}

type migrateContext struct {
	Ctx      context.Context
	Config   *MigrateConfig
	RootPath string `koanf:"root.path" short:"" description:"root search directory"`
}

func (c *migrateContext) PreRunE(cmd *cobra.Command) func(cmd *cobra.Command, args []string) error {
	c.Config = &MigrateConfig{
		RemoteName: "origin",
		CSVPath:    "./mapping.csv",
		Comma:      ";", // default separator
		OldColumn:  "0",
		NewColumn:  "1",
		Include:    strings.Join(defaults.Include, defaults.ListSeparator),
		Exclude:    strings.Join(defaults.Exclude, defaults.ListSeparator),
	}

	runParser := config.RegisterFlags(c.Config, true, cmd)

	return func(cmd *cobra.Command, args []string) error {
		abs, err := filepath.Abs(args[0])
		if err != nil {
			return err
		}
		c.RootPath = abs

		return runParser()
	}
}

func (c *migrateContext) RunE(cmd *cobra.Command, args []string) (err error) {

	gitUrlMap, importReplacer, err := csv.NewReplacerFromCSV(
		c.Config.CSVPath,
		c.Config.OldColumnIndex(),
		c.Config.NewColumnIndex(),
		c.Config.CommaRune(),
	)
	if err != nil {
		return err
	}

	targetUrlMap := make(map[string]bool, len(gitUrlMap))
	for _, v := range gitUrlMap {
		targetUrlMap[v] = true
	}

	repoDirs, err := utils.FindRepoDirs(c.RootPath)
	if err != nil {
		return fmt.Errorf("failed to find git folders: %w", err)
	}

	var wg sync.WaitGroup
	wg.Add(len(repoDirs))
	for _, repoDir := range repoDirs {
		go func(repoDir string) {
			defer wg.Done()
			err := migrateRepo(
				c.Ctx,
				gitUrlMap,
				targetUrlMap,
				repoDir,
				c.Config.RemoteName,
				c.Config.BranchName,
				c.Config.Additional(),
				c.Config.ExcludeRegex(),
				c.Config.IncludeRegex(),
				importReplacer,
			)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to migrate repo %s: %v\n", repoDir, err)
			} else {
				fmt.Printf("Successfully migrated %s\n", repoDir)
			}
		}(repoDir)
	}
	wg.Wait()

	return nil
}

func migrateRepo(ctx context.Context,
	gitUrlMap map[string]string,
	targetUrlMap map[string]bool,
	repoDir,
	remoteName string,
	targetBranch string,
	additionalFiles []string,
	exclude, include []*regexp.Regexp,
	importReplacer *strings.Replacer,
) (err error) {

	// pull before changing anything
	_ = utils.GitPull(ctx, repoDir)

	_, err = utils.ReplaceInDir(repoDir, exclude, include, importReplacer)
	if err != nil {
		return err
	}

	for _, af := range additionalFiles {
		err = utils.Copy(ctx, af, repoDir)
		if err != nil {
			return err
		}
	}

	err = utils.GoModTidy(ctx, repoDir)
	if err != nil {
		return err
	}
	return nil
}
