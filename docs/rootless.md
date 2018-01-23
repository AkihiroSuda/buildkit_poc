# Rootless mode (Experimental)

Requirements:
- runc with https://github.com/opencontainers/runc/pull/1688
- Some distros such as Arch Linux require `echo 1 > /proc/sys/kernel/unprivileged_ns_clone`


## Terminal 1:

```
$ unshare -U -m
unshared$ echo $$
3539
```

Unsharing mountns (and userns) is required for mounting filesystems without real root privileges.

## Terminal 2:

```
$ id -u
1001
$ grep $(whoami) /etc/subuid
suda:231072:65536
$ grep $(whoami) /etc/subgid
suda:231072:65536
$ newuidmap 3539 0 1001 1 1 231072 65536
$ newgidmap 3539 0 1001 1 1 231072 65536
```

## Terminal 1:

```
unshared# buildkitd --root /home/suda/.local/share/buildkit --addr unix:///run/user/1001/buildkitd.sock --containerd-worker false --oci-worker-overlayfs=false  --root
less
```

- On Ubuntu, no need to specify `--oci-worker-overlayfs` to `false`, as unprivileged overlayfs is supported: http://kernel.ubuntu.com/git/ubuntu/ubuntu-artful.git/commit/fs/overlayfs?h=Ubuntu-4.13.0-25.29&id=0a414bdc3d01f3b61ed86cfe3ce8b63a9240eba7
- containerd worker is not supported ( pending PR: https://github.com/containerd/containerd/pull/2006 )

## Terminal 2:

```
$ go run ./examples/buildkit0 | buildctl --addr unix:///run/user/1001/buildkitd.sock build
```

- `apt` is not supported (pending PRs: https://github.com/opencontainers/runc/pull/1693 https://github.com/opencontainers/runc/pull/1692 )
