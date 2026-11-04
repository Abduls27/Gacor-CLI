package cmd

import (
	"gacor/internal/ui"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "gacor",
	Short: "Dev environment manager",
	RunE: func(cmd *cobra.Command, args []string) error {
		root := cmd.Root()
		for {
			if err := updateProjectMenu(); err != nil {
				return err
			}
			chosen, err := ui.Home(root)
			if err != nil || chosen == nil {
				return err
			}
			root.SetArgs(ui.CommandPath(chosen))
			if err := root.Execute(); err != nil {
				return err
			}
			if chosen != databaseShowCmd {
				return nil
			}
			root.SetArgs([]string{})
		}
	},
}



func init() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(
		runCmd,
		databaseCmd,
		stopCmd,
		statusCmd,
		phpCmd,
		nginxCmd,
		projectCmd,
		secureCmd,
		configCmd,
		hostsSyncCmd,
	)
}
