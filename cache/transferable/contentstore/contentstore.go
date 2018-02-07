package contentstore

import (
	"context"

	"github.com/containerd/containerd/content"
	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/cache/instructioncache"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// ContentProvider is a read-only subset of content.Store
type ContentProvider interface {
	content.Provider
	Info(ctx context.Context, dgst digest.Digest) (content.Info, error)
}

// Transfer interface may change. (#224)
type Transfer struct {
	ContentProvider ContentProvider
	Digest          digest.Digest // digest of manifest list blob
}

type Importer interface {
	Import(ctx context.Context, tr Transfer) (instructioncache.InstructionCache, error)
}

// ContentEnsure ensures the blob to be available in the store.
// Expected to be used with a registry.
type ContentEnsurer func(context.Context, content.Store, ocispec.Descriptor) error

type EnsurableImporter interface {
	Import(ctx context.Context, tr Transfer) (instructioncache.InstructionCache, error)
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
