package source

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/containerd/containerd/reference"
	"github.com/moby/buildkit/llb"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

var (
	errInvalid  = errors.New("invalid")
	errNotFound = errors.New("not found")
)

const (
	DockerImageScheme = "docker-image"
	GitScheme         = "git"
	LocalScheme       = "local"
	HttpScheme        = "http"
	HttpsScheme       = "https"
)

type Identifier interface {
	ID() string // until sources are in process this string comparison could be avoided
}

func FromString(s string) (Identifier, error) {
	// TODO: improve this
	parts := strings.SplitN(s, "://", 2)
	if len(parts) != 2 {
		return nil, errors.Wrapf(errInvalid, "failed to parse %s", s)
	}

	switch parts[0] {
	case DockerImageScheme:
		return NewImageIdentifier(parts[1])
	case GitScheme:
		return NewGitIdentifier(parts[1])
	case LocalScheme:
		return NewLocalIdentifier(parts[1])
	case HttpsScheme:
		return NewHttpIdentifier(parts[1], true)
	case HttpScheme:
		return NewHttpIdentifier(parts[1], false)
	default:
		return nil, errors.Wrapf(errNotFound, "unknown schema %s", parts[0])
	}
}
func FromLLB(op *llb.Op_Source) (Identifier, error) {
	id, err := FromString(op.Source.Identifier)
	if err != nil {
		return nil, err
	}
	if id, ok := id.(*GitIdentifier); ok {
		for k, v := range op.Source.Attrs {
			switch k {
			case llb.AttrKeepGitDir:
				if v == "true" {
					id.KeepGitDir = true
				}
			}
		}
	}
	if id, ok := id.(*LocalIdentifier); ok {
		for k, v := range op.Source.Attrs {
			switch k {
			case llb.AttrLocalSessionID:
				id.SessionID = v
				if p := strings.SplitN(v, ":", 2); len(p) == 2 {
					id.Name = p[0] + "-" + id.Name
					id.SessionID = p[1]
				}
			case llb.AttrIncludePatterns:
				var patterns []string
				if err := json.Unmarshal([]byte(v), &patterns); err != nil {
					return nil, err
				}
				id.IncludePatterns = patterns
			case llb.AttrExcludePatterns:
				var patterns []string
				if err := json.Unmarshal([]byte(v), &patterns); err != nil {
					return nil, err
				}
				id.ExcludePatterns = patterns
			case llb.AttrSharedKeyHint:
				id.SharedKeyHint = v
			}
		}
	}
	if id, ok := id.(*HttpIdentifier); ok {
		for k, v := range op.Source.Attrs {
			switch k {
			case llb.AttrHTTPChecksum:
				dgst, err := digest.Parse(v)
				if err != nil {
					return nil, err
				}
				id.Checksum = dgst
			case llb.AttrHTTPFilename:
				id.Filename = v
			case llb.AttrHTTPPerm:
				i, err := strconv.ParseInt(v, 0, 64)
				if err != nil {
					return nil, err
				}
				id.Perm = int(i)
			case llb.AttrHTTPUID:
				i, err := strconv.ParseInt(v, 0, 64)
				if err != nil {
					return nil, err
				}
				id.UID = int(i)
			case llb.AttrHTTPGID:
				i, err := strconv.ParseInt(v, 0, 64)
				if err != nil {
					return nil, err
				}
				id.GID = int(i)
			}
		}
	}
	return id, nil
}

type ImageIdentifier struct {
	Reference reference.Spec
}

func NewImageIdentifier(str string) (*ImageIdentifier, error) {
	ref, err := reference.Parse(str)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if ref.Object == "" {
		return nil, errors.WithStack(reference.ErrObjectRequired)
	}
	return &ImageIdentifier{Reference: ref}, nil
}

func (_ *ImageIdentifier) ID() string {
	return DockerImageScheme
}

type LocalIdentifier struct {
	Name            string
	SessionID       string
	IncludePatterns []string
	ExcludePatterns []string
	SharedKeyHint   string
}

func NewLocalIdentifier(str string) (*LocalIdentifier, error) {
	return &LocalIdentifier{Name: str}, nil
}

func (*LocalIdentifier) ID() string {
	return LocalScheme
}

func NewHttpIdentifier(str string, tls bool) (*HttpIdentifier, error) {
	proto := "https://"
	if !tls {
		proto = "http://"
	}
	return &HttpIdentifier{TLS: tls, URL: proto + str}, nil
}

type HttpIdentifier struct {
	TLS      bool
	URL      string
	Checksum digest.Digest
	Filename string
	Perm     int
	UID      int
	GID      int
}

func (_ *HttpIdentifier) ID() string {
	return HttpsScheme
}
