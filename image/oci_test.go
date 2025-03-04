package image

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/types"
	"github.com/opencontainers/go-digest"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func manifestOCI1FromFixture(t *testing.T, src types.ImageSource, fixture string) genericManifest {
	manifest, err := os.ReadFile(filepath.Join("fixtures", fixture))
	require.NoError(t, err)

	m, err := manifestOCI1FromManifest(src, manifest)
	require.NoError(t, err)
	return m
}

func manifestOCI1FromComponentsLikeFixture(configBlob []byte) genericManifest {
	return manifestOCI1FromComponents(imgspecv1.Descriptor{
		MediaType: imgspecv1.MediaTypeImageConfig,
		Size:      5940,
		Digest:    "sha256:9ca4bda0a6b3727a6ffcc43e981cad0f24e2ec79d338f6ba325b4dfd0756fb8f",
		Annotations: map[string]string{
			"test-annotation-1": "one",
		},
	}, nil, configBlob, []imgspecv1.Descriptor{
		{
			MediaType: imgspecv1.MediaTypeImageLayerGzip,
			Digest:    "sha256:6a5a5368e0c2d3e5909184fa28ddfd56072e7ff3ee9a945876f7eee5896ef5bb",
			Size:      51354364,
		},
		{
			MediaType: imgspecv1.MediaTypeImageLayerGzip,
			Digest:    "sha256:1bbf5d58d24c47512e234a5623474acf65ae00d4d1414272a893204f44cc680c",
			Size:      150,
		},
		{
			MediaType: imgspecv1.MediaTypeImageLayerGzip,
			Digest:    "sha256:8f5dc8a4b12c307ac84de90cdd9a7f3915d1be04c9388868ca118831099c67a9",
			Size:      11739507,
			URLs: []string{
				"https://layer.url",
			},
		},
		{
			MediaType: imgspecv1.MediaTypeImageLayerGzip,
			Digest:    "sha256:bbd6b22eb11afce63cc76f6bc41042d99f10d6024c96b655dafba930b8d25909",
			Size:      8841833,
			Annotations: map[string]string{
				"test-annotation-2": "two",
			},
		},
		{
			MediaType: imgspecv1.MediaTypeImageLayerGzip,
			Digest:    "sha256:960e52ecf8200cbd84e70eb2ad8678f4367e50d14357021872c10fa3fc5935fa",
			Size:      291,
		},
	})
}

func TestManifestOCI1FromManifest(t *testing.T) {
	// This just tests that the JSON can be loaded; we test that the parsed
	// values are correctly returned in tests for the individual getter methods.
	_ = manifestOCI1FromFixture(t, unusedImageSource{}, "oci1.json")

	_, err := manifestOCI1FromManifest(nil, []byte{})
	assert.Error(t, err)
}

func TestManifestOCI1FromComponents(t *testing.T) {
	// This just smoke-tests that the manifest can be created; we test that the parsed
	// values are correctly returned in tests for the individual getter methods.
	_ = manifestOCI1FromComponentsLikeFixture(nil)
}

func TestManifestOCI1Serialize(t *testing.T) {
	for _, m := range []genericManifest{
		manifestOCI1FromFixture(t, unusedImageSource{}, "oci1.json"),
		manifestOCI1FromComponentsLikeFixture(nil),
	} {
		serialized, err := m.serialize()
		require.NoError(t, err)
		var contents map[string]interface{}
		err = json.Unmarshal(serialized, &contents)
		require.NoError(t, err)

		original, err := os.ReadFile("fixtures/oci1.json")
		require.NoError(t, err)
		var originalContents map[string]interface{}
		err = json.Unmarshal(original, &originalContents)
		require.NoError(t, err)

		// We would ideally like to compare “serialized” with some transformation of
		// “original”, but the ordering of fields in JSON maps is undefined, so this is
		// easier.
		assert.Equal(t, originalContents, contents)
	}
}

func TestManifestOCI1ManifestMIMEType(t *testing.T) {
	for _, m := range []genericManifest{
		manifestOCI1FromFixture(t, unusedImageSource{}, "oci1.json"),
		manifestOCI1FromComponentsLikeFixture(nil),
	} {
		assert.Equal(t, imgspecv1.MediaTypeImageManifest, m.manifestMIMEType())
	}
}

