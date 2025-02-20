package cmd

import (
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/NibiruChain/cosmopilot/pkg/dataexporter"
	"github.com/NibiruChain/cosmopilot/pkg/environ"
)

var concurrentDeleteJobs int

var deleteCmd = &cobra.Command{
	Use:   "delete <bucket> <name>",
	Short: "Deletes objects from external storage",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		bucket, name := args[0], args[1]
		start := time.Now()
		err := exporter.Delete(bucket, name,
			dataexporter.WithConcurrentDeleteJobs(concurrentDeleteJobs),
		)
		if err != nil {
			log.Fatal(err)
		}
		log.WithField("time-elapsed", time.Now().Sub(start)).Info("delete successful")
	},
}

func init() {
	deleteCmd.Flags().IntVar(&concurrentDeleteJobs, "concurrent-jobs",
		environ.GetInt("CONCURRENT_JOBS", dataexporter.DefaultConcurrentJobs),
		"Number of concurrent jobs",
	)
}
