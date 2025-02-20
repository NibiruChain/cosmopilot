package cmd

import (
	"github.com/spf13/cobra"

	"github.com/NibiruChain/cosmopilot/pkg/dataexporter"
)

var gcsCmd = &cobra.Command{
	Use:   "gcs",
	Short: "Google Cloud Storage (GCS) operations",
	Long:  "Manage uploads and deletions in Google Cloud Storage (GCS).",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		err := cmd.Parent().PersistentPreRunE(cmd.Parent(), args)
		if err != nil {
			return err
		}
		exporter, err = dataexporter.FromProvider(dataexporter.GCS)
		return err
	},
}

func init() {
	rootCmd.AddCommand(gcsCmd)
	gcsCmd.AddCommand(uploadCmd)
	gcsCmd.AddCommand(deleteCmd)
}
