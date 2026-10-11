package cmd

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/voluzi/cosmopilot/v2/pkg/dataexporter"
	"github.com/voluzi/cosmopilot/v2/pkg/environ"
)

var exporter dataexporter.Exporter
var logLevel string

var rootCmd = &cobra.Command{
	Use:   "dataexporter",
	Short: "CLI tool for exporting and managing data",
	Long:  `DataExporter is a command-line tool for uploading and deleting tarballs to external storage.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		logLvl, err := log.ParseLevel(logLevel)
		if err != nil {
			return fmt.Errorf("invalid log level: %w", err)
		}
		log.SetLevel(logLvl)
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&logLevel,
		"log-level",
		environ.GetString("LOG_LEVEL", "info"),
		"Log level. One of debug, info, warn, error, fatal, panic.",
	)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
