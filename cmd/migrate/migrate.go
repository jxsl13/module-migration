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
	"golang.org/x/mod/modfile"
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
		BranchName: "chore/module-migration",
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

	_, moduleMap, err := csv.NewReplacerFromCSV(
		c.Config.CSVPath,
		c.Config.OldColumnIndex(),
		c.Config.NewColumnIndex(),
		c.Config.CommaRune(),
	)
	if err != nil {
		return err
	}

	importReplacer := utils.NewReplacer(moduleMap)

	repoDirs, err := utils.FindGoRepoDirs(c.RootPath)
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
				repoDir,
				c.Config.RemoteName,
				c.Config.BranchName,
				c.Config.Additional(),
				c.Config.ExcludeRegex(),
				c.Config.IncludeRegex(),
				moduleMap,
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
	repoDir,
	remoteName string,
	targetBranch string,
	additionalFiles []string,
	exclude, include []*regexp.Regexp,
	moduleMap map[string]string,
	importReplacer *strings.Replacer,
) (err error) {

	// pull before changing anything
	_ = utils.GitPull(ctx, repoDir)
	goMod := filepath.Join(repoDir, "go.mod")
	err = migrateGoMod(ctx, repoDir, goMod, moduleMap)
	if err != nil {
		return fmt.Errorf("failed to migrate go mod: %s: %w", goMod, err)
	}

	exclude = append(exclude, regexp.MustCompile(`go\.mod$`), regexp.MustCompile(`go\.sum$`))
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

	// fix go.sum file
	err = utils.GoModTidy(ctx, repoDir)
	if err != nil {
		return err
	}

	return utils.GoBuildAll(ctx, repoDir)
}

func migrateGoMod(ctx context.Context, repoDir, goModFilePath string, moduleMap map[string]string) error {

	data, err := os.ReadFile(goModFilePath)
	if err != nil {
		return err
	}

	modFile, err := modfile.Parse(goModFilePath, data, nil)
	if err != nil {
		return fmt.Errorf("failed to read go mod file: %w", err)
	}

	// map module name
	moduleName := modFile.Module.Mod.Path
	if targetModuleName, found := moduleMap[moduleName]; found {
		fmt.Printf("Module: mapping found: %s -> %s\n", moduleName, targetModuleName)
		modFile.AddModuleStmt(targetModuleName)
	} else {
		fmt.Printf("Module: nothing to change for %s\n", moduleName)
	}

	// map dependencies
	foundDependencies := make([]string, 0, 1)
	for _, req := range modFile.Require {
		targetModulePath, found := moduleMap[req.Mod.Path]
		if !found {
			fmt.Printf("Dependency: nothing to do: %s\n", req.Mod.Path)
			continue
		}

		err = modFile.DropRequire(req.Mod.Path)
		if err != nil {
			return fmt.Errorf("failed to drop old dependency: %s: %w", req.Mod.Path, err)
		}

		fmt.Printf("Found dependency mapping: %s -> %s\n", req.Mod.Path, targetModulePath)
		foundDependencies = append(foundDependencies, targetModulePath)
	}

	modFile.Cleanup()

	data, err = modFile.Format()
	if err != nil {
		return fmt.Errorf("failed to format %s: %w", goModFilePath, err)
	}

	err = os.WriteFile(goModFilePath, data, 0666)
	if err != nil {
		return fmt.Errorf("failed to write to %s: %w", goModFilePath, err)
	}

	for _, dep := range foundDependencies {
		fmt.Printf("Dependency: updating: %s\n", dep)
		err = utils.GoGet(ctx, repoDir, fmt.Sprintf("%s@latest", dep))
		if err != nil {
			return err
		}
	}

	return nil
}
