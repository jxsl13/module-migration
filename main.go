package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/jxsl13/module-migration/cmd/commit"
	"github.com/jxsl13/module-migration/cmd/migrate"
	"github.com/jxsl13/module-migration/cmd/release"
	"github.com/spf13/cobra"
)

func main() {
	err := NewRootCmd().Execute()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func NewRootCmd() *cobra.Command {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)

	rootContext := rootContext{Ctx: ctx}

	// rootCmd represents the run command
	rootCmd := &cobra.Command{
		Use:   "module-migration root/path mapping.csv",
		Short: "replace all imports, module names and more using a csv file that contains mapping of old module git paths and new module git paths",
		RunE:  rootContext.RunE,
		PostRunE: func(cmd *cobra.Command, args []string) error {

			cancel()
			return nil
		},
	}

	// register flags but defer parsing and validation of the final values
	rootCmd.AddCommand(NewCompletionCmd(rootCmd.Name()))
	rootCmd.AddCommand(migrate.NewMigrateCmd())
	rootCmd.AddCommand(commit.NewCommitCmd())
	rootCmd.AddCommand(release.NewReleaseCmd())
	return rootCmd
}

type rootContext struct {
	Ctx context.Context
}

func (c *rootContext) RunE(cmd *cobra.Command, args []string) (err error) {
	return cmd.Usage()
}
