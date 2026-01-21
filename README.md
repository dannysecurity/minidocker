# minidocker

A minimal container runtime for learning how Docker-like tools work under the hood.

minidocker implements the core pieces of a container engine in Go: pulling and storing
images, creating isolated processes with Linux namespaces, attaching simple virtual
networking, and streaming container logs.

## Features

- **Images** — fetch, unpack, and store OCI-style root filesystems locally; list with `images`
- **Run** — start processes inside new PID, mount, UTS, IPC, and network namespaces
- **Logs** — capture stdout/stderr from running containers
- **Networking** — create veth pairs and assign IP addresses on a bridge
- **Exec** — attach to a running container's namespaces via `nsenter`
- **Lifecycle** — stop running containers and remove stopped ones with `rm`

## Requirements

- Linux (namespaces; cgroups are not used yet)
- Go 1.22+
- root privileges (for namespace and network setup)

## Quick start

```bash
go build -o minidocker ./cmd/minidocker

# Pull a minimal rootfs (busybox-based demo image)
sudo ./minidocker pull busybox:latest

# List images stored locally
sudo ./minidocker images

# Run an interactive shell
sudo ./minidocker run busybox:latest /bin/sh

# Run detached and publish a port (parsed at CLI; NAT forwarding not yet wired)
sudo ./minidocker run -d -p 8080:80 busybox:latest /bin/httpd -f

# View logs from a detached container
sudo ./minidocker logs <container-id>

# Inspect container metadata (JSON)
sudo ./minidocker inspect <container-id>

# Stop and remove a container
sudo ./minidocker stop <container-id>
sudo ./minidocker rm <container-id>
```

## Architecture

minidocker is a single-binary CLI that wires together four internal packages. The CLI
parses commands and delegates to package APIs; it does not implement container logic
itself.

### Component overview

```mermaid
flowchart TB
  subgraph CLI["cmd/minidocker"]
    pull[pull]
    images[images]
    run[run]
    ps[ps]
    inspect[inspect]
    logs[logs]
    exec[exec]
    stop[stop]
    rm[rm]
  end

  subgraph Internal["internal/"]
    image["image.Store\npull, list, unpack, rootfs lookup"]
    container["container.Runtime\nnamespaces, lifecycle, rm"]
    network["network.Manager\nbridge + veth"]
    log["log.Logger\nstdout/stderr capture"]
  end

  pull --> image
  images --> image
  run --> image
  run --> container
  run --> log
  container --> network
  ps --> container
  inspect --> container
  logs --> log
  exec --> container
  stop --> container
  rm --> container
```

| Package | Responsibility | Default on-disk root |
|---------|----------------|----------------------|
| `image` | Download tarballs, verify SHA-256, extract rootfs, list stored images | `/var/lib/minidocker/images/` |
| `container` | Create namespaces, start processes, persist metadata, stop/remove | `/var/lib/minidocker/containers/` |
| `network` | Bridge `minidocker0`, veth pairs, container IP allocation | (kernel interfaces) |
| `log` | Attach stdout/stderr writers per container | same dir as `container` metadata |

### Pull → run flow

```mermaid
sequenceDiagram
  participant User
  participant CLI
  participant Image as image.Store
  participant Runtime as container.Runtime
  participant Net as network.Manager
  participant Logger as log.Logger

  User->>CLI: minidocker pull busybox:latest
  CLI->>Image: Pull(name)
  Image->>Image: download, hash, extract to rootfs/

  User->>CLI: minidocker run busybox:latest /bin/sh
  CLI->>Image: RootfsPath(name)
  CLI->>Logger: NewLogger + Attach(id)
  CLI->>Runtime: Run(spec)
  Runtime->>Runtime: clone(2) with namespace flags
  Runtime->>Runtime: chroot into rootfs
  Runtime->>Net: Setup(id, pid)
  Net->>Net: veth pair + bridge + IP
  Runtime->>Runtime: save config.json
  Runtime-->>User: container ID + pid
```

### Container lifecycle

Containers move through a small set of states tracked in `config.json`:

```mermaid
stateDiagram-v2
  [*] --> running: run (Start + save config)
  running --> exited: process exits (foreground or detached waiter)
  running --> stopped: stop (SIGTERM)
  stopped --> exited: optional relabel on stale metadata
  exited --> [*]: rm (RemoveAll container dir)
  stopped --> [*]: rm
```

| State | Meaning | Allowed next commands |
|-------|---------|----------------------|
| `running` | PID alive in namespaces | `logs`, `exec`, `inspect`, `stop` |
| `stopped` | SIGTERM sent; directory kept | `logs`, `inspect`, `rm` |
| `exited` | Process finished | `logs`, `inspect`, `rm` |

