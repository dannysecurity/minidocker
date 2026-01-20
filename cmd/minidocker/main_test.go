package main

import (
	"bytes"
	"io"
	"os"
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
