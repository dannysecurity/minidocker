package image

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ApplyLayer extracts a single container filesystem layer tarball into dest.
// Docker overlay whiteouts (.wh. prefix and ..wh.. entries) are honored so
// later layers can remove files introduced by earlier ones.
func ApplyLayer(r io.Reader, dest string) error {
	tr := tar.NewReader(r)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		name := filepath.Clean(header.Name)
		if name == "." {
			continue
		}

		// Overlay whiteout: opaque directory marker.
		if strings.HasPrefix(filepath.Base(name), ".wh..wh.") {
			continue
		}

		// Overlay whiteout: delete a path from a lower layer.
		if base := filepath.Base(name); strings.HasPrefix(base, ".wh.") {
			rel := strings.TrimPrefix(base, ".wh.")
			target := filepath.Join(dest, filepath.Dir(name), rel)
			if err := validateExtractPath(dest, target); err != nil {
				return fmt.Errorf("invalid whiteout path: %s", name)
			}
			_ = os.RemoveAll(target)
			continue
		}

		target := filepath.Join(dest, name)
		if err := validateExtractPath(dest, target); err != nil {
			return fmt.Errorf("invalid tar path: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
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
			if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
				return err
			}
			if err := os.Symlink(header.Linkname, target); err != nil {
				return err
			}
		}
	}
	return nil
}

// ApplyLayerFile opens a layer blob (plain tar or gzip-compressed tar) and applies it.
func ApplyLayerFile(path, dest string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		f.Seek(0, io.SeekStart)
		return ApplyLayer(f, dest)
	}
	defer gz.Close()

	return ApplyLayer(gz, dest)
}

// ExtractLayers applies an ordered list of layer blobs onto dest.
func ExtractLayers(layerPaths []string, dest string) error {
	if err := os.MkdirAll(dest, 0755); err != nil {
		return fmt.Errorf("create rootfs: %w", err)
	}
	for i, layerPath := range layerPaths {
		if err := ApplyLayerFile(layerPath, dest); err != nil {
			return fmt.Errorf("apply layer %d: %w", i, err)
		}
	}
	return nil
}
