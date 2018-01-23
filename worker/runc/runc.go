package runc

import (
	"context"
	"os"
	"path/filepath"

	"github.com/boltdb/bolt"
	"github.com/containerd/containerd/content/local"
	"github.com/containerd/containerd/diff/walking"
	ctdmetadata "github.com/containerd/containerd/metadata"
	ctdsnapshot "github.com/containerd/containerd/snapshots"
	"github.com/containerd/containerd/snapshots/naive"
	"github.com/containerd/containerd/snapshots/overlay"
	"github.com/moby/buildkit/cache/metadata"
	"github.com/moby/buildkit/executor/runcexecutor"
	containerdsnapshot "github.com/moby/buildkit/snapshot/containerd"
	"github.com/moby/buildkit/worker/base"
)

// NewWorkerOpt creates a WorkerOpt.
// But it does not set the following fields:
//  - SessionManager
func NewWorkerOpt(root string, labels map[string]string, overlayfs bool, exeOpt *runcexecutor.Opt) (base.WorkerOpt, error) {
	snapshotterName := "naive"
	snapshotterNew := naive.NewSnapshotter
	if overlayfs {
		snapshotterName = "overlayfs"
		snapshotterNew = overlay.NewSnapshotter
	}
	var opt base.WorkerOpt
	name := "runc-" + snapshotterName
	root = filepath.Join(root, name)
	if err := os.MkdirAll(root, 0700); err != nil {
		return opt, err
	}
	md, err := metadata.NewStore(filepath.Join(root, "metadata.db"))
	if err != nil {
		return opt, err
	}
	exe, err := runcexecutor.New(filepath.Join(root, "executor"), exeOpt)
	if err != nil {
		return opt, err
	}
	s, err := snapshotterNew(filepath.Join(root, "snapshots"))
	if err != nil {
		return opt, err
	}

	c, err := local.NewStore(filepath.Join(root, "content"))
	if err != nil {
		return opt, err
	}

	db, err := bolt.Open(filepath.Join(root, "containerdmeta.db"), 0644, nil)
	if err != nil {
		return opt, err
	}

	mdb := ctdmetadata.NewDB(db, c, map[string]ctdsnapshot.Snapshotter{
		snapshotterName: s,
	})
	if err := mdb.Init(context.TODO()); err != nil {
		return opt, err
	}

	gc := func(ctx context.Context) error {
		_, err := mdb.GarbageCollect(ctx)
		return err
	}

	c = containerdsnapshot.NewContentStore(mdb.ContentStore(), "buildkit", gc)
	df, err := walking.NewWalkingDiff(c)
	if err != nil {
		return opt, err
	}

	id, err := base.ID(root)
	if err != nil {
		return opt, err
	}
	xlabels := base.Labels("oci", snapshotterName)
	for k, v := range labels {
		xlabels[k] = v
	}
	opt = base.WorkerOpt{
		ID:            id,
		Labels:        xlabels,
		MetadataStore: md,
		Executor:      exe,
		Snapshotter:   containerdsnapshot.NewSnapshotter(mdb.Snapshotter(snapshotterName), c, md, "buildkit", gc),
		ContentStore:  c,
		Applier:       df,
		Differ:        df,
		ImageStore:    nil, // explicitly
	}
	return opt, nil
}
