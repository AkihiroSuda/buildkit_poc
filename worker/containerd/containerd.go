package containerd

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/containerd/containerd"
	ctdintrospection "github.com/containerd/containerd/api/services/introspection/v1"
	"github.com/containerd/containerd/content"
	ctdplugin "github.com/containerd/containerd/plugin"
	"github.com/moby/buildkit/cache/metadata"
	"github.com/moby/buildkit/executor/containerdexecutor"
	"github.com/moby/buildkit/worker"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// TODO(AkihiroSuda): support setting containerd socket timeout
func NewWorkerOpts(root string, address string) ([]worker.WorkerOpt, error) {
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, errors.Wrapf(err, "failed to create %s", root)
	}

	// TODO: take lock to make sure there are no duplicates
	client, err := containerd.New(address, containerd.WithDefaultNamespace("buildkit"))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect client to %q . make sure containerd is running", address)
	}
	defaultCtd, err := newContainerd(root, client, containerd.DefaultSnapshotter)
	if err != nil {
		return nil, errors.Wrapf(err, "could not load the default containerd snapshotter %s", containerd.DefaultSnapshotter)
	}
	opts := []worker.WorkerOpt{defaultCtd}
	ps := client.IntrospectionService()
	response, err := ps.Plugins(context.TODO(), &ctdintrospection.PluginsRequest{})
	if err != nil {
		return nil, err
	}
	for _, plugin := range response.Plugins {
		if plugin.Type == ctdplugin.SnapshotPlugin.String() {
			if plugin.ID != containerd.DefaultSnapshotter {
				if plugin.InitErr != nil {
					logrus.Warnf("ignoring containerd snapshotter %s: %v", plugin.ID, plugin.InitErr)
					continue
				}
				opt, err := newContainerd(root, client, plugin.ID)
				if err != nil {
					logrus.Warnf("ignoring containerd snapshotter %s: %v", plugin.ID, err)
					continue
				}
				opts = append(opts, opt)
			}
		}
	}
	return opts, nil
}

func newContainerd(root string, client *containerd.Client, snapshotterName string) (worker.WorkerOpt, error) {
	name := "containerd-" + snapshotterName
	md, err := metadata.NewStore(filepath.Join(root, name+"-metadata.db"))
	if err != nil {
		return worker.WorkerOpt{}, err
	}
	df := client.DiffService()
	opt := worker.WorkerOpt{
		Name:            name,
		MetadataStore:   md,
		Executor:        containerdexecutor.New(client, root),
		BaseSnapshotter: client.SnapshotService(snapshotterName),
		ContentStore:    &noGCContentStore{client.ContentStore()},
		Applier:         df,
		Differ:          df,
		ImageStore:      client.ImageService(),
	}
	return opt, nil
}

// TODO: Replace this with leases

type noGCContentStore struct {
	content.Store
}
type noGCWriter struct {
	content.Writer
}

func (cs *noGCContentStore) Writer(ctx context.Context, ref string, size int64, expected digest.Digest) (content.Writer, error) {
	w, err := cs.Store.Writer(ctx, ref, size, expected)
	return &noGCWriter{w}, err
}

func (w *noGCWriter) Commit(ctx context.Context, size int64, expected digest.Digest, opts ...content.Opt) error {
	opts = append(opts, func(info *content.Info) error {
		if info.Labels == nil {
			info.Labels = map[string]string{}
		}
		info.Labels["containerd.io/gc.root"] = time.Now().UTC().Format(time.RFC3339Nano)
		return nil
	})
	return w.Writer.Commit(ctx, size, expected, opts...)
}
