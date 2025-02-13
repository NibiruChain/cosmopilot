#!/bin/bash
set -e
set -o pipefail

BUCKET=$1
NAME=$2
CREDENTIALS_FILE=${CREDENTIALS_FILE:-"/creds/credentials.json"}

if [ -z "$BUCKET" ] || [ -z "$NAME" ]; then
  echo "Usage: $0 <bucket> <prefix>"
  exit 1
fi

echo "Authenticating with GCP..."
gcloud auth activate-service-account --key-file="$CREDENTIALS_FILE"

echo "Deleting gs://$BUCKET/$NAME*.tar.gz ..."
gsutil -m rm gs://"$BUCKET"/"$NAME"*.tar.gz

echo "Delete completed!"