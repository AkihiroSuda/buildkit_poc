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
	"github.com/docker/libtrust"
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
	mediaType := images.MediaTypeDockerSchema1Manifest
	m1, err := convertToSchema1(m, mediaType, ref)
	if err != nil {
		return err
	}
	m1JSON, err := json.Marshal(m1)
	if err != nil {
		return err
	}
	m1JSON, err = addDummyV2S1Signature(m1JSON)
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
		// NOTE: Most implementations do not set m1.MediaType here
		MediaType:    schema1MediaType,
		Name:         named.Name(),
		Tag:          tag,
		Architecture: arch,
	}
	// from https://github.com/docker/distribution/blob/master/docs/spec/manifest-v2-1.md
	dummyHistory := "{\"id\":\"e45a5af57b00862e5ef5782a9925979a02ba2b12dff832fd0991335f4a11e5c5\",\"parent\":\"31cbccb51277105ba3ae35ce33c22b69c9e3f1002e76e4c736a2e8ebff9d7b5d\",\"created\":\"2014-12-31T22:57:59.178729048Z\",\"container\":\"27b45f8fb11795b52e9605b686159729b0d9ca92f76d40fb4f05a62e19c46b4f\",\"container_config\":{\"Hostname\":\"8ce6509d66e2\",\"Domainname\":\"\",\"User\":\"\",\"Memory\":0,\"MemorySwap\":0,\"CpuShares\":0,\"Cpuset\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\"],\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) CMD [/hello]\"],\"Image\":\"31cbccb51277105ba3ae35ce33c22b69c9e3f1002e76e4c736a2e8ebff9d7b5d\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[],\"SecurityOpt\":null,\"Labels\":null},\"docker_version\":\"1.4.1\",\"config\":{\"Hostname\":\"8ce6509d66e2\",\"Domainname\":\"\",\"User\":\"\",\"Memory\":0,\"MemorySwap\":0,\"CpuShares\":0,\"Cpuset\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\"],\"Cmd\":[\"/hello\"],\"Image\":\"31cbccb51277105ba3ae35ce33c22b69c9e3f1002e76e4c736a2e8ebff9d7b5d\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[],\"SecurityOpt\":null,\"Labels\":null},\"architecture\":\"amd64\",\"os\":\"linux\",\"Size\":0}\n"
	for _, d := range m.Layers {
		// len(FSLayers) needs to be == len(History)
		m1.FSLayers = append(m1.FSLayers, FSLayer{BlobSum: d.Digest})
		m1.History = append(m1.History, History{V1Compatibility: dummyHistory})
	}
	return m1, nil
}

// addDummyV2S1Signature adds an JWS signature with a temporary key (i.e. useless) to a v2s1 manifest.
// This is useful to make the manifest acceptable to a Docker Registry (even though nothing needs or wants the JWS signature).
// Ported over from https://github.com/containers/image/blob/5144ced37a1b21b63c6ef605e56811e29a687528/manifest/manifest.go#L163-L179
func addDummyV2S1Signature(manifest []byte) ([]byte, error) {
	key, err := libtrust.GenerateECP256PrivateKey()
	if err != nil {
		return nil, err // Coverage: This can fail only if rand.Reader fails.
	}

	js, err := libtrust.NewJSONSignature(manifest)
	if err != nil {
		return nil, err
	}
	if err := js.Sign(key); err != nil { // Coverage: This can fail basically only if rand.Reader fails.
		return nil, err
	}
	return js.PrettySignature("signatures")
}
