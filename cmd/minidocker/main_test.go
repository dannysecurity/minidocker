package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestRunCLIUnknownCommand(t *testing.T) {
	if code := runCLI([]string{"minidocker", "unknown"}); code != 1 {
		t.Fatalf("runCLI() = %d, want 1", code)
	}
}

func TestRunCLIUsage(t *testing.T) {
	if code := runCLI([]string{"minidocker"}); code != 1 {
		t.Fatalf("runCLI() = %d, want 1", code)
	}
}

func TestCmdPsAllFlagParsing(t *testing.T) {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	done := make(chan struct{})
	var buf bytes.Buffer
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()

	if err := cmdPs([]string{"-a"}); err != nil {
		t.Fatalf("cmdPs: %v", err)
	}

	w.Close()
	os.Stdout = oldStdout
	<-done

	if !bytes.Contains(buf.Bytes(), []byte("CONTAINER ID")) {
		t.Fatalf("cmdPs output = %q, want header", buf.String())
	}
}

func TestCmdRunUsageValidation(t *testing.T) {
	err := cmdRun([]string{"busybox:latest"})
	if err == nil {
		t.Fatal("expected error for missing command")
	}
	if !strings.Contains(err.Error(), "usage:") {
		t.Fatalf("error = %q, want usage message", err)
	}
}

func TestCmdLogsUsageValidation(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "missing id", args: []string{}},
		{name: "missing tail value", args: []string{"--tail"}},
		{name: "invalid tail", args: []string{"--tail", "x"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := cmdLogs(tc.args); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestCmdExecUsageValidation(t *testing.T) {
	err := cmdExec([]string{"abc123"})
	if err == nil {
		t.Fatal("expected error for missing command")
	}
	if !strings.Contains(err.Error(), "usage:") {
		t.Fatalf("error = %q, want usage message", err)
	}
}
