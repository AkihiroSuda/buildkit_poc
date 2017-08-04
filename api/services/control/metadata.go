package moby_buildkit_v1

// MetadataKey is the key for grpc metadata
const MetadataKey = "json"

// Metadata is JSON-encoded value that corresponds to MetadataKey
type Metadata struct {
	Constraints []Constraint
}

// Constraint for specifying the worker to handle the request.
// Fields within structure: OR-match
// Slice of structures: AND-match (but zero-length slice matches anything)
type Constraint struct {
	Controller string // controller instance name e.g. "standalone" or "containerd" or an empty
	// TODO: hostname, annotations
}
