package cmd

import (
	"gacor/internal/config"
	"gacor/internal/hosts"
	"gacor/internal/nginx"
	"gacor/internal/project"

	"github.com/spf13/cobra"
)

var hostsSyncCmd = &cobra.Command{
	Use:    "hosts-sync",
	Short:  "Update the hosts file (called automatically by gacor run)",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		projects := project.ListProjects(cfg)
		domains := nginx.DomainsFromProjects(projects)
		return hosts.Update(domains)
	},
}
