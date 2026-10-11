package cmd

import (
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/voluzi/cosmopilot/v2/pkg/dataexporter"
	"github.com/voluzi/cosmopilot/v2/pkg/environ"
)

var partSize string
var chunkSize string
var sizeLimit string
var reportPeriod time.Duration
var concurrentUploadJobs int
var bufferSize string

var uploadCmd = &cobra.Command{
	Use:   "upload <dir> <bucket> <name>",
	Short: "Uploads data to external storage",
	Args:  cobra.ExactArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		dir, bucket, name := args[0], args[1], args[2]
		start := time.Now()
		err := exporter.Upload(dir, bucket, name,
			dataexporter.WithChunkSize(chunkSize),
			dataexporter.WithSizeLimit(sizeLimit),
			dataexporter.WithPartSize(partSize),
			dataexporter.WithReportPeriod(reportPeriod),
			dataexporter.WithConcurrentUploadJobs(concurrentUploadJobs),
			dataexporter.WithBufferSize(bufferSize),
		)
		if err != nil {
			log.Fatal(err)
		}
		log.WithField("time-elapsed", time.Since(start)).Info("upload successful")
	},
}

func init() {
	uploadCmd.Flags().StringVar(&chunkSize, "chunk-size",
		environ.GetString("CHUNK_SIZE", dataexporter.DefaultChunkSize),
		"Chunk size for multi-part uploads",
	)
	uploadCmd.Flags().StringVar(&partSize, "part-size",
		environ.GetString("PART_SIZE", dataexporter.DefaultPartSize),
		"Part size on multi-part uploads (when size limit is crossed)",
	)
	uploadCmd.Flags().StringVar(&sizeLimit, "size-limit",
		environ.GetString("SIZE_LIMIT", dataexporter.DefaultSizeLimit),
		"Size limit for single file",
	)
	uploadCmd.Flags().DurationVar(&reportPeriod, "report-period",
		environ.GetDuration("REPORT_PERIOD", dataexporter.DefaultReportPeriod),
		"Period for progress reporting",
	)
	uploadCmd.Flags().IntVar(&concurrentUploadJobs, "concurrent-jobs",
		environ.GetInt("CONCURRENT_JOBS", dataexporter.DefaultConcurrentJobs),
		"Number of concurrent jobs",
	)
	uploadCmd.Flags().StringVar(&bufferSize, "buffer-size",
		environ.GetString("BUFFER_SIZE", dataexporter.DefaultBufferSize),
		"Buffer size on upload",
	)
}
