package image

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const DefaultRoot = "/var/lib/minidocker/images"

// ImageInfo describes a locally stored image.
type ImageInfo struct {
	Name   string
	Digest string
	Source string // empty for pulled images, "local" for InstallFromTar, "oci" for InstallFromOCI
	Layers []string
}

// Store manages local image storage and retrieval.
type Store struct {
	root string
}

// NewStore creates an image store rooted at the given directory.
func NewStore(root string) *Store {
	return &Store{root: root}
}

// Pull downloads an image tarball and unpacks it into the local store.
// For demo purposes, known image names map to public rootfs tarballs.
func (s *Store) Pull(name string) error {
	url, err := resolveURL(name)
	if err != nil {
		return err
	}

	dest := filepath.Join(s.root, sanitize(name))
	if err := os.MkdirAll(dest, 0755); err != nil {
		return fmt.Errorf("create image dir: %w", err)
	}

	fmt.Printf("Pulling %s...\n", name)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	tmpFile, err := os.CreateTemp(dest, "download-*.tar.gz")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	hasher := sha256.New()
	writer := io.MultiWriter(tmpFile, hasher)

	if _, err := io.Copy(writer, resp.Body); err != nil {
		tmpFile.Close()
		return fmt.Errorf("write download: %w", err)
	}
	tmpFile.Close()

	digest := hex.EncodeToString(hasher.Sum(nil))
	fmt.Printf("Digest: sha256:%s\n", digest[:12])

	rootfs := filepath.Join(dest, "rootfs")
	if err := os.RemoveAll(rootfs); err != nil {
		return fmt.Errorf("clean rootfs: %w", err)
	}
	if err := os.MkdirAll(rootfs, 0755); err != nil {
		return fmt.Errorf("create rootfs: %w", err)
	}

	if err := extractTarGz(tmpPath, rootfs); err != nil {
		return fmt.Errorf("extract rootfs: %w", err)
	}

	meta := fmt.Sprintf("name=%s\ndigest=sha256:%s\n", name, digest)
	if err := os.WriteFile(filepath.Join(dest, "meta"), []byte(meta), 0644); err != nil {
		return fmt.Errorf("write metadata: %w", err)
	}

	fmt.Printf("Successfully pulled %s\n", name)
	return nil
}

// InstallFromTar unpacks a local rootfs tarball into the image store.
// This is useful for offline loading and integration tests with fixture images.
func (s *Store) InstallFromTar(name, tarPath string) error {
	dest := filepath.Join(s.root, sanitize(name))
	if err := os.MkdirAll(dest, 0755); err != nil {
		return fmt.Errorf("create image dir: %w", err)
	}

	data, err := os.ReadFile(tarPath)
	if err != nil {
		return fmt.Errorf("read tarball: %w", err)
	}

	hasher := sha256.New()
	hasher.Write(data)
	digest := hex.EncodeToString(hasher.Sum(nil))

	rootfs := filepath.Join(dest, "rootfs")
	if err := os.RemoveAll(rootfs); err != nil {
		return fmt.Errorf("clean rootfs: %w", err)
	}
	if err := os.MkdirAll(rootfs, 0755); err != nil {
		return fmt.Errorf("create rootfs: %w", err)
	}

	tmpFile, err := os.CreateTemp(dest, "install-*.tar.gz")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	tmpFile.Close()

	if err := extractTarGz(tmpPath, rootfs); err != nil {
		return fmt.Errorf("extract rootfs: %w", err)
	}

	meta := fmt.Sprintf("name=%s\ndigest=sha256:%s\nsource=local\n", name, digest)
	if err := os.WriteFile(filepath.Join(dest, "meta"), []byte(meta), 0644); err != nil {
		return fmt.Errorf("write metadata: %w", err)
	}

	return nil
}

// List returns metadata for every image in the local store.
func (s *Store) List() ([]ImageInfo, error) {
	entries, err := os.ReadDir(s.root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var images []ImageInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		metaPath := filepath.Join(s.root, entry.Name(), "meta")
		data, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}
		info := parseMeta(string(data))
		if info.Name == "" {
			info.Name = unsanitize(entry.Name())
		}
		images = append(images, info)
	}
	return images, nil
}

// Remove deletes a locally stored image directory, including metadata and rootfs.
func (s *Store) Remove(name string) error {
	dir := filepath.Join(s.root, sanitize(name))
	rootfs := filepath.Join(dir, "rootfs")
	if _, err := os.Stat(rootfs); os.IsNotExist(err) {
		return fmt.Errorf("image %q not found", name)
	}
	return os.RemoveAll(dir)
}

// RootfsPath returns the path to a pulled image's root filesystem.
func (s *Store) RootfsPath(name string) (string, error) {
	rootfs := filepath.Join(s.root, sanitize(name), "rootfs")
	if _, err := os.Stat(rootfs); os.IsNotExist(err) {
		return "", fmt.Errorf("image %q not found — run: minidocker pull %s", name, name)
	}
	return rootfs, nil
}

func resolveURL(name string) (string, error) {
	// Demo images: small rootfs tarballs suitable for learning.
	known := map[string]string{
		"busybox:latest":  "https://github.com/docker-library/busybox/releases/download/1.36.1/busybox.tar.gz",
		"alpine:latest":   "https://dl-cdn.alpinelinux.org/alpine/v3.19/releases/x86_64/alpine-minirootfs-3.19.0-x86_64.tar.gz",
		"demo:latest":     "https://github.com/docker-library/busybox/releases/download/1.36.1/busybox.tar.gz",
	}

	if url, ok := known[name]; ok {
		return url, nil
	}
	return "", fmt.Errorf("unknown image %q — supported: busybox:latest, alpine:latest, demo:latest", name)
}

func sanitize(name string) string {
	return strings.ReplaceAll(name, ":", "_")
}

func unsanitize(dir string) string {
	if idx := strings.LastIndex(dir, "_"); idx >= 0 {
		return dir[:idx] + ":" + dir[idx+1:]
	}
	return dir
}

func parseMeta(content string) ImageInfo {
	var info ImageInfo
	for _, line := range strings.Split(content, "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch key {
		case "name":
			info.Name = value
		case "digest":
			info.Digest = value
		case "source":
			info.Source = value
		case "layers":
			if value != "" {
				info.Layers = strings.Split(value, ",")
			}
		}
	}
	return info
}

func validateExtractPath(dest, target string) error {
	cleanDest := filepath.Clean(dest)
	cleanTarget := filepath.Clean(target)
	if cleanTarget != cleanDest && !strings.HasPrefix(cleanTarget, cleanDest+string(os.PathSeparator)) {
		return fmt.Errorf("path escapes destination")
	}
	return nil
}

func extractTarGz(src, dest string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		// Not gzipped — try plain tar.
		f.Seek(0, io.SeekStart)
		return extractTar(f, dest)
	}
	defer gz.Close()

	return extractTar(gz, dest)
}

func extractTar(r io.Reader, dest string) error {
	tr := tar.NewReader(r)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(dest, header.Name)
		if err := validateExtractPath(dest, target); err != nil {
			return fmt.Errorf("invalid tar path: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			if err := os.Symlink(header.Linkname, target); err != nil {
				return err
			}
		}
	}
	return nil
}
