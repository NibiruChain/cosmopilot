package dataexporter

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"cloud.google.com/go/storage"
	"github.com/c2h5oh/datasize"
	log "github.com/sirupsen/logrus"
	"google.golang.org/api/iterator"
)

const (
	GCS                   Provider = "gcs"
	CompositionBatchLimit          = 32
)

type GcsExporter struct {
	client *storage.Client
}

func NewGcsExporter() (*GcsExporter, error) {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %v", err)
	}
	return &GcsExporter{
		client: client,
	}, nil
}

func (gcs *GcsExporter) Provider() Provider {
	return GCS
}

func (gcs *GcsExporter) Upload(dir, bucket, name string, opts ...UploadOption) error {
	options := defaultUploadOptions()
	for _, opt := range opts {
		opt(options)
	}

	// Check directory existence
	fi, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("cannot stat directory %q: %v", dir, err)
	}
	if !fi.IsDir() {
		return fmt.Errorf("%q is not a directory", dir)
	}

	// Calculate total size (for progress % and composition)
	totalSize, err := GetDirSize(dir)
	if err != nil {
		return fmt.Errorf("failed to calculate directory size: %v", err)
	}
	if totalSize > options.SizeLimit && options.ChunkSize > options.PartSize {
		return errors.New("on multi-part, chunk size cannot be greater than part size")
	}

	log.WithFields(map[string]interface{}{
		"size":   datasize.ByteSize(totalSize).HumanReadable(),
		"source": dir,
		"target": fmt.Sprintf("gs://%s/%s.tar.gz", bucket, name),
	}).Infof("start compressing and uploading")

	// Create an io.Pipe to connect tar+gzip => GCS
	pr, pw := io.Pipe()

	// Tar + gzip in a goroutine
	go func() {
		defer pw.Close()
		if err := compressTarGz(dir, pw); err != nil {
			pw.CloseWithError(err)
		}
	}()

	return gcs.uploadChunks(context.Background(), pr, bucket, name, totalSize, options)
}

func (gcs *GcsExporter) uploadChunks(ctx context.Context, reader io.Reader, bucket, objectName string, totalSize datasize.ByteSize, opts *UploadOptions) error {
	partIndex := 0
	partNames := []string{}
	var bytesCompressed uint64
	var bytesUploaded atomic.Uint64

	// Progress
	progressCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func(ctx context.Context) {
		ticker := time.NewTicker(opts.ReportPeriod)
		defer ticker.Stop()

		var lastCompressed, lastUploaded uint64

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if lastCompressed != bytesCompressed || lastUploaded != bytesUploaded.Load() {
					log.WithFields(map[string]interface{}{
						"compressed": datasize.ByteSize(bytesCompressed).HumanReadable(),
						"uploaded":   datasize.ByteSize(bytesUploaded.Load()).HumanReadable(),
						"dir-size":   totalSize.HumanReadable(),
					}).Info("compressing and uploading",
						int(float64(bytesCompressed)/float64(totalSize)*100),
						int(float64(bytesUploaded.Load())/float64(totalSize)*100),
					)
				}
				lastCompressed, lastUploaded = bytesCompressed, bytesUploaded.Load()
			}
		}
	}(progressCtx)

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, opts.ConcurrentJobs)
	buf := make([]byte, opts.ChunkSize)

	for {
		semaphore <- struct{}{}

		log.WithFields(map[string]interface{}{
			"part-index": partIndex,
			"max-size":   opts.ChunkSize.HumanReadable(),
		}).Trace("reading chunk")
		n, err := io.ReadFull(reader, buf)
		if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
			return fmt.Errorf("error reading chunk: %v", err)
		}
		bytesCompressed += uint64(n)
		if n == 0 {
			break // Done reading
		}
		chunk := make([]byte, n)
		copy(chunk, buf[:n])
		log.WithFields(map[string]interface{}{
			"part-index": partIndex,
			"size":       datasize.ByteSize(len(chunk)).HumanReadable(),
		}).Trace("got chunk")

		partName := fmt.Sprintf("%s-part-%08d", objectName, partIndex)
		partIndex++
		partNames = append(partNames, partName)

		// Start upload in a goroutine
		wg.Add(1)
		go func(partData []byte, partName string, semaphore chan struct{}) {
			defer wg.Done()

			log.WithFields(map[string]interface{}{
				"part": partName,
				"size": datasize.ByteSize(len(partData)).HumanReadable(),
			}).Debug("starting part upload")
			if err := gcs.uploadToGCS(ctx,
				bucket,
				partName,
				newReaderWithBytesCounter(bytes.NewReader(partData), &bytesUploaded),
				opts.BufferSize.Bytes(),
			); err != nil {
				log.Errorf("failed to upload part %s: %v", partName, err)
			}
			log.WithFields(map[string]interface{}{
				"part": partName,
				"size": datasize.ByteSize(len(partData)).HumanReadable(),
			}).Debug("finish uploading part")
			<-semaphore
		}(chunk, partName, semaphore)

		if errors.Is(err, io.ErrUnexpectedEOF) {
			break // Last chunk
		}
	}

	wg.Wait()
	return gcs.composeParts(ctx, bucket, partNames, objectName, totalSize, opts)
}

