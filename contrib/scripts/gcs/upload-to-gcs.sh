#!/bin/bash
set -e  # Exit on error
set -o pipefail  # Catch errors in pipes

# Arguments
DATA_DIR=${1:-"data"}         # Directory to upload
BUCKET=${2}                   # Name of GCS bucket (no "gs://", just the bucket name)
NAME=${3:-"$DATA_DIR"}        # Base name for archive (e.g. "backup", "snapshot", etc.)

# Environment / default variables
CHUNK_SIZE=${CHUNK_SIZE:-"500GB"}
CREDENTIALS_FILE=${CREDENTIALS_FILE:-"/creds/credentials.json"}

# Make sure bucket is specified
if [ -z "$BUCKET" ]; then
  echo "Usage: $0 <data_dir> <bucket> [base_name]"
  exit 1
fi

echo "Authenticating with GCP..."
gcloud auth activate-service-account --key-file="$CREDENTIALS_FILE"

echo "Preparing to upload directory: $DATA_DIR"

# Calculate total size (in bytes)
SIZE=$(du -sb "$DATA_DIR" | awk '{print $1}')
LIMIT=$((5 * 1024 * 1024 * 1024 * 1024))  # 5TB in bytes

if [ "$SIZE" -gt "$LIMIT" ]; then
  echo "Data size ($SIZE bytes) exceeds 5TB limit for a single GCS object."
  echo "Splitting into chunks of $CHUNK_SIZE each..."

  tar cf - "$DATA_DIR" \
      | pv -s "$SIZE" \
      | pigz -1 \
      | split \
          --bytes="$CHUNK_SIZE" \
          --numeric-suffixes=0 \
          -d \
          -a 3 \
          --additional-suffix=.tar.gz \
          --filter='
            echo "Uploading $FILE..."
            pv | gsutil -o "GSUtil:parallel_composite_upload_threshold=150M" cp - "gs://'"$BUCKET"'/$FILE"
          ' \
          - "${NAME}-part-"

else
  echo "Data size ($SIZE bytes) is within the 5TB limit."
  echo "Performing single archive upload to gs://$BUCKET/$NAME.tar.gz..."

  tar cf - "$DATA_DIR" \
    | pv -s "$SIZE" \
    | pigz -1 \
    | gsutil -o "GSUtil:parallel_composite_upload_threshold=150M" cp - "gs://$BUCKET/$NAME.tar.gz"
fi

echo "Upload completed successfully!"