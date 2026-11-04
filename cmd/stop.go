package cmd

import (
	"errors"
	"fmt"
	"gacor/internal/database"
	"gacor/internal/process"
	"gacor/internal/ui"

	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop PHP, Nginx, and the database",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println()
		ui.StepTimed("Welcome to Gacor - stopping your Dev Environment")

		ui.StepTimed("Stopping PHP...")
		if err := process.Stop("php"); err != nil {
			ui.Warn(err.Error())
		} else {
			ui.StepDetail("PHP stopped")
		}

		ui.StepTimed("Stopping Nginx...")
		if err := process.Stop("nginx"); err != nil {
			ui.Warn(err.Error())
		} else {
			ui.StepDetail("Nginx stopped")
		}

		instance, err := configuredDatabase()
		if !errors.Is(err, errNoDatabaseConfigured) && err != nil {
			ui.Warn(err.Error())
		} else if err == nil {
			ui.StepTimed("Stopping " + instance.Engine.String() + "...")
			if !database.IsRunning(instance) {
				ui.StepDetail(instance.Engine.String() + " is not running")
			} else if err := stopDatabase(instance); err != nil {
				ui.Warn(err.Error())
			} else {
				ui.StepDetail(instance.Engine.String() + " stopped")
			}
		}

		ui.StepTimed("Stop sequence completed.")
		fmt.Println()
		return nil
	},
}
