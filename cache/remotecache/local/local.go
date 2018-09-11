package local

import (
	"context"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/reference"
	"github.com/moby/buildkit/cache/remotecache"
	"github.com/moby/buildkit/session"
	sessioncontent "github.com/moby/buildkit/session/content"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// ResolveCacheExporterFunc for "local" cache exporter.
// ref is ignored at the moment
func ResolveCacheExporterFunc(sm *session.Manager) remotecache.ResolveCacheExporterFunc {
	return func(ctx context.Context, typ, ref string) (remotecache.Exporter, error) {
		if typ != "local" {
			return nil, errors.Errorf("unsupported cache exporter type: %s", typ)
		}
		cs, err := getContentStore(ctx, sm)
		if err != nil {
			return nil, err
		}
		return remotecache.NewExporter(cs), nil
	}
}

// ResolveCacheImporterFunc for "local" cache importer.
// ref must be like "tag@sha256:deadbeef" (tag is ignored at the moment)
func ResolveCacheImporterFunc(sm *session.Manager) remotecache.ResolveCacheImporterFunc {
	return func(ctx context.Context, typ, ref string) (remotecache.Importer, specs.Descriptor, error) {
		if typ != "local" {
			return nil, specs.Descriptor{}, errors.Errorf("unsupported cache importer type: %s", typ)
		}
		_, dgst := reference.SplitObject(ref)
		if dgst == "" {
			return nil, specs.Descriptor{}, errors.New("local cache importer requires ref to contain explicit digest")
		}
		cs, err := getContentStore(ctx, sm)
		if err != nil {
			return nil, specs.Descriptor{}, err
		}
		info, err := cs.Info(ctx, dgst)
		if err != nil {
			return nil, specs.Descriptor{}, err
		}
		desc := specs.Descriptor{
			// MediaType is typically MediaTypeDockerSchema2ManifestList,
			// but we leave it empty until we get correct support for local index.json
			Digest: dgst,
			Size: info.Size,
		}
		return remotecache.NewImporter(cs), desc, nil
	}
}

func getContentStore(ctx context.Context, sm *session.Manager) (content.Store, error) {
	id := session.FromContext(ctx)
	if id == "" {
		return nil, errors.New("local cache exporter/importer requires session")
	}
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	caller, err := sm.Get(timeoutCtx, id)
	if err != nil {
		return nil, err
	}
	return sessioncontent.CallerStore(caller), nil
}
