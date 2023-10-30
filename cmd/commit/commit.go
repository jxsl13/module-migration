package commit

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"

	"github.com/jxsl13/module-migration/config"
	"github.com/jxsl13/module-migration/csv"
	"github.com/jxsl13/module-migration/utils"
	"github.com/spf13/cobra"
)

func NewCommitCmd() *cobra.Command {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)

	commitContext := commitContext{
		Ctx: ctx,
	}

	// cmd represents the run command
	cmd := &cobra.Command{
		Use:   "commit",
		Short: "walks through all known git repositories that have a known remote url and commits all the changes created by the migrate subcommand",
		Args:  cobra.ExactArgs(1),
		RunE:  commitContext.RunE,
		PostRunE: func(cmd *cobra.Command, args []string) error {

			cancel()
			return nil
		},
	}

	// register flags but defer parsing and validation of the final values
	cmd.PreRunE = commitContext.PreRunE(cmd)

	return cmd
}

type commitContext struct {
	Ctx      context.Context
	Config   *CommitConfig
	RootPath string `koanf:"root.path" short:"" description:"root search directory"`
}

func (c *commitContext) PreRunE(cmd *cobra.Command) func(cmd *cobra.Command, args []string) error {
	c.Config = &CommitConfig{
		RemoteName: "origin",
		CSVPath:    "./mapping.csv",
		Comma:      ";", // default separator
		OldColumn:  "0",
		NewColumn:  "1",
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

func (c *commitContext) RunE(cmd *cobra.Command, args []string) (err error) {
	gitUrlMap, _, err := csv.NewReplacerFromCSV(
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
			err := commit(c.Ctx, gitUrlMap, targetUrlMap, repoDir, c.Config.RemoteName, c.Config.BranchName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to commit repo %s: %v\n", repoDir, err)
			} else {
				fmt.Printf("Successfully committed %s\n", repoDir)
			}
		}(repoDir)
	}
	wg.Wait()

	return nil
}

func commit(ctx context.Context,
	gitUrlMap map[string]string,
	targetUrlMap map[string]bool,
	repoDir,
	remoteName string,
	targetBranch string) error {

	repoUrl, err := utils.GitRemoteUrl(ctx, repoDir, remoteName)
	if err != nil {
		return err
	}

	targetUrl, found := gitUrlMap[repoUrl]
	if found {
		err = utils.GitChangeRemoteUrl(ctx, repoDir, remoteName, targetBranch)
		if err != nil {
			return err
		}
	} else if targetUrlMap[repoUrl] {
		// nothing todo, already target url
		targetUrl = repoUrl
	} else {
		return fmt.Errorf("skipping %s: unkown remote url: %s", repoDir, repoUrl)
	}

	// check if target url exists
	err = utils.GitCheckRemoteUrl(ctx, repoDir, targetUrl)
	if err != nil {
		return err
	}
	err = utils.GitRefreshIndex(ctx, repoDir)
	if err != nil {
		return fmt.Errorf("failed to refresh git repo index: %w", err)
	}

	currentBranch, err := utils.GitBranchName(ctx, repoDir)
	if err != nil {
		return err
	}

	if currentBranch != targetBranch {
		// create a new branch with the current changes
		err = utils.GitCheckoutNewBranch(ctx, repoDir, targetBranch)
		if err != nil {
			return err
		}
		defer func() {
			if err != nil {
				e := utils.GitCheckoutBranch(ctx, repoDir, currentBranch)
				if e != nil {
					err = errors.Join(err, e)
					return
				}

				e = utils.GitDeleteBranch(ctx, repoDir, targetBranch)
				if e != nil {
					err = errors.Join(err, e)
					return
				}
				e = utils.GitDeleteRemoteBranch(ctx, repoDir, remoteName, targetBranch)
				if e != nil {
					err = errors.Join(err, e)
					return
				}
			}
		}()
	}

	err = utils.GitAddAll(ctx, repoDir)
	if err != nil {
		return err
	}

	// max commit subject length is 50 characters
	// max body length is 75 characters
	err = utils.GitCommit(ctx, repoDir, "chore: Go module migration")
	if err != nil {
		return err
	}

	err = utils.GitPushUpstream(ctx, repoDir, remoteName, targetBranch)
	if err != nil {
		return err
	}

	err = utils.CreateGithubPullRequest(ctx, repoDir, "chore: Go module migration")
	if err != nil {
		return err
	}
	return nil
}
