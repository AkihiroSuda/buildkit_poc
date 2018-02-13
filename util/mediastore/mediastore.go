// Package mediastore provides wrapper for contentstore that requires media type string
package mediastore

import (
	"context"

	"github.com/containerd/containerd/content"
	"github.com/opencontainers/go-digest"
)

type MediaTypeMapper interface {
	SetMediaType(digest.Digest, string)
	ReleaseMediaType(digest.Digest)
}

type Ensurer interface {
	Ensure(ctx context.Context, dgst digest.Digest, off, len int64) error
}

type Ingester interface {
	content.Ingester
	MediaTypeMapper
}

type Provider interface {
	content.Provider
	MediaTypeMapper
}
