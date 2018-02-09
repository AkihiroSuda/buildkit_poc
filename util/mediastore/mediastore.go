// Package mediastore provides wrapper for contentstore that requires media type string
package mediastore

import (
	"github.com/containerd/containerd/content"
	"github.com/opencontainers/go-digest"
)

type MediaTypeMapper interface {
	SetMediaType(digest.Digest, string)
	ReleaseMediaType(digest.Digest)
}

type Ingester interface {
	content.Ingester
	MediaTypeMapper
}

type Provider interface {
	content.Provider
	MediaTypeMapper
	// TODO: add blob size mapper?
}
