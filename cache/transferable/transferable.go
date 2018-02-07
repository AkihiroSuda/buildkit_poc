package transferable

import (
	"github.com/containerd/containerd/images"
	"github.com/docker/distribution/manifest"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// MediaTypeManifestList is for ManifestList JSON
// TODO: use OCI or our own media type?
const MediaTypeManifestList = images.MediaTypeDockerSchema2ManifestList

// MediaTypeCacheConfig is for CacheConfig JSON
const MediaTypeCacheConfig = "application/vnd.buildkit.cacheconfig.v0"

// ManifestList holds CacheConfig manifests.
// This is our own type because oci type can't be pushed and docker type doesn't have annotations.
// MediaType is expected to be MediaTypeManifestList
type ManifestList struct {
	// SchemaVersion = 2
	manifest.Versioned

	// Manifests references platform specific manifests.
	// MediaType is expected to be MediaTypeCacheConfig
	Manifests []ocispec.Descriptor `json:"manifests"`
}

// CacheConfig is used for transferable cache objects.
// The media type is MediaTypeCacheConfig
type CacheConfig struct {
	Items []ConfigItem
}

// ConfigItem is an item in CacheConfig
type ConfigItem struct {
	// The content referenced by Blobsum has schema2.MediaTypeLayer media type.
	Blobsum    digest.Digest
	CacheKey   digest.Digest
	ContentKey digest.Digest
	Parent     digest.Digest
	DiffID     digest.Digest
}