func TestManifestOCI1ConfigInfo(t *testing.T) {
	for _, m := range []genericManifest{
		manifestOCI1FromFixture(t, unusedImageSource{}, "oci1.json"),
		manifestOCI1FromComponentsLikeFixture(nil),
	} {
		assert.Equal(t, types.BlobInfo{
			Size:   5940,
			Digest: "sha256:9ca4bda0a6b3727a6ffcc43e981cad0f24e2ec79d338f6ba325b4dfd0756fb8f",
			Annotations: map[string]string{
				"test-annotation-1": "one",
			},
			MediaType: "application/vnd.oci.image.config.v1+json",
		}, m.ConfigInfo())
	}
}

func TestManifestOCI1ConfigBlob(t *testing.T) {
	realConfigJSON, err := os.ReadFile("fixtures/oci1-config.json")
	require.NoError(t, err)

	for _, c := range []struct {
		cbISfn func(digest digest.Digest) (io.ReadCloser, int64, error)
		blob   []byte
	}{
		// Success
		{func(digest digest.Digest) (io.ReadCloser, int64, error) {
			return io.NopCloser(bytes.NewReader(realConfigJSON)), int64(len(realConfigJSON)), nil
		}, realConfigJSON},
		// Various kinds of failures
		{nil, nil},
		{func(digest digest.Digest) (io.ReadCloser, int64, error) {
			return nil, -1, errors.New("Error returned from GetBlob")
		}, nil},
		{func(digest digest.Digest) (io.ReadCloser, int64, error) {
			reader, writer := io.Pipe()
			err = writer.CloseWithError(errors.New("Expected error reading input in ConfigBlob"))
			require.NoError(t, err)
			return reader, 1, nil
		}, nil},
		{func(digest digest.Digest) (io.ReadCloser, int64, error) {
			nonmatchingJSON := []byte("This does not match ConfigDescriptor.Digest")
			return io.NopCloser(bytes.NewReader(nonmatchingJSON)), int64(len(nonmatchingJSON)), nil
		}, nil},
	} {
		var src types.ImageSource
		if c.cbISfn != nil {
			src = configBlobImageSource{unusedImageSource{}, c.cbISfn}
		} else {
			src = nil
		}
		m := manifestOCI1FromFixture(t, src, "oci1.json")
		blob, err := m.ConfigBlob(context.Background())
		if c.blob != nil {
			assert.NoError(t, err)
			assert.Equal(t, c.blob, blob)
		} else {
			assert.Error(t, err)
		}
	}

	// Generally configBlob should match ConfigInfo; we don’t quite need it to, and this will
	// guarantee that the returned object is returning the original contents instead
	// of reading an object from elsewhere.
	configBlob := []byte("config blob which does not match ConfigInfo")
	// This just tests that the manifest can be created; we test that the parsed
	// values are correctly returned in tests for the individual getter methods.
	m := manifestOCI1FromComponentsLikeFixture(configBlob)
	cb, err := m.ConfigBlob(context.Background())
	require.NoError(t, err)
	assert.Equal(t, configBlob, cb)
}

func TestManifestOCI1LayerInfo(t *testing.T) {
	for _, m := range []genericManifest{
		manifestOCI1FromFixture(t, unusedImageSource{}, "oci1.json"),
		manifestOCI1FromComponentsLikeFixture(nil),
	} {
		assert.Equal(t, []types.BlobInfo{
			{
				Digest:    "sha256:6a5a5368e0c2d3e5909184fa28ddfd56072e7ff3ee9a945876f7eee5896ef5bb",
				Size:      51354364,
				MediaType: imgspecv1.MediaTypeImageLayerGzip,
			},
			{
				Digest:    "sha256:1bbf5d58d24c47512e234a5623474acf65ae00d4d1414272a893204f44cc680c",
				Size:      150,
				MediaType: imgspecv1.MediaTypeImageLayerGzip,
			},
			{
				Digest: "sha256:8f5dc8a4b12c307ac84de90cdd9a7f3915d1be04c9388868ca118831099c67a9",
				Size:   11739507,
				URLs: []string{
					"https://layer.url",
				},
				MediaType: imgspecv1.MediaTypeImageLayerGzip,
			},
			{
				Digest: "sha256:bbd6b22eb11afce63cc76f6bc41042d99f10d6024c96b655dafba930b8d25909",
				Size:   8841833,
				Annotations: map[string]string{
					"test-annotation-2": "two",
				},
				MediaType: imgspecv1.MediaTypeImageLayerGzip,
			},
			{
				Digest:    "sha256:960e52ecf8200cbd84e70eb2ad8678f4367e50d14357021872c10fa3fc5935fa",
				Size:      291,
				MediaType: imgspecv1.MediaTypeImageLayerGzip,
			},
		}, m.LayerInfos())
	}
}

