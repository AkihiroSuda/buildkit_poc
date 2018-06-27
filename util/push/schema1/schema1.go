package schema1

import (
	"context"
	"encoding/json"
	"runtime"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/remotes"
	"github.com/docker/distribution/reference"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func PushAsSchema1(ctx context.Context, pusher remotes.Pusher, provider content.Provider, desc ocispec.Descriptor, ref string) error {
	switch desc.MediaType {
	case images.MediaTypeDockerSchema2Manifest, ocispec.MediaTypeImageManifest:
	default:
		return errors.Wrapf(errdefs.ErrInvalidArgument, "mediatype cannot be converted to schema1 manifest: %s", desc.MediaType)
	}
	p, err := content.ReadBlob(ctx, provider, desc)
	if err != nil {
		return err
	}

	// TODO(stevvooe): We just assume oci manifest, for now. There may be
	// subtle differences from the docker version.
	var m ocispec.Manifest
	if err := json.Unmarshal(p, &m); err != nil {
		return err
	}
	mediaType := images.MediaTypeDockerSchema1ManifestUnsigned
	m1, err := convertToSchema1(m, mediaType, ref)
	if err != nil {
		return err
	}
	m1JSON, err := json.Marshal(m1)
	if err != nil {
		return err
	}
	logrus.Debugf("Generated schema1 manifest: %q", string(m1JSON))
	// NOTE: m1 is not written to the local content store
	m1Desc := ocispec.Descriptor{
		MediaType: mediaType,
		Size:      int64(len(m1JSON)),
		Digest:    digest.SHA256.FromBytes(m1JSON),
	}
	w, err := pusher.Push(ctx, m1Desc)
	if err != nil {
		return err
	}
	defer w.Close()
	if _, err := w.Write(m1JSON); err != nil {
		return err
	}
	return w.Commit(ctx, m1Desc.Size, m1Desc.Digest)
}

type FSLayer struct {
	BlobSum digest.Digest `json:"blobSum"`
}

type History struct {
	V1Compatibility string `json:"v1Compatibility"`
}

type Manifest struct {
	SchemaVersion int       `json:"schemaVersion"`
	MediaType     string    `json:"mediaType,omitempty"`
	Name          string    `json:"name"`
	Tag           string    `json:"tag"`
	Architecture  string    `json:"architecture"`
	FSLayers      []FSLayer `json:"fsLayers"`
	History       []History `json:"history"`
}

func convertToSchema1(m ocispec.Manifest, schema1MediaType, ref string) (*Manifest, error) {
	named, err := reference.ParseNamed(ref)
	if err != nil {
		return nil, err
	}
	tag := ""
	if namedTagged, ok := named.(reference.NamedTagged); ok {
		tag = namedTagged.Tag()
	}
	arch := runtime.GOARCH
	// TODO: propagate architecture from the manifest
	m1 := &Manifest{
		SchemaVersion: 1,
		Name:          named.Name(),
		Tag:           tag,
		Architecture:  arch,
		// TODO: set History
	}
	for _, d := range m.Layers {
		m1.FSLayers = append(m1.FSLayers, FSLayer{BlobSum: d.Digest})
	}
	return m1, nil
}
