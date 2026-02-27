//go:build integration

package integrationtest

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dannysecurity/minidocker/internal/container"
	"github.com/dannysecurity/minidocker/internal/network"
	"github.com/dannysecurity/minidocker/internal/testutil"
)

func TestIntegration_RunFixtureEcho(t *testing.T) {
	testutil.RequireRoot(t)
	env := NewEnv(t)

	id, err := env.Runtime.Run(container.RunSpec{
		Image:   ImageRef,
		Rootfs:  env.Rootfs,
		Command: []string{"/bin/echo", "hello", "fixture"},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if id == "" {
		t.Fatal("Run returned empty container ID")
	}

	logs, err := env.Logger.Read(id)
	if err != nil {
		t.Fatalf("Read logs: %v", err)
	}
	if !strings.Contains(string(logs), "hello fixture") {
		t.Fatalf("logs = %q, want output containing %q", logs, "hello fixture")
	}

	containers, err := env.Runtime.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	var found container.Info
	for _, c := range containers {
		if c.ID == id {
			found = c
			break
		}
	}
	if found.ID == "" {
		t.Fatalf("container %q not found in List()", id)
	}
	if found.Status != "exited" {
		t.Fatalf("status = %q, want exited", found.Status)
	}
	if found.Image != ImageRef {
		t.Fatalf("image = %q, want %s", found.Image, ImageRef)
	}
}

func TestIntegration_HostnameSetInUTSNamespace(t *testing.T) {
	testutil.RequireRoot(t)
	env := NewEnv(t)

	id, err := env.Runtime.Run(container.RunSpec{
		Image:   ImageRef,
		Rootfs:  env.Rootfs,
		Command: []string{"/bin/readhostname"},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	logs, err := env.Logger.Read(id)
	if err != nil {
		t.Fatalf("Read logs: %v", err)
	}
	got := strings.TrimSpace(string(logs))
	if got != id {
		t.Fatalf("hostname = %q, want container id %q", got, id)
	}
}

func TestIntegration_InspectAndRemoveFixture(t *testing.T) {
	testutil.RequireRoot(t)
	env := NewEnv(t)

	id, err := env.Runtime.Run(container.RunSpec{
		Image:   ImageRef,
		Rootfs:  env.Rootfs,
		Command: []string{"/bin/echo", "inspect-me"},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	info, err := env.Runtime.Inspect(id)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if info.ID != id {
		t.Fatalf("Inspect().ID = %q, want %q", info.ID, id)
	}
	if info.Image != ImageRef {
		t.Fatalf("Inspect().Image = %q, want %s", info.Image, ImageRef)
	}
	if info.Command != "/bin/echo inspect-me" {
		t.Fatalf("Inspect().Command = %q, want %q", info.Command, "/bin/echo inspect-me")
	}
	if info.Status != "exited" {
		t.Fatalf("Inspect().Status = %q, want exited", info.Status)
	}

	prefix := id[:6]
	resolved, err := env.Runtime.ResolveID(prefix)
	if err != nil {
		t.Fatalf("ResolveID(%q): %v", prefix, err)
	}
	if resolved != id {
		t.Fatalf("ResolveID(%q) = %q, want %q", prefix, resolved, id)
	}

	if err := env.Runtime.Remove(prefix); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	if _, err := env.Runtime.Inspect(id); err == nil {
		t.Fatal("Inspect after Remove: expected error")
	}

	containers, err := env.Runtime.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, c := range containers {
		if c.ID == id {
			t.Fatalf("container %q still listed after Remove", id)
		}
	}
}

func TestIntegration_DetachedRunCapturesLogs(t *testing.T) {
	testutil.RequireRoot(t)
	env := NewEnv(t)

	id, err := env.Runtime.Run(container.RunSpec{
		Image:   ImageRef,
		Rootfs:  env.Rootfs,
		Command: []string{"/bin/echo", "detached", "fixture"},
		Detach:  true,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	WaitForStatus(t, env.Runtime, id, "exited", 10*time.Second)

	logs, err := env.Logger.Read(id)
	if err != nil {
		t.Fatalf("Read logs: %v", err)
	}
	if !strings.Contains(string(logs), "detached fixture") {
		t.Fatalf("logs = %q, want output containing %q", logs, "detached fixture")
	}
}

func TestIntegration_StopLongRunningFixture(t *testing.T) {
	testutil.RequireRoot(t)
	env := NewEnv(t)

	id, err := env.Runtime.Run(container.RunSpec{
		Image:   ImageRef,
		Rootfs:  env.Rootfs,
		Command: []string{"/bin/sleep", "3600"},
		Detach:  true,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	t.Cleanup(func() { EnsureContainerDead(t, env.Runtime, id) })

	WaitForStatus(t, env.Runtime, id, "running", 5*time.Second)

	if err := env.Runtime.Stop(id); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	info, err := env.Runtime.Inspect(id)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if info.Status != "stopped" {
		t.Fatalf("status after Stop = %q, want stopped", info.Status)
	}

	EnsureContainerDead(t, env.Runtime, id)
}

func TestIntegration_PortMappingForwardsTCP(t *testing.T) {
	testutil.RequireRoot(t)
	testutil.RequireNetworkTools(t)
	env := NewEnv(t)

	hostPort := 40000 + (os.Getpid() % 20000)
	containerPort := 9000

	id, err := env.Runtime.Run(container.RunSpec{
		Image:   ImageRef,
		Rootfs:  env.Rootfs,
		Command: []string{"/bin/tcpecho", fmt.Sprintf("%d", containerPort)},
		Detach:  true,
		PortMappings: []network.PortMapping{
			{HostPort: hostPort, ContainerPort: containerPort},
		},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	t.Cleanup(func() { EnsureContainerDead(t, env.Runtime, id) })

	WaitForStatus(t, env.Runtime, id, "running", 5*time.Second)

	info, err := env.Runtime.Inspect(id)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if info.IP == "" {
		t.Fatal("Inspect().IP is empty")
	}
	if len(info.PortMappings) != 1 {
		t.Fatalf("Inspect().PortMappings = %+v, want one mapping", info.PortMappings)
	}
	if info.PortMappings[0].HostPort != hostPort {
		t.Fatalf("host port = %d, want %d", info.PortMappings[0].HostPort, hostPort)
	}

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", hostPort), 2*time.Second)
	if err != nil {
		t.Fatalf("Dial host port: %v", err)
	}
	defer conn.Close()

	msg := "port-mapping-ok"
	if _, err := conn.Write([]byte(msg)); err != nil {
		t.Fatalf("Write: %v", err)
	}

	buf := make([]byte, len(msg))
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatalf("Read echo: %v", err)
	}
	if string(buf) != msg {
		t.Fatalf("echo = %q, want %q", buf, msg)
	}

	if err := env.Runtime.Remove(id); err != nil {
		t.Fatalf("Remove: %v", err)
	}
}

func TestIntegration_ExecIntoRunningFixture(t *testing.T) {
	testutil.RequireRoot(t)
	env := NewEnv(t)

	id, err := env.Runtime.Run(container.RunSpec{
		Image:   ImageRef,
		Rootfs:  env.Rootfs,
		Command: []string{"/bin/sleep", "3600"},
		Detach:  true,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	t.Cleanup(func() { EnsureContainerDead(t, env.Runtime, id) })

	WaitForStatus(t, env.Runtime, id, "running", 5*time.Second)

	out := captureStdout(t, func() {
		if err := env.Runtime.Exec(id, []string{"/bin/echo", "from-exec"}); err != nil {
			t.Fatalf("Exec: %v", err)
		}
	})
	if !strings.Contains(out, "from-exec") {
		t.Fatalf("exec stdout = %q, want output containing %q", out, "from-exec")
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}

	old := os.Stdout
	os.Stdout = w
	fn()
	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("read captured stdout: %v", err)
	}
	_ = r.Close()
	return buf.String()
}
