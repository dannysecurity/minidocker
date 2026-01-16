package log

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const DefaultRoot = "/var/lib/minidocker/containers"

// Logger captures container stdout and stderr to disk.
type Logger struct {
	root string
}

// NewLogger creates a log manager rooted at the given directory.
func NewLogger(root string) (*Logger, error) {
	if err := os.MkdirAll(root, 0755); err != nil {
		return nil, fmt.Errorf("create log root: %w", err)
	}
	return &Logger{root: root}, nil
}

// Attach returns writers that tee container output to log files.
func (l *Logger) Attach(containerID string) (stdout, stderr io.WriteCloser, err error) {
	dir := filepath.Join(l.root, containerID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, nil, err
	}

	stdoutFile, err := os.Create(filepath.Join(dir, "stdout.log"))
	if err != nil {
		return nil, nil, err
	}

	stderrFile, err := os.Create(filepath.Join(dir, "stderr.log"))
	if err != nil {
		stdoutFile.Close()
		return nil, nil, err
	}

	return stdoutFile, stderrFile, nil
}

// Read returns combined stdout and stderr logs for a container.
func (l *Logger) Read(containerID string) ([]byte, error) {
	dir := filepath.Join(l.root, containerID)

	var combined []byte
	for _, name := range []string{"stdout.log", "stderr.log"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		combined = append(combined, data...)
	}

	if len(combined) == 0 {
		return nil, fmt.Errorf("no logs found for container %q", containerID)
	}
	return combined, nil
}
