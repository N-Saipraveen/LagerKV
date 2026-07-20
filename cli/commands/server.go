package commands

import (
	"github.com/N-Saipraveen/LagerKV/cli/output"
	"github.com/spf13/cobra"
)

// NewServerCommand creates a new server command
func NewServerCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "server",
		Short: "Start the LagerKV server",
		Run: func(cmd *cobra.Command, args []string) {
			output.Info("Starting LagerKV server...")
			output.Info("Server commands are not implemented in CLI mode.")
			output.Info("Use 'lagerkvd' binary to start the server.")
		},
	}
}
