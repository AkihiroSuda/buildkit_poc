package contentstore

import (
	"context"

	"github.com/containerd/containerd/content"
	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/cache/instructioncache"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// Transfer interface may change. (#224)
type Transfer struct {
	ContentProvider content.Provider
	Digest          digest.Digest // digest of manifest list blob
}

type Importer interface {
	Import(ctx context.Context, tr Transfer) (instructioncache.InstructionCache, error)
}

// ContentEnsure ensures the blob to be available in the store.
// Expected to be used with a registry.
type ContentEnsurer func(context.Context, content.Store, ocispec.Descriptor) error

type EnsurableImporter interface {
	Importer
	ImportWithContentEnsurer(ctx context.Context, tr Transfer, ensurer ContentEnsurer) (instructioncache.InstructionCache, error)
}

type CacheRecord struct {
	CacheKey   digest.Digest
	Reference  cache.ImmutableRef
	ContentKey digest.Digest
}

type Exporter interface {
	Export(ctx context.Context, rec []CacheRecord) (*Transfer, error)
}