func (gcs *GcsExporter) uploadToGCS(ctx context.Context, bucket, objName string, r io.Reader, bufferSize uint64) error {
	w := gcs.client.Bucket(bucket).Object(objName).NewWriter(ctx)
	defer w.Close()

	buf := make([]byte, bufferSize)
	if _, err := io.CopyBuffer(w, r, buf); err != nil {
		return err
	}
	return w.Close()
}

func (gcs *GcsExporter) composeParts(ctx context.Context, bucket string, objects []string, objectName string, totalSize datasize.ByteSize, opts *UploadOptions) error {

	if totalSize <= opts.SizeLimit {
		log.WithFields(map[string]interface{}{
			"parts": len(objects),
			"name":  fmt.Sprintf("%s.tar.gz", objectName),
		}).Info("composing final file")
		return gcs.composeIntoSingleObject(ctx, bucket, objects, fmt.Sprintf("%s.tar.gz", objectName), opts)
	}

	// Edge case: if chunk size == part size, just rename parts with .tar.gz extension.
	if totalSize > opts.SizeLimit && opts.ChunkSize == opts.PartSize {
		log.WithFields(map[string]interface{}{
			"parts":     len(objects),
			"part-size": opts.PartSize.HumanReadable(),
			"name":      fmt.Sprintf("%s-part-N.tar.gz", objectName),
		}).Info("composing final file parts")
		return gcs.renameToFinalNames(ctx, objects, bucket, objectName, opts)
	}

	partSize := opts.PartSize

	// Adjust part-size if needed
	if opts.PartSize%opts.ChunkSize != 0 {
		partSize = opts.PartSize + (opts.ChunkSize - opts.PartSize%opts.ChunkSize)
		log.WithFields(map[string]interface{}{
			"part-size": opts.PartSize,
			"new-size":  partSize,
		}).Warn("adjusting part size to be a multiple of chunk-size")
	}
	chunksPerPart := int(partSize / opts.ChunkSize)

	digits := getDigitCount(len(objects) / chunksPerPart)
	formatString := fmt.Sprintf("%%s-part-%%0%dd.tar.gz", digits)

	log.WithFields(map[string]interface{}{
		"parts":     len(objects) / chunksPerPart,
		"part-size": partSize.HumanReadable(),
		"name":      fmt.Sprintf("%s-part-N.tar.gz", objectName),
	}).Info("composing final file parts")

	for i := 0; i < len(objects); i += chunksPerPart {
		partName := fmt.Sprintf(formatString, objectName, i/chunksPerPart)
		end := min(i+chunksPerPart, len(objects))
		part := make([]string, len(objects[i:end]))
		copy(part, objects[i:end])

		if err := gcs.composeIntoSingleObject(ctx, bucket, part, partName, opts); err != nil {
			return err
		}
	}

	return nil
}

func (gcs *GcsExporter) renameToFinalNames(ctx context.Context, objects []string, bucket, objectName string, opts *UploadOptions) error {
	log.Infof("renaming %d objects to have tar.gz extension", len(objects))

	if len(objects) == 0 {
		return nil
	}

	if len(objects) == 1 {
		return gcs.renameObject(ctx, bucket, objectName, fmt.Sprintf("%s.tar.gz", objectName))
	}

	digits := getDigitCount(len(objects) - 1)
	formatString := fmt.Sprintf("%%s-part-%%0%dd.tar.gz", digits)

	for i, object := range objects {
		finalName := fmt.Sprintf(formatString, objectName, i)
		if err := gcs.renameObject(ctx, bucket, object, finalName); err != nil {
			return err
		}
	}
	return nil
}

func (gcs *GcsExporter) renameObject(ctx context.Context, bucket, oldName, newName string) error {
	log.Debugf("renaming %s to %s", oldName, newName)
	src := gcs.client.Bucket(bucket).Object(oldName)
	if _, err := gcs.client.Bucket(bucket).Object(newName).CopierFrom(src).Run(ctx); err != nil {
		return fmt.Errorf("failed to rename object %s -> %s: %v", oldName, oldName, err)
	}
	return src.Delete(ctx)
}

