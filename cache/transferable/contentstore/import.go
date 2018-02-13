package contentstore

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/diff"
	"github.com/containerd/containerd/rootfs"
	cdsnapshot "github.com/containerd/containerd/snapshots"
	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/cache/blobs"
	"github.com/moby/buildkit/cache/instructioncache"
	"github.com/moby/buildkit/cache/transferable"
	"github.com/moby/buildkit/client"
	buildkitidentity "github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/snapshot"
	"github.com/moby/buildkit/util/mediastore"
	"github.com/moby/buildkit/util/progress"
	digest "github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/identity"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

type ImporterOpt struct {
	Snapshotter   snapshot.Snapshotter
	Applier       diff.Differ
	CacheAccessor cache.Accessor
}

func NewImporter(opt ImporterOpt) Importer {
	return &importer{opt: opt}
}

type importer struct {
	opt ImporterOpt
}

func (ci *importer) Import(ctx context.Context, tr Transfer) (instructioncache.InstructionCache, error) {
	dt, err := content.ReadBlob(ctx, tr.ContentProvider, tr.Digest)
	if err != nil {
		return nil, err
	}

	var mfst ocispec.Index
	if err := json.Unmarshal(dt, &mfst); err != nil {
		return nil, err
	}

	allDesc := map[digest.Digest]ocispec.Descriptor{}
	allBlobs := map[digest.Digest]transferable.ConfigItem{}
	byCacheKey := map[digest.Digest]transferable.ConfigItem{}
	byContentKey := map[digest.Digest][]digest.Digest{}

	var configDesc ocispec.Descriptor

	for _, m := range mfst.Manifests {
		if m.MediaType == transferable.MediaTypeCacheConfig {
			configDesc = m
			continue
		}
		allDesc[m.Digest] = m
	}

	if configDesc.Digest == "" {
		return nil, errors.Errorf("invalid build cache: %s", tr.Digest)
	}

	mtm, ok := tr.ContentProvider.(mediastore.MediaTypeMapper)
	if ok {
		mtm.SetMediaType(configDesc.Digest, configDesc.MediaType)
	}

	dt, err = content.ReadBlob(ctx, tr.ContentProvider, configDesc.Digest)
	if err != nil {
		return nil, err
	}

	var cfg transferable.CacheConfig
	if err := json.Unmarshal(dt, &cfg); err != nil {
		return nil, err
	}

	for _, ci := range cfg.Items {
		if ci.Blobsum != "" {
			allBlobs[ci.Blobsum] = ci
		}
		if ci.CacheKey != "" {
			byCacheKey[ci.CacheKey] = ci
			if ci.ContentKey != "" {
				byContentKey[ci.ContentKey] = append(byContentKey[ci.ContentKey], ci.CacheKey)
			}
		}
	}

	return &importInfo{
		importer:     ci,
		byCacheKey:   byCacheKey,
		byContentKey: byContentKey,
		allBlobs:     allBlobs,
		allDesc:      allDesc,
		description:  "content " + string(tr.Digest),
		provider:     tr.ContentProvider,
		ensurer:      ensurer,
	}, nil
}

type importInfo struct {
	*importer
	byCacheKey   map[digest.Digest]transferable.ConfigItem
	byContentKey map[digest.Digest][]digest.Digest
	allDesc      map[digest.Digest]ocispec.Descriptor
	allBlobs     map[digest.Digest]transferable.ConfigItem
	description  string // just human-readable
	provider     content.Provider
}

func (ii *importInfo) Probe(ctx context.Context, key digest.Digest) (bool, error) {
	_, ok := ii.byCacheKey[key]
	return ok, nil
}

func (ii *importInfo) getChain(dgst digest.Digest) ([]blobs.DiffPair, error) {
	cfg, ok := ii.allBlobs[dgst]
	if !ok {
		return nil, errors.Errorf("blob %s not found in cache", dgst)
	}
	parent := cfg.Parent

	var out []blobs.DiffPair
	if parent != "" {
		parentChain, err := ii.getChain(parent)
		if err != nil {
			return nil, err
		}
		out = parentChain
	}
	return append(out, blobs.DiffPair{Blobsum: dgst, DiffID: cfg.DiffID}), nil
}

func (ii *importInfo) Lookup(ctx context.Context, key digest.Digest, msg string) (interface{}, error) {
	desc, ok := ii.byCacheKey[key]
	if !ok || desc.Blobsum == "" {
		return nil, nil
	}
	var out interface{}
	if err := inVertexContext(ctx, fmt.Sprintf("cache from %s for %s", ii.description, msg), func(ctx context.Context) error {

		ch, err := ii.getChain(desc.Blobsum)
		if err != nil {
			return err
		}
		res, err := ii.fetch(ctx, ch)
		if err != nil {
			return err
		}
		out = res
		return nil
	}); err != nil {
		return nil, err
	}
	return out, nil
}

func (ii *importInfo) Set(key digest.Digest, ref interface{}) error {
	return nil
}

