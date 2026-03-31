package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// version is set via ldflags at build time.
var version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print csm version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("csm " + version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
