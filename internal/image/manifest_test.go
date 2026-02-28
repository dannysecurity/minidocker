package image

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dannysecurity/minidocker/internal/testutil"
)

func TestParseManifest(t *testing.T) {
	data := []byte(`{
		"schemaVersion": 2,
		"mediaType": "application/vnd.oci.image.manifest.v1+json",
		"config": {
			"mediaType": "application/vnd.oci.image.config.v1+json",
			"digest": "sha256:abc123",
			"size": 100
		},
		"layers": [
			{
				"mediaType": "application/vnd.oci.image.layer.v1.tar+gzip",
				"digest": "sha256:layer1",
				"size": 2048
			},
			{
				"mediaType": "application/vnd.oci.image.layer.v1.tar+gzip",
				"digest": "sha256:layer2",
				"size": 1024
			}
		]
	}`)

	manifest, err := ParseManifest(data)
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	if manifest.SchemaVersion != 2 {
		t.Fatalf("SchemaVersion = %d, want 2", manifest.SchemaVersion)
	}
	if len(manifest.Layers) != 2 {
		t.Fatalf("len(Layers) = %d, want 2", len(manifest.Layers))
	}
	if manifest.Config.Digest != "sha256:abc123" {
		t.Fatalf("Config.Digest = %q", manifest.Config.Digest)
	}

	digests := manifest.LayerDigests()
	if digests[0] != "sha256:layer1" || digests[1] != "sha256:layer2" {
		t.Fatalf("LayerDigests() = %v", digests)
	}
}