func TestManifestOCI1EmbeddedDockerReferenceConflicts(t *testing.T) {
	for _, m := range []genericManifest{
		manifestOCI1FromFixture(t, unusedImageSource{}, "oci1.json"),
		manifestOCI1FromComponentsLikeFixture(nil),
	} {
		for _, name := range []string{"busybox", "example.com:5555/ns/repo:tag"} {
			ref, err := reference.ParseNormalizedNamed(name)
			require.NoError(t, err)
			conflicts := m.EmbeddedDockerReferenceConflicts(ref)
			assert.False(t, conflicts)
		}
	}
}

func TestManifestOCI1Inspect(t *testing.T) {
	configJSON, err := os.ReadFile("fixtures/oci1-config.json")
	require.NoError(t, err)

	m := manifestOCI1FromComponentsLikeFixture(configJSON)
	ii, err := m.Inspect(context.Background())
	require.NoError(t, err)
	created := time.Date(2016, 9, 23, 23, 20, 45, 789764590, time.UTC)
	assert.Equal(t, types.ImageInspectInfo{
		Tag:           "",
		Created:       &created,
		DockerVersion: "1.12.1",
		Labels:        map[string]string{},
		Architecture:  "amd64",
		Os:            "linux",
		Layers: []string{
			"sha256:6a5a5368e0c2d3e5909184fa28ddfd56072e7ff3ee9a945876f7eee5896ef5bb",
			"sha256:1bbf5d58d24c47512e234a5623474acf65ae00d4d1414272a893204f44cc680c",
			"sha256:8f5dc8a4b12c307ac84de90cdd9a7f3915d1be04c9388868ca118831099c67a9",
			"sha256:bbd6b22eb11afce63cc76f6bc41042d99f10d6024c96b655dafba930b8d25909",
			"sha256:960e52ecf8200cbd84e70eb2ad8678f4367e50d14357021872c10fa3fc5935fa",
		},
		Env: []string{
			"PATH=/usr/local/apache2/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
			"HTTPD_PREFIX=/usr/local/apache2",
			"HTTPD_VERSION=2.4.23",
			"HTTPD_SHA1=5101be34ac4a509b245adb70a56690a84fcc4e7f",
			"HTTPD_BZ2_URL=https://www.apache.org/dyn/closer.cgi?action=download&filename=httpd/httpd-2.4.23.tar.bz2",
			"HTTPD_ASC_URL=https://www.apache.org/dist/httpd/httpd-2.4.23.tar.bz2.asc",
		},
	}, *ii)

	// nil configBlob will trigger an error in m.ConfigBlob()
	m = manifestOCI1FromComponentsLikeFixture(nil)
	_, err = m.Inspect(context.Background())
	assert.Error(t, err)

	m = manifestOCI1FromComponentsLikeFixture([]byte("invalid JSON"))
	_, err = m.Inspect(context.Background())
	assert.Error(t, err)
}

func TestManifestOCI1UpdatedImageNeedsLayerDiffIDs(t *testing.T) {
	for _, m := range []genericManifest{
		manifestOCI1FromFixture(t, unusedImageSource{}, "oci1.json"),
		manifestOCI1FromComponentsLikeFixture(nil),
	} {
		assert.False(t, m.UpdatedImageNeedsLayerDiffIDs(types.ManifestUpdateOptions{
			ManifestMIMEType: manifest.DockerV2Schema2MediaType,
		}))
	}
}

// oci1ImageSource is plausible enough for schema conversions in manifestOCI1.UpdatedImage() to work.
type oci1ImageSource struct {
	configBlobImageSource
	ref reference.Named
}

func (OCIis *oci1ImageSource) Reference() types.ImageReference {
	return refImageReferenceMock{OCIis.ref}
}

func newOCI1ImageSource(t *testing.T, dockerRef string) *oci1ImageSource {
	realConfigJSON, err := os.ReadFile("fixtures/oci1-config.json")
	require.NoError(t, err)

	ref, err := reference.ParseNormalizedNamed(dockerRef)
	require.NoError(t, err)

	return &oci1ImageSource{
		configBlobImageSource: configBlobImageSource{
			f: func(digest digest.Digest) (io.ReadCloser, int64, error) {
				return io.NopCloser(bytes.NewReader(realConfigJSON)), int64(len(realConfigJSON)), nil
			},
		},
		ref: ref,
	}
}

