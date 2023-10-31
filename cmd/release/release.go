package release

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"

	"github.com/jxsl13/module-migration/config"
	"github.com/jxsl13/module-migration/utils"
	"github.com/spf13/cobra"
)

func NewReleaseCmd() *cobra.Command {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)

	releaseContext := releaseContext{
		Ctx: ctx,
	}

	// cmd represents the run command
	cmd := &cobra.Command{
		Use:   "release",
		Short: "walks through all known git repositories that have a known remote url and releases all the changes created by the migrate subcommand",
		Args:  cobra.ExactArgs(1),
		RunE:  releaseContext.RunE,
		PostRunE: func(cmd *cobra.Command, args []string) error {

			cancel()
			return nil
		},
	}

	// register flags but defer parsing and validation of the final values
	cmd.PreRunE = releaseContext.PreRunE(cmd)

	return cmd
}

type releaseContext struct {
	Ctx      context.Context
	Config   *ReleaseConfig
	RootPath string `koanf:"root.path" short:"" description:"root search directory"`
}

func (c *releaseContext) PreRunE(cmd *cobra.Command) func(cmd *cobra.Command, args []string) error {
	c.Config = &ReleaseConfig{
		RemoteName: "origin",
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

func (c *releaseContext) RunE(cmd *cobra.Command, args []string) (err error) {
	var (
		ctx        = c.Ctx
		remoteName = c.Config.RemoteName
		push       = c.Config.Push
	)
	repoDirs, err := utils.FindRepoDirs(c.RootPath)
	if err != nil {
		return fmt.Errorf("failed to find git folders: %w", err)
	}

	var wg sync.WaitGroup
	wg.Add(len(repoDirs))
	for _, repoDir := range repoDirs {
		go func(repoDir string) {
			defer wg.Done()

			err := bump(ctx, repoDir, remoteName, push)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to release repo %s: %v\n", repoDir, err)
			} else {
				fmt.Printf("Successfully released %s\n", repoDir)
			}
		}(repoDir)
	}
	wg.Wait()

	return nil
}

func bump(ctx context.Context, repoDir, remoteName string, push bool) error {

	err := utils.GitBumpVersionTag(ctx, repoDir, remoteName, false, false, true)
	if err != nil {
		return err
	}

	if !push {
		return nil
	}

	return utils.GitPushTags(ctx, repoDir, remoteName)
}