`container.Runtime.Remove` resolves ID prefixes (same rules as `inspect`), refuses
containers whose PID still responds to signal `0`, and deletes the entire
`<id>/` directory—including `config.json`, `stdout.log`, and `stderr.log`.

### Exec flow

`exec` does not start a new container. It re-enters the **existing** process
namespaces with `nsenter`:

```mermaid
sequenceDiagram
  participant User
  participant CLI
  participant Runtime as container.Runtime
  participant Nsenter as nsenter(1)

  User->>CLI: minidocker exec <id> /bin/sh
  CLI->>Runtime: Exec(id, command)
  Runtime->>Runtime: ResolveID + load config.json
  Runtime->>Runtime: verify status=running, kill(pid,0)
  Runtime->>Nsenter: --target PID --mount --uts --ipc --net --pid -- cmd
  Nsenter-->>User: attached stdio in container namespaces
```

### On-disk layout

After pulling `busybox:latest` and running one container, state looks like this:

```
/var/lib/minidocker/
├── images/
│   └── busybox_latest/
│       ├── meta                 # name, digest, optional source=local
│       └── rootfs/              # unpacked container filesystem
│           ├── bin/
│           ├── etc/
│           └── ...
└── containers/
    └── <12-char-id>/
        ├── config.json          # id, image, command, status, pid, created
        ├── stdout.log           # captured stdout (when logger attached)
        └── stderr.log           # captured stderr
```

Container metadata is JSON (`config.json`) and is written at start, updated on exit,
and read by `ps`, `inspect`, and `stop`. Use `minidocker inspect <id>` to dump the
full record:

```bash
sudo ./minidocker inspect abc123def456
```

### Isolation model

`container.Runtime.Run` starts the workload with these `clone(2)` flags:

| Flag | Namespace | Effect |
|------|-----------|--------|
| `CLONE_NEWUTS` | UTS | Separate hostname (set to container ID) |
| `CLONE_NEWPID` | PID | Process tree isolated from host |
| `CLONE_NEWNS` | Mount | Private mount namespace; rootfs via `chroot` |
| `CLONE_NEWIPC` | IPC | Separate SysV IPC / POSIX message queues |
| `CLONE_NEWNET` | Network | Dedicated network stack; veth moved in after start |

Networking runs **after** `cmd.Start()` so the child PID is available for
`ip link set … netns /proc/<pid>/ns/net`. The host bridge `minidocker0` uses
`172.17.0.1/16`; each container receives `172.17.0.<n>/24` derived from its ID.

### Runtime behavior

- **`run` is foreground by default** — pass `-d` to detach; the CLI returns the
  container ID immediately and a background goroutine waits for exit.
- **Shared container directory** — `container.Runtime` and `log.Logger` both
  use `/var/lib/minidocker/containers/`; each run creates
  `<id>/config.json`, `stdout.log`, and `stderr.log` under the same folder.
- **Best-effort networking** — if veth/bridge setup fails, minidocker prints a
  warning and keeps the container running without external connectivity.
- **Port publish parsing** — `-p host:container` is validated at the CLI and
  stored on `RunSpec`; iptables NAT forwarding is not implemented yet.
- **Offline images** — tests and fixtures load tarballs via
  `image.Store.InstallFromTar` instead of `Pull`; `images` shows `source=local`.
- **Cleanup** — `rm` removes stopped/exited container directories; running
  containers must be stopped first.

### Package map

```
cmd/minidocker/     CLI entry point (pull, images, run, ps, inspect, logs, exec, stop, rm)
internal/
  image/            image store, listing, and rootfs extraction
  container/        namespace setup, process lifecycle, metadata I/O, removal
  network/          bridge and veth management, port mapping parse helpers
  log/              stdout/stderr capture
  testutil/         shared helpers for unit and integration tests
testdata/fixtures/  checked-in rootfs tarball for offline tests
```

## How it works

1. **Pull** downloads a tarball, verifies its digest, and unpacks it into
   `/var/lib/minidocker/images/<name>/rootfs`.
2. **Run** uses `clone(2)` with `CLONE_NEW*` flags to create an isolated process
   tree, then `chroot(2)` into the image rootfs before the workload starts.
3. **Network** creates a veth pair, moves one end into the container namespace,
   and attaches the host end to the `minidocker0` bridge (no NAT/iptables yet).
4. **Logs** redirect the container's stdout/stderr through a pipe to a log file
   under `/var/lib/minidocker/containers/<id>/`.

## Testing

Unit tests run without root and use the checked-in `testdata/fixtures/tiny-rootfs.tar.gz`
fixture instead of downloading images:

```bash
go test ./...
```

Integration tests exercise the full run path (namespaces, chroot, log capture) with the
fixture image and require root:

```bash
sudo go test -tags=integration ./...
```

Regenerate the fixture tarball after changing the embedded echo helper:

```bash
./scripts/build-test-fixture.sh
```

## License

MIT
