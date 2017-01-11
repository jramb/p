package cmd

import (
	"github.com/jramb/p/server"
	"github.com/spf13/cobra"
)

var serverCmd = &cobra.Command{
	Use:   `server`,
	Short: `Start the interactive server and wait for connections`,
	Long: `Start the interactive server and wait for connections.
	`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return server.StartServer(args)
	},
}

func init() {
	RootCmd.AddCommand(serverCmd)
}