func (gcs *GcsExporter) composeIntoSingleObject(ctx context.Context, bucket string, objects []string, objectName string, opts *UploadOptions) error {
	log.Debugf("composing %d objects into %s", len(objects), objectName)

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, opts.ConcurrentJobs) // Limit concurrent compositions

	// While there are more than 32 parts, keep composing
	round := 0
	for len(objects) > CompositionBatchLimit {
		numBatches := (len(objects) + 31) / CompositionBatchLimit
		newParts := make([]string, numBatches)
		partMutex := sync.Mutex{}          // Protect newParts slice
		batchObjectsToDelete := []string{} // Keep track of intermediate batch files

		for i := 0; i < len(objects); i += CompositionBatchLimit {
			semaphore <- struct{}{}
			wg.Add(1)

			end := min(i+CompositionBatchLimit, len(objects))
			batch := make([]string, len(objects[i:end]))
			copy(batch, objects[i:end])

			go func(batchIndex int, batch []string, totalBatches int) {
				defer wg.Done()
				defer func() { <-semaphore }()

				tempObjectName := fmt.Sprintf("%s-temp-%d-%d", objectName, round, batchIndex)

				log.WithFields(map[string]interface{}{
					"batch":      batchIndex,
					"total":      totalBatches,
					"object":     tempObjectName,
					"batch-size": len(batch),
				}).Debug("composing batch")
				tempObject := gcs.client.Bucket(bucket).Object(tempObjectName)
				composer := tempObject.ComposerFrom(objectsToHandles(gcs.client, bucket, batch)...)

				if _, err := composer.Run(ctx); err != nil {
					log.Errorf("failed to compose batch %d: %v", batchIndex, err)
					return
				}

				// Safely append new composed part
				partMutex.Lock()
				newParts[batchIndex] = tempObjectName
				batchObjectsToDelete = append(batchObjectsToDelete, batch...) // Add batch parts for deletion
				partMutex.Unlock()
			}(i/CompositionBatchLimit, batch, len(objects)/CompositionBatchLimit)
		}

		wg.Wait()
		log.Debugf("finished composing a round, %d parts remain", len(newParts))

		// Delete all composed intermediate parts from previous batch
		if err := gcs.batchDelete(ctx, bucket, batchObjectsToDelete, opts.ConcurrentJobs*10); err != nil {
			log.Warnf("failed to delete intermediate parts: %v", err)
		}
		batchObjectsToDelete = []string{}

		// Move to next round
		objects = newParts
		round += 1
	}

	finalObject := gcs.client.Bucket(bucket).Object(objectName)
	composer := finalObject.ComposerFrom(objectsToHandles(gcs.client, bucket, objects)...)
	if _, err := composer.Run(ctx); err != nil {
		return fmt.Errorf("failed to compose final object: %v", err)
	}

	// Cleanup final composed parts
	return gcs.batchDelete(ctx, bucket, objects, opts.ConcurrentJobs*10)
}

func objectsToHandles(client *storage.Client, bucket string, names []string) []*storage.ObjectHandle {
	handles := make([]*storage.ObjectHandle, len(names))
	for i, name := range names {
		handles[i] = client.Bucket(bucket).Object(name)
	}
	return handles
}

func (gcs *GcsExporter) batchDelete(ctx context.Context, bucket string, objectNames []string, batchSize int) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(objectNames))
	semaphore := make(chan struct{}, batchSize)

	for _, objName := range objectNames {
		wg.Add(1)
		semaphore <- struct{}{}

		go func(obj string) {
			defer wg.Done()
			defer func() { <-semaphore }()

			log.WithFields(map[string]interface{}{
				"object": obj,
				"bucket": bucket,
			}).Debug("deleting object")
			err := gcs.client.Bucket(bucket).Object(obj).Delete(ctx)
			if err != nil {
				errChan <- fmt.Errorf("failed to delete %s: %v", obj, err)
			}
		}(objName)
	}

	wg.Wait()
	close(errChan)

	// Collect errors
	var errList []error
	for err := range errChan {
		errList = append(errList, err)
	}
	if len(errList) > 0 {
		return fmt.Errorf("some deletions failed: %v", errList)
	}

	return nil
}

func (gcs *GcsExporter) Delete(bucket, name string, opts ...DeleteOption) error {
	options := defaultDeleteOptions()
	for _, opt := range opts {
		opt(options)
	}

	ctx := context.Background()
	// List objects that match the prefix
	var objectNames []string
	it := gcs.client.Bucket(bucket).Objects(ctx, &storage.Query{Prefix: name})

	for {
		objAttrs, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to list objects: %v", err)
		}
		objectNames = append(objectNames, objAttrs.Name)
	}

	if len(objectNames) == 0 {
		log.Warnf("no objects found with prefix: %s", name)
		return nil
	}

	log.WithFields(map[string]interface{}{
		"bucket":  bucket,
		"name":    name,
		"objects": len(objectNames),
	}).Infof("deleting object(s) with name(prefix): %s", name)
	return gcs.batchDelete(ctx, bucket, objectNames, options.ConcurrentJobs)
}
