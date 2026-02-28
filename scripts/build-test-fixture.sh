#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
build_dir="$(mktemp -d)"
rootfs="${build_dir}/rootfs"
fixture_dir="${root}/testdata/fixtures"

cleanup() {
  rm -rf "${build_dir}"
}
trap cleanup EXIT

mkdir -p "${rootfs}/bin" "${rootfs}/etc" "${rootfs}/proc" "${rootfs}/dev" "${rootfs}/tmp"

cat > "${build_dir}/echo.go" <<'EOF'
package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	if len(os.Args) > 1 {
		fmt.Println(strings.Join(os.Args[1:], " "))
	}
}
EOF

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" \
  -o "${rootfs}/bin/echo" "${build_dir}/echo.go"

cat > "${build_dir}/readhostname.go" <<'EOF'
package main

import (
	"fmt"
	"os"
)

func main() {
	data, err := os.ReadFile("/proc/sys/kernel/hostname")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Print(string(data))
}
EOF

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" \
  -o "${rootfs}/bin/readhostname" "${build_dir}/readhostname.go"

cat > "${build_dir}/sleep.go" <<'EOF'
package main

import (
	"os"
	"strconv"
	"time"
)

func main() {
	seconds := 3600
	if len(os.Args) > 1 {
		if n, err := strconv.Atoi(os.Args[1]); err == nil && n > 0 {
			seconds = n
		}
	}
	time.Sleep(time.Duration(seconds) * time.Second)
}
EOF

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" \
  -o "${rootfs}/bin/sleep" "${build_dir}/sleep.go"

cat > "${build_dir}/tcpecho.go" <<'EOF'
package main

import (
	"fmt"
	"io"
	"net"
	"os"
)

func main() {
	port := "9000"
	if len(os.Args) > 1 {
		port = os.Args[1]
	}
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer ln.Close()
	conn, err := ln.Accept()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer conn.Close()
	if _, err := io.Copy(conn, conn); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
EOF

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" \
  -o "${rootfs}/bin/tcpecho" "${build_dir}/tcpecho.go"

cat > "${build_dir}/writestderr.go" <<'EOF'
package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	if len(os.Args) > 1 {
		fmt.Fprintln(os.Stderr, strings.Join(os.Args[1:], " "))
	}
}
EOF

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" \
  -o "${rootfs}/bin/writestderr" "${build_dir}/writestderr.go"

cat > "${build_dir}/opendevnull.go" <<'EOF'
package main

import (
	"fmt"
	"os"
)

func main() {
	f, err := os.OpenFile("/dev/null", os.O_WRONLY, 0)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	_ = f.Close()
	fmt.Println("dev-null-ok")
}
EOF

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" \
  -o "${rootfs}/bin/opendevnull" "${build_dir}/opendevnull.go"
echo "tiny-fixture" > "${rootfs}/etc/hostname"
chmod +x "${rootfs}/bin/echo" "${rootfs}/bin/readhostname" "${rootfs}/bin/sleep" \
  "${rootfs}/bin/tcpecho" "${rootfs}/bin/writestderr" "${rootfs}/bin/opendevnull"

mkdir -p "${fixture_dir}"
tar -C "${rootfs}" -czf "${fixture_dir}/tiny-rootfs.tar.gz" bin etc proc dev tmp
echo "Wrote ${fixture_dir}/tiny-rootfs.tar.gz"
