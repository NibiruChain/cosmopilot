package datasnapshot

import (
	"context"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
)

type SnapshotStatus string

const (
	SnapshotSucceeded SnapshotStatus = "succeeded"
	SnapshotFailed    SnapshotStatus = "failed"
	SnapshotActive    SnapshotStatus = "active"
	SnapshotNotFound  SnapshotStatus = "notfound"

	labelExporter = "exporter"
	labelOwner    = "owner"
	labelType     = "type"

	typeUpload = "upload"
	typeDelete = "delete"
)

type SnapshotProvider interface {
	CreateSnapshot(context.Context, string, *snapshotv1.VolumeSnapshot) error
	GetSnapshotStatus(context.Context, string) (SnapshotStatus, error)
	DeleteSnapshot(context.Context, string) error
	ListSnapshots(ctx context.Context) ([]string, error)
}
