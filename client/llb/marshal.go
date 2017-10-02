package llb

import (
	"encoding/binary"
	"io"

	"github.com/moby/buildkit/solver/pb"
	digest "github.com/opencontainers/go-digest"
)

// Definition is the LLB definition structure with per-vertex metadata entries
// Corresponds to the Definition structure defined in solver/pb.Definition.
type Definition struct {
	Def      [][]byte
	Metadata map[digest.Digest]OpMetadata
}

func (def Definition) ToPB() *pb.Definition {
	md := make(map[digest.Digest]pb.OpMetadata)
	for k, v := range def.Metadata {
		md[k] = v.OpMetadata
	}
	return &pb.Definition{
		Def:      def.Def,
		Metadata: md,
	}
}

func (def *Definition) FromPB(x *pb.Definition) {
	def.Def = x.Def
	def.Metadata = make(map[digest.Digest]OpMetadata)
	for k, v := range x.Metadata {
		def.Metadata[k] = OpMetadata{v}
	}
}

type OpMetadata struct {
	pb.OpMetadata
}

func WriteTo(def Definition, w io.Writer) error {
	b, err := def.ToPB().Marshal()
	if err != nil {
		return err
	}
	l := make([]byte, 4)
	binary.LittleEndian.PutUint32(l, uint32(len(b)))
	if _, err := w.Write(l); err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

func ReadFrom(r io.Reader) (Definition, error) {
	var def Definition
	b := make([]byte, 4)
	if _, err := io.ReadFull(r, b); err != nil {
		return def, err
	}
	l := binary.LittleEndian.Uint32(b)
	b = make([]byte, l)
	if _, err := io.ReadFull(r, b); err != nil {
		return def, err
	}
	var x pb.Definition
	if err := x.Unmarshal(b); err != nil {
		return def, err
	}
	def.FromPB(&x)
	return def, nil
}