func TestParseManifestRejectsInvalid(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{name: "wrong schema", data: `{"schemaVersion":1,"config":{"digest":"sha256:x"},"layers":[{"digest":"sha256:y"}]}`},
		{name: "no layers", data: `{"schemaVersion":2,"config":{"digest":"sha256:x"},"layers":[]}`},
		{name: "missing config", data: `{"schemaVersion":2,"layers":[{"digest":"sha256:y"}]}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseManifest([]byte(tc.data)); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestParseIndex(t *testing.T) {
	data := []byte(`{
		"schemaVersion": 2,
		"manifests": [
			{"mediaType":"application/vnd.oci.image.manifest.v1+json","digest":"sha256:manifest","size":512}
		]
	}`)
	index, err := ParseIndex(data)
	if err != nil {
		t.Fatalf("ParseIndex: %v", err)
	}
	if len(index.Manifests) != 1 {
		t.Fatalf("len(Manifests) = %d, want 1", len(index.Manifests))
	}
	if index.Manifests[0].Digest != "sha256:manifest" {
		t.Fatalf("digest = %q", index.Manifests[0].Digest)
	}
}

func TestApplyLayerWhiteout(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "rootfs")
	if err := os.MkdirAll(filepath.Join(dest, "etc"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dest, "etc", "hosts"), []byte("keep\n"), 0644); err != nil {
		t.Fatalf("write hosts: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dest, "etc", "remove-me"), []byte("gone\n"), 0644); err != nil {
		t.Fatalf("write remove-me: %v", err)
	}

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := tw.WriteHeader(&tar.Header{
		Name:     "etc/.wh.remove-me",
		Typeflag: tar.TypeReg,
		Size:     0,
		Mode:     0644,
	}); err != nil {
		t.Fatalf("write whiteout header: %v", err)
	}
	if err := tw.WriteHeader(&tar.Header{
		Name:     "etc/new-file",
		Typeflag: tar.TypeReg,
		Size:     3,
		Mode:     0644,
	}); err != nil {
		t.Fatalf("write new file header: %v", err)
	}
	if _, err := tw.Write([]byte("new")); err != nil {
		t.Fatalf("write new file body: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}

	if err := ApplyLayer(&buf, dest); err != nil {
		t.Fatalf("ApplyLayer: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dest, "etc", "remove-me")); !os.IsNotExist(err) {
		t.Fatal("expected remove-me to be deleted by whiteout")
	}
	if data, err := os.ReadFile(filepath.Join(dest, "etc", "hosts")); err != nil || string(data) != "keep\n" {
		t.Fatalf("hosts = %q, err = %v", data, err)
	}
	if data, err := os.ReadFile(filepath.Join(dest, "etc", "new-file")); err != nil || string(data) != "new" {
		t.Fatalf("new-file = %q, err = %v", data, err)
	}
}

func TestInstallFromOCI(t *testing.T) {
	layout := buildTestOCILayout(t)
	store := NewStore(t.TempDir())

	if err := store.InstallFromOCI("oci-tiny:latest", layout); err != nil {
		t.Fatalf("InstallFromOCI: %v", err)
	}

	rootfs, err := store.RootfsPath("oci-tiny:latest")
	if err != nil {
		t.Fatalf("RootfsPath: %v", err)
	}
	echoPath := filepath.Join(rootfs, "bin", "echo")
	if _, err := os.Stat(echoPath); err != nil {
		t.Fatalf("stat echo: %v", err)
	}

	details, err := store.Inspect("oci-tiny:latest")
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if details.Source != "oci" {
		t.Fatalf("Source = %q, want oci", details.Source)
	}
	if details.LayerCount != 1 {
		t.Fatalf("LayerCount = %d, want 1", details.LayerCount)
	}
	if len(details.Layers) != 1 {
		t.Fatalf("Layers = %v, want 1 entry", details.Layers)
	}
}

func buildTestOCILayout(t *testing.T) string {
	t.Helper()

	layout := t.TempDir()
	blobs := filepath.Join(layout, "blobs", "sha256")
	if err := os.MkdirAll(blobs, 0755); err != nil {
		t.Fatalf("mkdir blobs: %v", err)
	}

	layerPath := testutil.FixturePath(t, "tiny-rootfs.tar.gz")
	layerData, err := os.ReadFile(layerPath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	layerDigest := "sha256:" + sha256Hex(layerData)
	if err := os.WriteFile(filepath.Join(blobs, layerDigest[7:]), layerData, 0644); err != nil {
		t.Fatalf("write layer blob: %v", err)
	}

	config := map[string]any{
		"architecture": "amd64",
		"os":           "linux",
		"rootfs":       map[string]any{"type": "layers", "diff_ids": []string{layerDigest}},
	}
	configData, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	configDigest := "sha256:" + sha256Hex(configData)
	if err := os.WriteFile(filepath.Join(blobs, configDigest[7:]), configData, 0644); err != nil {
		t.Fatalf("write config blob: %v", err)
	}

	manifest := Manifest{
		SchemaVersion: 2,
		MediaType:     MediaTypeOCIManifest,
		Config: Descriptor{
			MediaType: "application/vnd.oci.image.config.v1+json",
			Digest:    configDigest,
			Size:      int64(len(configData)),
		},
		Layers: []Descriptor{{
			MediaType: MediaTypeOCILayer,
			Digest:    layerDigest,
			Size:      int64(len(layerData)),
		}},
	}
	manifestData, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	manifestDigest := "sha256:" + sha256Hex(manifestData)
	if err := os.WriteFile(filepath.Join(blobs, manifestDigest[7:]), manifestData, 0644); err != nil {
		t.Fatalf("write manifest blob: %v", err)
	}

	index := Index{
		SchemaVersion: 2,
		MediaType:     "application/vnd.oci.image.index.v1+json",
		Manifests: []Descriptor{{
			MediaType: MediaTypeOCIManifest,
			Digest:    manifestDigest,
			Size:      int64(len(manifestData)),
		}},
	}
	indexData, err := json.Marshal(index)
	if err != nil {
		t.Fatalf("marshal index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(layout, "index.json"), indexData, 0644); err != nil {
		t.Fatalf("write index.json: %v", err)
	}

	layoutMeta := []byte(`{"imageLayoutVersion":"1.0.0"}`)
	if err := os.WriteFile(filepath.Join(layout, "oci-layout"), layoutMeta, 0644); err != nil {
		t.Fatalf("write oci-layout: %v", err)
	}

	return layout
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
