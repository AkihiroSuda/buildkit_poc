// Package ocicontentstore provides additional interfaces for contentstore implementation
// that requires OCI media type strings. e.g. Docker registry.
//
// We may want to propose the regular containerd contentstore to take OCI descriptor,
// but let's incubate the interface design here for now.
package ocicontentstore

import (
	"context"

	"github.com/containerd/containerd/content"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type Ingester interface {
	content.Ingester
	WriterWithOCIDescriptor(ctx context.Context, ref string, desc ocispec.Descriptor) (content.Writer, error)
}

type Provider interface {
	content.Provider
	ReaderAtWithOCIDescriptor(ctx context.Context, desc ocispec.Descriptor) (content.ReaderAt, error)
}
