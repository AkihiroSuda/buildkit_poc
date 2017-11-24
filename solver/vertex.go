package solver

import (
	"sync"
	"time"

	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/progress"
	digest "github.com/opencontainers/go-digest"
	"golang.org/x/net/context"
)

// Vertex is one node in the build graph
type Vertex interface {
	// Digest is a content-addressable vertex identifier
	Digest() digest.Digest
	// Sys returns an internal value that is used to execute the vertex. Usually
	// this is capured by the operation resolver method during solve.
	Sys() interface{}
	// FIXME(AkihiroSuda): we should not import pb pkg here.
	Metadata() *pb.OpMetadata
	// Array of vertexes current vertex depends on.
	Inputs() []Input
	Name() string // change this to general metadata
}

type Index int

// Input is an pointer to a single reference from a vertex by an index.
type Input struct {
	Index  Index
	Vertex Vertex
}

type input struct {
	index  Index
	vertex *vertex
}

type vertex struct {
	mu           sync.Mutex
	sys          interface{}
	metadata     *pb.OpMetadata
	inputs       []*input
	err          error
	digest       digest.Digest
	clientVertex client.Vertex
	name         string
	notifyMu     sync.Mutex
}

func (v *vertex) initClientVertex() {
	inputDigests := make([]digest.Digest, 0, len(v.inputs))
	for _, inp := range v.inputs {
		inputDigests = append(inputDigests, inp.vertex.Digest())
	}
	v.clientVertex = client.Vertex{
		Inputs: inputDigests,
		Name:   v.Name(),
		Digest: v.digest,
	}
}

func (v *vertex) Digest() digest.Digest {
	return v.digest
}

func (v *vertex) Sys() interface{} {
	return v.sys
}

func (v *vertex) Metadata() *pb.OpMetadata {
	return v.metadata
}

func (v *vertex) Inputs() (inputs []Input) {
	inputs = make([]Input, 0, len(v.inputs))
	for _, i := range v.inputs {
		inputs = append(inputs, Input{i.index, i.vertex})
	}
	return
}

func (v *vertex) Name() string {
	return v.name
}

func notifyStarted(ctx context.Context, v *client.Vertex) {
	pw, _, _ := progress.FromContext(ctx)
	defer pw.Close()
	now := time.Now()
	v.Started = &now
	v.Completed = nil
	pw.Write(v.Digest.String(), *v)
}

func notifyCompleted(ctx context.Context, v *client.Vertex, err error) {
	pw, _, _ := progress.FromContext(ctx)
	defer pw.Close()
	now := time.Now()
	if v.Started == nil {
		v.Started = &now
	}
	v.Completed = &now
	v.Cached = false
	if err != nil {
		v.Error = err.Error()
	}
	pw.Write(v.Digest.String(), *v)
}

func inVertexContext(ctx context.Context, name string, f func(ctx context.Context) error) error {
	v := client.Vertex{
		Digest: digest.FromBytes([]byte(identity.NewID())),
		Name:   name,
	}
	pw, _, ctx := progress.FromContext(ctx, progress.WithMetadata("vertex", v.Digest))
	notifyStarted(ctx, &v)
	defer pw.Close()
	err := f(ctx)
	notifyCompleted(ctx, &v, err)
	return err
}
