package image

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	MediaTypeOCIManifest = "application/vnd.oci.image.manifest.v1+json"
	MediaTypeDockerManifest = "application/vnd.docker.distribution.manifest.v2+json"
	MediaTypeOCILayer = "application/vnd.oci.image.layer.v1.tar+gzip"
	MediaTypeDockerLayer = "application/vnd.docker.image.rootfs.diff.tar.gzip"
)

// Descriptor identifies a blob in an OCI/Docker image layout.
type Descriptor struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
}

// Manifest is an OCI or Docker Schema 2 image manifest.
type Manifest struct {
	SchemaVersion int          `json:"schemaVersion"`
	MediaType     string       `json:"mediaType,omitempty"`
	Config        Descriptor   `json:"config"`
	Layers        []Descriptor `json:"layers"`
}

// Index is an OCI image index (or Docker manifest list) pointing at manifests.
type Index struct {
	SchemaVersion int          `json:"schemaVersion"`
	MediaType     string       `json:"mediaType,omitempty"`
	Manifests     []Descriptor `json:"manifests"`
}

// ParseManifest decodes an OCI or Docker Schema 2 image manifest from JSON.
func ParseManifest(data []byte) (*Manifest, error) {
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("decode manifest: %w", err)
	}
	if manifest.SchemaVersion != 2 {
		return nil, fmt.Errorf("unsupported manifest schemaVersion %d", manifest.SchemaVersion)
	}
	if len(manifest.Layers) == 0 {
		return nil, fmt.Errorf("manifest has no layers")
	}
	if manifest.Config.Digest == "" {
		return nil, fmt.Errorf("manifest missing config descriptor")
	}
	return &manifest, nil
}

// ParseIndex decodes an OCI image index from JSON.
func ParseIndex(data []byte) (*Index, error) {
	var index Index
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("decode index: %w", err)
	}
	if index.SchemaVersion != 2 {
		return nil, fmt.Errorf("unsupported index schemaVersion %d", index.SchemaVersion)
	}
	if len(index.Manifests) == 0 {
		return nil, fmt.Errorf("index has no manifests")
	}
	return &index, nil
}

// LayerDigests returns the digest of each layer in order.
func (m *Manifest) LayerDigests() []string {
	digests := make([]string, len(m.Layers))
	for i, layer := range m.Layers {
		digests[i] = layer.Digest
	}
	return digests
}

// blobPath converts a content digest to a path under blobs/sha256/.
func blobPath(blobsRoot, digest string) (string, error) {
	algo, hash, ok := strings.Cut(digest, ":")
	if !ok || algo != "sha256" || hash == "" {
		return "", fmt.Errorf("unsupported digest %q", digest)
	}
	return fmt.Sprintf("%s/sha256/%s", blobsRoot, hash), nil
}
