# minidocker

A minimal container runtime for learning how Docker-like tools work under the hood.

minidocker implements the core pieces of a container engine in Go: pulling and storing
images, creating isolated processes with Linux namespaces, attaching simple virtual
networking, and streaming container logs.

## Features

- **Images** — fetch, unpack, and store OCI-style root filesystems locally
- **Run** — start processes inside new PID, mount, UTS, IPC, and network namespaces
- **Logs** — capture stdout/stderr from running containers
- **Networking** — create veth pairs and assign IP addresses on a bridge

## Requirements

- Linux (namespaces and cgroups v2)
- Go 1.22+
- root privileges (for namespace and network setup)

## Quick start

```bash
go build -o minidocker ./cmd/minidocker

# Pull a minimal rootfs (busybox-based demo image)
sudo ./minidocker pull busybox:latest

# Run an interactive shell
sudo ./minidocker run busybox:latest /bin/sh

# View logs from a detached container
sudo ./minidocker logs <container-id>
```

## Architecture

```
cmd/minidocker/     CLI entry point (pull, run, ps, logs, stop)
internal/
  image/            image store and rootfs extraction
  container/        namespace setup, process lifecycle
  network/          bridge and veth management
  log/              stdout/stderr capture
```

## How it works

1. **Pull** downloads a tarball, verifies its digest, and unpacks it into
   `/var/lib/minidocker/images/<name>/rootfs`.
2. **Run** uses `clone(2)` with `CLONE_NEW*` flags to create an isolated process
   tree, then `pivot_root(2)` to switch into the container rootfs.
3. **Network** creates a veth pair, moves one end into the container namespace,
   and attaches the host end to a Linux bridge with NAT.
4. **Logs** redirect the container's stdout/stderr through a pipe to a log file
   under `/var/lib/minidocker/containers/<id>/`.

## License

MIT