func TestManifestOCI1UpdatedImage(t *testing.T) {
	originalSrc := newOCI1ImageSource(t, "httpd:latest")
	original := manifestOCI1FromFixture(t, originalSrc, "oci1.json")

	// LayerInfos:
	layerInfos := append(original.LayerInfos()[1:], original.LayerInfos()[0])
	res, err := original.UpdatedImage(context.Background(), types.ManifestUpdateOptions{
		LayerInfos: layerInfos,
	})
	require.NoError(t, err)
	assert.Equal(t, layerInfos, res.LayerInfos())
	_, err = original.UpdatedImage(context.Background(), types.ManifestUpdateOptions{
		LayerInfos: append(layerInfos, layerInfos[0]),
	})
	assert.Error(t, err)

	// EmbeddedDockerReference:
	// … is ignored
	embeddedRef, err := reference.ParseNormalizedNamed("busybox")
	require.NoError(t, err)
	res, err = original.UpdatedImage(context.Background(), types.ManifestUpdateOptions{
		EmbeddedDockerReference: embeddedRef,
	})
	require.NoError(t, err)
	nonEmbeddedRef, err := reference.ParseNormalizedNamed("notbusybox:notlatest")
	require.NoError(t, err)
	conflicts := res.EmbeddedDockerReferenceConflicts(nonEmbeddedRef)
	assert.False(t, conflicts)

	// ManifestMIMEType:
	// Only smoke-test the valid conversions, detailed tests are below. (This also verifies that “original” is not affected.)
	for _, mime := range []string{
		manifest.DockerV2Schema2MediaType,
	} {
		_, err = original.UpdatedImage(context.Background(), types.ManifestUpdateOptions{
			ManifestMIMEType: mime,
			InformationOnly: types.ManifestUpdateInformation{
				Destination: &memoryImageDest{ref: originalSrc.ref},
			},
		})
		assert.NoError(t, err, mime)
	}
	for _, mime := range []string{
		imgspecv1.MediaTypeImageManifest, // This indicates a confused caller, not a no-op.
		"this is invalid",
	} {
		_, err = original.UpdatedImage(context.Background(), types.ManifestUpdateOptions{
			ManifestMIMEType: mime,
		})
		assert.Error(t, err, mime)
	}

	// m hasn’t been changed:
	m2 := manifestOCI1FromFixture(t, originalSrc, "oci1.json")
	typedOriginal, ok := original.(*manifestOCI1)
	require.True(t, ok)
	typedM2, ok := m2.(*manifestOCI1)
	require.True(t, ok)
	assert.Equal(t, *typedM2, *typedOriginal)
}

