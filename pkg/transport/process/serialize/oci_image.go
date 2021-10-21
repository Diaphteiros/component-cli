// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier
package serialize

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"path"
	"strings"

	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go"
	ocispecv1 "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/gardener/component-cli/ociclient/cache"
	"github.com/gardener/component-cli/ociclient/oci"
	"github.com/gardener/component-cli/pkg/utils"
)

const (
	ManifestFile = "manifest.json"
	IndexFile    = "index.json"
	BlobsDir     = "blobs"
)

func SerializeOCIArtifact(ociArtifact oci.Artifact, cache cache.Cache) (io.ReadCloser, error) {
	tmpfile, err := ioutil.TempFile("", "")
	if err != nil {
		return nil, fmt.Errorf("unable to create tempfile: %w", err)
	}

	if ociArtifact.IsIndex() {
		if err := serializeImageIndex(cache, ociArtifact.GetIndex(), tmpfile); err != nil {
			return nil, fmt.Errorf("unable to serialize image index: %w", err)
		}
	} else {
		if err := serializeImage(cache, ociArtifact.GetManifest(), ManifestFile, tar.NewWriter(tmpfile)); err != nil {
			return nil, fmt.Errorf("unable to serialize image: %w", err)
		}
	}

	if _, err := tmpfile.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("unable to seek to beginning of tempfile: %w", err)
	}

	return tmpfile, nil
}

func serializeImageIndex(cache cache.Cache, index *oci.Index, w io.Writer) error {
	tw := tar.NewWriter(w)
	defer tw.Close()

	manifestDescs := []ocispecv1.Descriptor{}
	for _, m := range index.Manifests {
		manifestFile := path.Join(BlobsDir, m.Descriptor.Digest.Encoded())
		if err := serializeImage(cache, m, manifestFile, tw); err != nil {
			return fmt.Errorf("unable to serialize image: %w", err)
		}
		manifestDescs = append(manifestDescs, m.Descriptor)
	}

	i := ocispecv1.Index{
		Versioned: specs.Versioned{
			SchemaVersion: 2,
		},
		Manifests:   manifestDescs,
		Annotations: index.Annotations,
	}

	indexBytes, err := json.Marshal(i)
	if err != nil {
		return fmt.Errorf("unable to marshal index manifest: %w", err)
	}

	if err := utils.WriteFileToTARArchive(IndexFile, bytes.NewReader(indexBytes), tw); err != nil {
		return fmt.Errorf("unable to write index manifest: %w", err)
	}

	return nil
}

func serializeImage(cache cache.Cache, manifest *oci.Manifest, manifestFile string, tw *tar.Writer) error {
	manifestBytes, err := json.Marshal(manifest.Data)
	if err != nil {
		return fmt.Errorf("unable to marshal manifest: %w", err)
	}

	if err := utils.WriteFileToTARArchive(manifestFile, bytes.NewReader(manifestBytes), tw); err != nil {
		return fmt.Errorf("unable to write manifest: %w", err)
	}

	configReader, err := cache.Get(manifest.Data.Config)
	if err != nil {
		return fmt.Errorf("unable to get config blob from cache: %w", err)
	}
	defer configReader.Close()

	cfgFile := path.Join(BlobsDir, manifest.Data.Config.Digest.Encoded())
	if err := utils.WriteFileToTARArchive(cfgFile, configReader, tw); err != nil {
		return fmt.Errorf("unable to write config: %w", err)
	}

	for _, layer := range manifest.Data.Layers {
		layerReader, err := cache.Get(layer)
		if err != nil {
			return fmt.Errorf("unable to get layer blob from cache: %w", err)
		}
		defer layerReader.Close()

		layerFile := path.Join(BlobsDir, layer.Digest.Encoded())
		if err := utils.WriteFileToTARArchive(layerFile, layerReader, tw); err != nil {
			return fmt.Errorf("unable to write layer: %w", err)
		}
	}

	return nil
}

func DeserializeOCIArtifact(r io.Reader, cache cache.Cache) (*oci.Artifact, error) {
	tr := tar.NewReader(r)

	buf := bytes.NewBuffer([]byte{})
	isImageIndex := false

	for {
		header, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("unable to read tar header: %w", err)
		}

		if header.Name == ManifestFile {
			if _, err := io.Copy(buf, tr); err != nil {
				return nil, fmt.Errorf("unable to copy %s to buffer: %w", ManifestFile, err)
			}
		} else if header.Name == IndexFile {
			if _, err := io.Copy(buf, tr); err != nil {
				return nil, fmt.Errorf("unable to copy %s to buffer: %w", IndexFile, err)
			}
			isImageIndex = true
		} else if strings.HasPrefix(header.Name, BlobsDir) {
			tmpfile, err := ioutil.TempFile("", "")
			if err != nil {
				return nil, fmt.Errorf("")
			}

			if _, err := io.Copy(tmpfile, tr); err != nil {
				return nil, fmt.Errorf("")
			}

			dgst, err := digest.FromReader(tmpfile)
			if err != nil {
				return nil, fmt.Errorf("unable to calculate digest for blobfile %s: %w", header.Name, err)
			}

			if _, err := tmpfile.Seek(0, io.SeekStart); err != nil {
				return nil, fmt.Errorf("unable to seek to beginning of file: %w", err)
			}

			desc := ocispecv1.Descriptor{
				Digest: dgst,
			}
			if err := cache.Add(desc, tmpfile); err != nil {
				return nil, fmt.Errorf("unable to write blob %+v to cache: %w", desc, err)
			}
		} else {
			return nil, fmt.Errorf("unknown file")
		}
	}

	var ociArtifact *oci.Artifact
	var err error
	if isImageIndex {
		var index ocispecv1.Index
		if err := json.Unmarshal(buf.Bytes(), &index); err != nil {
			return nil, fmt.Errorf("unable to unmarshal image index: %w", err)
		}

		manifests := []*oci.Manifest{}
		for _, m := range index.Manifests {
			blobreader, err := cache.Get(m)
			if err != nil {
				return nil, fmt.Errorf("unable to get manifest blob: %w", err)
			}
			defer blobreader.Close()

			buf := bytes.NewBuffer([]byte{})
			if _, err := io.Copy(buf, blobreader); err != nil {
				return nil, fmt.Errorf("unable to copy %s to buffer: %w", ManifestFile, err)
			}

			var manifest ocispecv1.Manifest
			if err := json.Unmarshal(buf.Bytes(), &manifest); err != nil {
				return nil, fmt.Errorf("unable to unmarshal %s: %w", ManifestFile, err)
			}

			m := oci.Manifest{
				Descriptor: m,
				Data:       &manifest,
			}
			manifests = append(manifests, &m)
		}

		i := oci.Index{
			Manifests:   manifests,
			Annotations: index.Annotations,
		}
		if ociArtifact, err = oci.NewIndexArtifact(&i); err != nil {
			return nil, fmt.Errorf("unable to create oci artifact: %w", err)
		}
	} else {
		var manifest ocispecv1.Manifest
		if err := json.Unmarshal(buf.Bytes(), &manifest); err != nil {
			return nil, fmt.Errorf("unable to unmarshal manifest: %w", err)
		}

		m := oci.Manifest{
			Descriptor: ocispecv1.Descriptor{
				Digest: digest.FromBytes(buf.Bytes()),
			},
			Data: &manifest,
		}
		if ociArtifact, err = oci.NewManifestArtifact(&m); err != nil {
			return nil, fmt.Errorf("unable to create oci artifact: %w", err)
		}
	}

	return ociArtifact, nil
}
