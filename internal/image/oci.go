package image

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ImageDetails describes a locally stored image including parsed layer metadata.
type ImageDetails struct {
	ImageInfo
	LayerCount int
}

// InstallFromOCI loads an image from an OCI image layout directory.
// The layout must contain oci-layout, index.json, and blobs/sha256/.
func (s *Store) InstallFromOCI(name, layoutDir string) error {
	indexPath := filepath.Join(layoutDir, "index.json")
	indexData, err := os.ReadFile(indexPath)
	if err != nil {
		return fmt.Errorf("read index.json: %w", err)
	}

	index, err := ParseIndex(indexData)
	if err != nil {
		return fmt.Errorf("parse index: %w", err)
	}

	blobsRoot := filepath.Join(layoutDir, "blobs")
	manifestPath, err := blobPath(blobsRoot, index.Manifests[0].Digest)
	if err != nil {
		return fmt.Errorf("resolve manifest blob: %w", err)
	}

	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}

	manifest, err := ParseManifest(manifestData)
	if err != nil {
		return fmt.Errorf("parse manifest: %w", err)
	}

	dest := filepath.Join(s.root, sanitize(name))
	if err := os.MkdirAll(dest, 0755); err != nil {
		return fmt.Errorf("create image dir: %w", err)
	}

	rootfs := filepath.Join(dest, "rootfs")
	if err := os.RemoveAll(rootfs); err != nil {
		return fmt.Errorf("clean rootfs: %w", err)
	}

	layerPaths := make([]string, len(manifest.Layers))
	for i, layer := range manifest.Layers {
		path, err := blobPath(blobsRoot, layer.Digest)
		if err != nil {
			return fmt.Errorf("resolve layer %d: %w", i, err)
		}
		layerPaths[i] = path
	}

	if err := ExtractLayers(layerPaths, rootfs); err != nil {
		return fmt.Errorf("extract layers: %w", err)
	}

	configPath, err := blobPath(blobsRoot, manifest.Config.Digest)
	if err != nil {
		return fmt.Errorf("resolve config blob: %w", err)
	}
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	hasher := sha256.New()
	hasher.Write(configData)
	for _, layerPath := range layerPaths {
		data, err := os.ReadFile(layerPath)
		if err != nil {
			return fmt.Errorf("hash layer %s: %w", layerPath, err)
		}
		hasher.Write(data)
	}
	digest := "sha256:" + hex.EncodeToString(hasher.Sum(nil))

	layers := strings.Join(manifest.LayerDigests(), ",")
	meta := fmt.Sprintf("name=%s\ndigest=%s\nsource=oci\nlayers=%s\n", name, digest, layers)
	if err := os.WriteFile(filepath.Join(dest, "meta"), []byte(meta), 0644); err != nil {
		return fmt.Errorf("write metadata: %w", err)
	}

	manifestCopy, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode manifest copy: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dest, "manifest.json"), manifestCopy, 0644); err != nil {
		return fmt.Errorf("write manifest.json: %w", err)
	}

	return nil
}

// Inspect returns detailed metadata for a locally stored image.
func (s *Store) Inspect(name string) (*ImageDetails, error) {
	dir := filepath.Join(s.root, sanitize(name))
	rootfs := filepath.Join(dir, "rootfs")
	if _, err := os.Stat(rootfs); os.IsNotExist(err) {
		return nil, fmt.Errorf("image %q not found", name)
	}

	metaPath := filepath.Join(dir, "meta")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, fmt.Errorf("read metadata: %w", err)
	}

	info := parseMeta(string(data))
	if info.Name == "" {
		info.Name = name
	}

	details := &ImageDetails{
		ImageInfo:  info,
		LayerCount: len(info.Layers),
	}

	manifestPath := filepath.Join(dir, "manifest.json")
	if manifestData, err := os.ReadFile(manifestPath); err == nil {
		if manifest, err := ParseManifest(manifestData); err == nil {
			details.Layers = manifest.LayerDigests()
			details.LayerCount = len(details.Layers)
		}
	}

	return details, nil
}