func TestManifestOCI1ConvertToManifestSchema1(t *testing.T) {
	originalSrc := newOCI1ImageSource(t, "httpd-copy:latest")
	original := manifestOCI1FromFixture(t, originalSrc, "oci1.json")
	memoryDest := &memoryImageDest{ref: originalSrc.ref}
	res, err := original.UpdatedImage(context.Background(), types.ManifestUpdateOptions{
		ManifestMIMEType: manifest.DockerV2Schema1SignedMediaType,
		InformationOnly: types.ManifestUpdateInformation{
			Destination: memoryDest,
		},
	})
	require.NoError(t, err)

	convertedJSON, mt, err := res.Manifest(context.Background())
	require.NoError(t, err)
	assert.Equal(t, manifest.DockerV2Schema1SignedMediaType, mt)

	byHandJSON, err := os.ReadFile("fixtures/oci1-to-schema1.json")
	require.NoError(t, err)
	var converted, byHand map[string]interface{}
	err = json.Unmarshal(byHandJSON, &byHand)
	require.NoError(t, err)
	err = json.Unmarshal(convertedJSON, &converted)
	require.NoError(t, err)
	delete(byHand, "signatures")
	delete(converted, "signatures")
	assert.Equal(t, byHand, converted)

	assert.Equal(t, GzippedEmptyLayer, memoryDest.storedBlobs[GzippedEmptyLayerDigest])

	// Conversion to schema1 together with changing LayerInfos works as expected (which requires
	// handling schema1 empty layers):
	updatedLayers, updatedLayersCopy := modifiedLayerInfos(t, original.LayerInfos())
	res, err = original.UpdatedImage(context.Background(), types.ManifestUpdateOptions{
		LayerInfos:       updatedLayers,
		ManifestMIMEType: manifest.DockerV2Schema1SignedMediaType,
		InformationOnly: types.ManifestUpdateInformation{
			Destination: memoryDest,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, updatedLayersCopy, updatedLayers) // updatedLayers have not been modified in place
	convertedJSON, mt, err = res.Manifest(context.Background())
	require.NoError(t, err)
	assert.Equal(t, manifest.DockerV2Schema1SignedMediaType, mt)
	// Layers have been updated as expected
	s1Manifest, err := manifestSchema1FromManifest(convertedJSON)
	require.NoError(t, err)
	assert.Equal(t, []types.BlobInfo{
		{Digest: "sha256:6a5a5368e0c2d3e5909184fa28ddfd56072e7ff3ee9a945876f7eee5896ef5ba", Size: -1},
		{Digest: GzippedEmptyLayerDigest, Size: -1},
		{Digest: GzippedEmptyLayerDigest, Size: -1},
		{Digest: GzippedEmptyLayerDigest, Size: -1},
		{Digest: "sha256:1bbf5d58d24c47512e234a5623474acf65ae00d4d1414272a893204f44cc680d", Size: -1},
		{Digest: GzippedEmptyLayerDigest, Size: -1},
		{Digest: "sha256:8f5dc8a4b12c307ac84de90cdd9a7f3915d1be04c9388868ca118831099c67a8", Size: -1},
		{Digest: GzippedEmptyLayerDigest, Size: -1},
		{Digest: GzippedEmptyLayerDigest, Size: -1},
		{Digest: GzippedEmptyLayerDigest, Size: -1},
		{Digest: GzippedEmptyLayerDigest, Size: -1},
		{Digest: "sha256:bbd6b22eb11afce63cc76f6bc41042d99f10d6024c96b655dafba930b8d25908", Size: -1},
		{Digest: "sha256:960e52ecf8200cbd84e70eb2ad8678f4367e50d14357021872c10fa3fc5935fb", Size: -1},
		{Digest: GzippedEmptyLayerDigest, Size: -1},
		{Digest: GzippedEmptyLayerDigest, Size: -1},
	}, s1Manifest.LayerInfos())

	// FIXME? Test also the various failure cases, if only to see that we don't crash?
}

func TestConvertToManifestSchema2(t *testing.T) {
	originalSrc := newOCI1ImageSource(t, "httpd-copy:latest")
	original := manifestOCI1FromFixture(t, originalSrc, "oci1.json")
	res, err := original.UpdatedImage(context.Background(), types.ManifestUpdateOptions{
		ManifestMIMEType: manifest.DockerV2Schema2MediaType,
	})
	require.NoError(t, err)

	convertedJSON, mt, err := res.Manifest(context.Background())
	require.NoError(t, err)
	assert.Equal(t, manifest.DockerV2Schema2MediaType, mt)

	byHandJSON, err := os.ReadFile("fixtures/oci1-to-schema2.json")
	require.NoError(t, err)
	var converted, byHand map[string]interface{}
	err = json.Unmarshal(byHandJSON, &byHand)
	require.NoError(t, err)
	err = json.Unmarshal(convertedJSON, &converted)
	require.NoError(t, err)
	assert.Equal(t, byHand, converted)

	// FIXME? Test also the various failure cases, if only to see that we don't crash?
}

func TestConvertToManifestSchema2AllMediaTypes(t *testing.T) {
	originalSrc := newOCI1ImageSource(t, "httpd-copy:latest")
	original := manifestOCI1FromFixture(t, originalSrc, "oci1-all-media-types.json")
	_, err := original.UpdatedImage(context.Background(), types.ManifestUpdateOptions{
		ManifestMIMEType: manifest.DockerV2Schema2MediaType,
	})
	require.Error(t, err) // zstd compression is not supported for docker images
}

func TestConvertToV2S2WithInvalidMIMEType(t *testing.T) {
	originalSrc := newOCI1ImageSource(t, "httpd-copy:latest")
	manifest, err := os.ReadFile(filepath.Join("fixtures", "oci1-invalid-media-type.json"))
	require.NoError(t, err)

	_, err = manifestOCI1FromManifest(originalSrc, manifest)
	require.NoError(t, err)
}
