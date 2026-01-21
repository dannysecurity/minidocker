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
echo "tiny-fixture" > "${rootfs}/etc/hostname"
chmod +x "${rootfs}/bin/echo" "${rootfs}/bin/readhostname"

mkdir -p "${fixture_dir}"
tar -C "${rootfs}" -czf "${fixture_dir}/tiny-rootfs.tar.gz" bin etc proc dev tmp
echo "Wrote ${fixture_dir}/tiny-rootfs.tar.gz"
