package transferable

import (
	"github.com/moby/buildkit/cache"
	digest "github.com/opencontainers/go-digest"
)

// CacheRecord is used as an argument for Exporter implementations.
type CacheRecord struct {
	CacheKey   digest.Digest
	Reference  cache.ImmutableRef
	ContentKey digest.Digest
}

// No common interface for exporters now..
// TODO: use "Resolver" pattern?