func (ii *importInfo) SetContentMapping(contentKey, key digest.Digest) error {
	return nil
}

func (ii *importInfo) GetContentMapping(dgst digest.Digest) ([]digest.Digest, error) {
	dgsts, ok := ii.byContentKey[dgst]
	if !ok {
		return nil, nil
	}
	return dgsts, nil
}

func (ii *importInfo) fetch(ctx context.Context, chain []blobs.DiffPair) (cache.ImmutableRef, error) {
	mtm, ok := ii.provider.(mediastore.MediaTypeMapper)
	if ok {
		eg, ctx := errgroup.WithContext(ctx)
		for _, dp := range chain {
			func(dp blobs.DiffPair) {
				eg.Go(func() error {
					desc, ok := ii.allDesc[dp.Blobsum]
					if !ok {
						return errors.Errorf("failed to find %s for fetch", dp.Blobsum)
					}
					mtm.SetMediaType(desc.Digest, desc.MediaType)
					// FIXME(AkihiroSuda): this bad hack ensures the content to be fetched.
					// Note that the read cache store needs to be the differ store.
					if _, err := ii.provider.ReaderAt(ctx, desc.Digest); err != nil {
						return err
					}
					mtm.ReleaseMediaType(desc.Digest, desc.MediaType)
					return nil
				})
			}(dp)
		}
		if err := eg.Wait(); err != nil {
			return nil, err
		}

	}
	chainid, err := ii.unpack(ctx, chain)
	if err != nil {
		return nil, err
	}

	return ii.opt.CacheAccessor.Get(ctx, chainid, cache.WithDescription("imported cache")) // TODO: more descriptive name
}

func (ii *importInfo) unpack(ctx context.Context, dpairs []blobs.DiffPair) (string, error) {
	layers, err := ii.getLayers(ctx, dpairs)
	if err != nil {
		return "", err
	}

	var chain []digest.Digest
	for _, layer := range layers {
		labels := map[string]string{
			"containerd.io/uncompressed": layer.Diff.Digest.String(),
		}
		if _, err := rootfs.ApplyLayer(ctx, layer, chain, ii.opt.Snapshotter, ii.opt.Applier, cdsnapshot.WithLabels(labels)); err != nil {
			return "", err
		}
		chain = append(chain, layer.Diff.Digest)
	}
	chainID := identity.ChainID(chain)

	if err := ii.fillBlobMapping(ctx, layers); err != nil {
		return "", err
	}

	return string(chainID), nil
}

func (ii *importInfo) fillBlobMapping(ctx context.Context, layers []rootfs.Layer) error {
	var chain []digest.Digest
	for _, l := range layers {
		chain = append(chain, l.Diff.Digest)
		chainID := identity.ChainID(chain)
		if err := ii.opt.Snapshotter.SetBlob(ctx, string(chainID), l.Diff.Digest, l.Blob.Digest); err != nil {
			return err
		}
	}
	return nil
}

func (ii *importInfo) getLayers(ctx context.Context, dpairs []blobs.DiffPair) ([]rootfs.Layer, error) {
	layers := make([]rootfs.Layer, len(dpairs))
	for i := range dpairs {
		layers[i].Diff = ocispec.Descriptor{
			// TODO: derive media type from compressed type
			MediaType: ocispec.MediaTypeImageLayer,
			Digest:    dpairs[i].DiffID,
		}
		ra, err := ii.provider.ReaderAt(ctx, dpairs[i].Blobsum)
		if err != nil {
			return nil, err
		}
		layers[i].Blob = ocispec.Descriptor{
			// TODO: derive media type from compressed type
			MediaType: ocispec.MediaTypeImageLayerGzip,
			Digest:    dpairs[i].Blobsum,
			Size:      ra.Size(),
		}
	}
	return layers, nil
}

func inVertexContext(ctx context.Context, name string, f func(ctx context.Context) error) error {
	v := client.Vertex{
		Digest: digest.FromBytes([]byte(buildkitidentity.NewID())),
		Name:   name,
	}
	pw, _, ctx := progress.FromContext(ctx, progress.WithMetadata("vertex", v.Digest))
	notifyStarted(ctx, &v)
	defer pw.Close()
	err := f(ctx)
	notifyCompleted(ctx, &v, err)
	return err
}

func notifyStarted(ctx context.Context, v *client.Vertex) {
	pw, _, _ := progress.FromContext(ctx)
	defer pw.Close()
	now := time.Now()
	v.Started = &now
	v.Completed = nil
	pw.Write(v.Digest.String(), *v)
}

func notifyCompleted(ctx context.Context, v *client.Vertex, err error) {
	pw, _, _ := progress.FromContext(ctx)
	defer pw.Close()
	now := time.Now()
	if v.Started == nil {
		v.Started = &now
	}
	v.Completed = &now
	v.Cached = false
	if err != nil {
		v.Error = err.Error()
	}
	pw.Write(v.Digest.String(), *v)
}
