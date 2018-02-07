package contentstore

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/diff"
	"github.com/docker/distribution/manifest/schema2"
	"github.com/moby/buildkit/cache/blobs"
	"github.com/moby/buildkit/cache/transferable"
	"github.com/moby/buildkit/snapshot"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type ExporterOpt struct {
	Snapshotter  snapshot.Snapshotter
	ContentStore content.Store
	Differ       diff.Differ
}

func NewExporter(opt ExporterOpt) Exporter {
	return &exporter{opt: opt}
}

type exporter struct {
	opt ExporterOpt
}

// Export returns an OCI descriptor for transferable.ManifestList.
func (ce *exporter) Export(ctx context.Context, rec []CacheRecord) (*Transfer, error) {
	allBlobs := map[digest.Digest][]blobs.DiffPair{}
	currentBlobs := map[digest.Digest]struct{}{}
	type cr struct {
		CacheRecord
		dgst digest.Digest
	}

	list := make([]cr, 0, len(rec))

	for _, r := range rec {
		ref := r.Reference
		if ref == nil {
			list = append(list, cr{CacheRecord: r})
			continue
		}

		dpairs, err := blobs.GetDiffPairs(ctx, ce.opt.ContentStore, ce.opt.Snapshotter, ce.opt.Differ, ref)
		if err != nil {
			return nil, err
		}

		for i, dp := range dpairs {
			allBlobs[dp.Blobsum] = dpairs[:i+1]
		}

		dgst := dpairs[len(dpairs)-1].Blobsum
		list = append(list, cr{CacheRecord: r, dgst: dgst})
		currentBlobs[dgst] = struct{}{}
	}

	for b := range allBlobs {
		if _, ok := currentBlobs[b]; !ok {
			list = append(list, cr{dgst: b})
		}
	}

	var config transferable.CacheConfig

	var mfst transferable.ManifestList
	mfst.SchemaVersion = 2
	mfst.MediaType = transferable.MediaTypeManifestList

	for _, l := range list {
		var size int64
		var parent digest.Digest
		var diffID digest.Digest
		if l.dgst != "" {
			info, err := ce.opt.ContentStore.Info(ctx, l.dgst)
			if err != nil {
				return nil, err
			}
			size = info.Size
			chain := allBlobs[l.dgst]
			if len(chain) > 1 {
				parent = chain[len(chain)-2].Blobsum
			}
			diffID = chain[len(chain)-1].DiffID

			mfst.Manifests = append(mfst.Manifests, ocispec.Descriptor{
				MediaType: schema2.MediaTypeLayer,
				Size:      size,
				Digest:    l.dgst,
			})
		}

		config.Items = append(config.Items, transferable.ConfigItem{
			Blobsum:    l.dgst,
			CacheKey:   l.CacheKey,
			ContentKey: l.ContentKey,
			Parent:     parent,
			DiffID:     diffID,
		})
	}

	dt, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	dgst := digest.FromBytes(dt)

	addAsRoot := content.WithLabels(map[string]string{
		"containerd.io/gc.root": time.Now().UTC().Format(time.RFC3339Nano),
	})

	if err := content.WriteBlob(ctx, ce.opt.ContentStore, dgst.String(), bytes.NewReader(dt), int64(len(dt)), dgst, addAsRoot); err != nil {
		return nil, errors.Wrap(err, "error writing config blob")
	}

	mfst.Manifests = append(mfst.Manifests, ocispec.Descriptor{
		MediaType: transferable.MediaTypeCacheConfig,
		Size:      int64(len(dt)),
		Digest:    dgst,
	})

	dt, err = json.Marshal(mfst)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal manifest")
	}

	dgst = digest.FromBytes(dt)

	if err := content.WriteBlob(ctx, ce.opt.ContentStore, dgst.String(), bytes.NewReader(dt), int64(len(dt)), dgst, addAsRoot); err != nil {
		return nil, errors.Wrap(err, "error writing manifest blob")
	}

	logrus.Debugf("cache-manifest: %s", dgst)

	return &Transfer{
		ContentProvider: ce.opt.ContentStore,
		Digest:          dgst,
	}, nil
}
