package oci

import (
	"context"

	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/oci"
	"github.com/opencontainers/runc/libcontainer/specconv"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// WithRootless sets the container to be rootless mode.
// This function will be removed when containerd/containerd#2006 gets merged
func WithRootless(_ context.Context, _ oci.Client, _ *containers.Container, s *specs.Spec) error {
	specconv.ToRootless(s)
	// without removing CgroupsPath, runc fails:
	// "process_linux.go:279: applying cgroup configuration for process caused \"mkdir /sys/fs/cgroup/cpuset/default: permission denied\""
	s.Linux.CgroupsPath = ""
	return nil
}
