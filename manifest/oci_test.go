package manifest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/containers/image/v5/pkg/compression"
	"github.com/containers/image/v5/types"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSupportedOCI1MediaType(t *testing.T) {
	type testData struct {
		m        string
		mustFail bool
	}
	data := []testData{
		{
			imgspecv1.MediaTypeDescriptor,
			false,
		},
		{
			imgspecv1.MediaTypeImageConfig,
			false,
		},
		{
			imgspecv1.MediaTypeImageLayer,
			false,
		},
		{
			imgspecv1.MediaTypeImageLayerGzip,
			false,
		},
		{
			imgspecv1.MediaTypeImageLayerNonDistributable,
			false,
		},
		{
			imgspecv1.MediaTypeImageLayerNonDistributableGzip,
			false,
		},
		{
			imgspecv1.MediaTypeImageLayerNonDistributableZstd,
			false,
		},
		{
			imgspecv1.MediaTypeImageLayerZstd,
			false,
		},
		{
			imgspecv1.MediaTypeImageManifest,
			false,
		},
		{
			imgspecv1.MediaTypeLayoutHeader,
			false,
		},
		{
			"application/vnd.oci.image.layer.nondistributable.v1.tar+unknown",
			true,
		},
	}
	for _, d := range data {
		err := SupportedOCI1MediaType(d.m)
		if d.mustFail {
			assert.NotNil(t, err)
		} else {
			assert.Nil(t, err)
		}
	}
}

func TestOCI1FromManifest(t *testing.T) {
	validManifest, err := os.ReadFile(filepath.Join("fixtures", "ociv1.manifest.json"))
	require.NoError(t, err)

	parser := func(m []byte) error {
		_, err := OCI1FromManifest(m)
		return err
	}
	// Schema mismatch is rejected
	testManifestFixturesAreRejected(t, parser, []string{
		"schema2-to-schema1-by-docker.json",
		// Not "v2s2.manifest.json" yet, without mediaType the two are too similar to tell the difference.
		"v2list.manifest.json",
		"ociv1.image.index.json",
	})
	// Extra fields are rejected
	testValidManifestWithExtraFieldsIsRejected(t, parser, validManifest, []string{"fsLayers", "history", "manifests"})
}

func TestUpdateLayerInfosOCIGzipToZstd(t *testing.T) {
	bytes, err := os.ReadFile("fixtures/ociv1.manifest.json")
	assert.Nil(t, err)

	manifest, err := OCI1FromManifest(bytes)
	assert.Nil(t, err)

	err = manifest.UpdateLayerInfos([]types.BlobInfo{
		{
			Digest:               "sha256:e692418e4cbaf90ca69d05a66403747baa33ee08806650b51fab815ad7fc331f",
			Size:                 32654,
			MediaType:            imgspecv1.MediaTypeImageLayerGzip,
			CompressionOperation: types.Compress,
			CompressionAlgorithm: &compression.Zstd,
		},
		{
			Digest:               "sha256:3c3a4604a545cdc127456d94e421cd355bca5b528f4a9c1905b15da2eb4a4c6b",
			Size:                 16724,
			MediaType:            imgspecv1.MediaTypeImageLayerGzip,
			CompressionOperation: types.Compress,
			CompressionAlgorithm: &compression.Zstd,
		},
		{
			Digest:               "sha256:ec4b8955958665577945c89419d1af06b5f7636b4ac3da7f12184802ad867736",
			Size:                 73109,
			MediaType:            imgspecv1.MediaTypeImageLayerGzip,
			CompressionOperation: types.Compress,
			CompressionAlgorithm: &compression.Zstd,
		},
	})
	assert.Nil(t, err)

	updatedManifestBytes, err := manifest.Serialize()
	assert.Nil(t, err)

	bytes, err = os.ReadFile("fixtures/ociv1.zstd.manifest.json")
	assert.Nil(t, err)

	expectedManifest, err := OCI1FromManifest(bytes)
	assert.Nil(t, err)

	expectedManifestBytes, err := expectedManifest.Serialize()
	assert.Nil(t, err)

	assert.Equal(t, string(expectedManifestBytes), string(updatedManifestBytes))
}

func TestUpdateLayerInfosOCIZstdToGzip(t *testing.T) {
	bytes, err := os.ReadFile("fixtures/ociv1.zstd.manifest.json")
	assert.Nil(t, err)

	manifest, err := OCI1FromManifest(bytes)
	assert.Nil(t, err)

	err = manifest.UpdateLayerInfos([]types.BlobInfo{
		{
			Digest:               "sha256:e692418e4cbaf90ca69d05a66403747baa33ee08806650b51fab815ad7fc331f",
			Size:                 32654,
			MediaType:            imgspecv1.MediaTypeImageLayerZstd,
			CompressionOperation: types.Compress,
			CompressionAlgorithm: &compression.Gzip,
		},
		{
			Digest:               "sha256:3c3a4604a545cdc127456d94e421cd355bca5b528f4a9c1905b15da2eb4a4c6b",
			Size:                 16724,
			MediaType:            imgspecv1.MediaTypeImageLayerZstd,
			CompressionOperation: types.Compress,
			CompressionAlgorithm: &compression.Gzip,
		},
		{
			Digest:               "sha256:ec4b8955958665577945c89419d1af06b5f7636b4ac3da7f12184802ad867736",
			Size:                 73109,
			MediaType:            imgspecv1.MediaTypeImageLayerZstd,
			CompressionOperation: types.Compress,
			CompressionAlgorithm: &compression.Gzip,
		},
	})
	assert.Nil(t, err)

	updatedManifestBytes, err := manifest.Serialize()
	assert.Nil(t, err)

	bytes, err = os.ReadFile("fixtures/ociv1.manifest.json")
	assert.Nil(t, err)

	expectedManifest, err := OCI1FromManifest(bytes)
	assert.Nil(t, err)

	expectedManifestBytes, err := expectedManifest.Serialize()
	assert.Nil(t, err)

	assert.Equal(t, string(expectedManifestBytes), string(updatedManifestBytes))
}

func TestUpdateLayerInfosOCIZstdToUncompressed(t *testing.T) {
	bytes, err := os.ReadFile("fixtures/ociv1.zstd.manifest.json")
	assert.Nil(t, err)

	manifest, err := OCI1FromManifest(bytes)
	assert.Nil(t, err)

	err = manifest.UpdateLayerInfos([]types.BlobInfo{
		{
			Digest:               "sha256:e692418e4cbaf90ca69d05a66403747baa33ee08806650b51fab815ad7fc331f",
			Size:                 32654,
			MediaType:            imgspecv1.MediaTypeImageLayerZstd,
			CompressionOperation: types.Decompress,
		},
		{
			Digest:               "sha256:3c3a4604a545cdc127456d94e421cd355bca5b528f4a9c1905b15da2eb4a4c6b",
			Size:                 16724,
			MediaType:            imgspecv1.MediaTypeImageLayerZstd,
			CompressionOperation: types.Decompress,
		},
		{
			Digest:               "sha256:ec4b8955958665577945c89419d1af06b5f7636b4ac3da7f12184802ad867736",
			Size:                 73109,
			MediaType:            imgspecv1.MediaTypeImageLayerZstd,
			CompressionOperation: types.Decompress,
		},
	})
	assert.Nil(t, err)

	updatedManifestBytes, err := manifest.Serialize()
	assert.Nil(t, err)

	bytes, err = os.ReadFile("fixtures/ociv1.uncompressed.manifest.json")
	assert.Nil(t, err)

	expectedManifest, err := OCI1FromManifest(bytes)
	assert.Nil(t, err)

	expectedManifestBytes, err := expectedManifest.Serialize()
	assert.Nil(t, err)

	assert.Equal(t, string(expectedManifestBytes), string(updatedManifestBytes))
}

func TestUpdateLayerInfosInvalidCompressionOperation(t *testing.T) {
	bytes, err := os.ReadFile("fixtures/ociv1.zstd.manifest.json")
	assert.Nil(t, err)

	manifest, err := OCI1FromManifest(bytes)
	assert.Nil(t, err)

	err = manifest.UpdateLayerInfos([]types.BlobInfo{
		{
			Digest:               "sha256:e692418e4cbaf90ca69d05a66403747baa33ee08806650b51fab815ad7fc331f",
			Size:                 32654,
			MediaType:            imgspecv1.MediaTypeImageLayerZstd,
			CompressionOperation: types.Compress,
			CompressionAlgorithm: &compression.Gzip,
		},
		{
			Digest:               "sha256:3c3a4604a545cdc127456d94e421cd355bca5b528f4a9c1905b15da2eb4a4c6b",
			Size:                 16724,
			MediaType:            imgspecv1.MediaTypeImageLayerZstd,
			CompressionOperation: 42, // MUST fail here
			CompressionAlgorithm: &compression.Gzip,
		},
		{
			Digest:               "sha256:ec4b8955958665577945c89419d1af06b5f7636b4ac3da7f12184802ad867736",
			Size:                 73109,
			MediaType:            imgspecv1.MediaTypeImageLayerZstd,
			CompressionOperation: types.Compress,
			CompressionAlgorithm: &compression.Gzip,
		},
	})
	assert.NotNil(t, err)
}

func TestUpdateLayerInfosInvalidCompressionAlgorithm(t *testing.T) {
	bytes, err := os.ReadFile("fixtures/ociv1.zstd.manifest.json")
	assert.Nil(t, err)

	manifest, err := OCI1FromManifest(bytes)
	assert.Nil(t, err)

	customCompression := compression.Algorithm{}
	err = manifest.UpdateLayerInfos([]types.BlobInfo{
		{
			Digest:               "sha256:e692418e4cbaf90ca69d05a66403747baa33ee08806650b51fab815ad7fc331f",
			Size:                 32654,
			MediaType:            imgspecv1.MediaTypeImageLayerZstd,
			CompressionOperation: types.Compress,
			CompressionAlgorithm: &compression.Gzip,
		},
		{
			Digest:               "sha256:3c3a4604a545cdc127456d94e421cd355bca5b528f4a9c1905b15da2eb4a4c6b",
			Size:                 16724,
			MediaType:            imgspecv1.MediaTypeImageLayerZstd,
			CompressionOperation: 42,
			CompressionAlgorithm: &compression.Gzip,
		},
		{
			Digest:               "sha256:ec4b8955958665577945c89419d1af06b5f7636b4ac3da7f12184802ad867736",
			Size:                 73109,
			MediaType:            imgspecv1.MediaTypeImageLayerZstd,
			CompressionOperation: types.Compress,
			CompressionAlgorithm: &customCompression, // MUST fail here
		},
	})
	assert.NotNil(t, err)
}

func TestUpdateLayerInfosOCIGzipToUncompressed(t *testing.T) {
	bytes, err := os.ReadFile("fixtures/ociv1.manifest.json")
	assert.Nil(t, err)

	manifest, err := OCI1FromManifest(bytes)
	assert.Nil(t, err)

	err = manifest.UpdateLayerInfos([]types.BlobInfo{
		{
			Digest:               "sha256:e692418e4cbaf90ca69d05a66403747baa33ee08806650b51fab815ad7fc331f",
			Size:                 32654,
			MediaType:            imgspecv1.MediaTypeImageLayerGzip,
			CompressionOperation: types.Decompress,
		},
		{
			Digest:               "sha256:3c3a4604a545cdc127456d94e421cd355bca5b528f4a9c1905b15da2eb4a4c6b",
			Size:                 16724,
			MediaType:            imgspecv1.MediaTypeImageLayerGzip,
			CompressionOperation: types.Decompress,
		},
		{
			Digest:               "sha256:ec4b8955958665577945c89419d1af06b5f7636b4ac3da7f12184802ad867736",
			Size:                 73109,
			MediaType:            imgspecv1.MediaTypeImageLayerGzip,
			CompressionOperation: types.Decompress,
		},
	})
	assert.Nil(t, err)

	updatedManifestBytes, err := manifest.Serialize()
	assert.Nil(t, err)

	bytes, err = os.ReadFile("fixtures/ociv1.uncompressed.manifest.json")
	assert.Nil(t, err)

	expectedManifest, err := OCI1FromManifest(bytes)
	assert.Nil(t, err)

	expectedManifestBytes, err := expectedManifest.Serialize()
	assert.Nil(t, err)

	assert.Equal(t, string(expectedManifestBytes), string(updatedManifestBytes))
}

func TestUpdateLayerInfosOCINondistributableToGzip(t *testing.T) {
	bytes, err := os.ReadFile("fixtures/ociv1.nondistributable.manifest.json")
	assert.Nil(t, err)

	manifest, err := OCI1FromManifest(bytes)
	assert.Nil(t, err)

	err = manifest.UpdateLayerInfos([]types.BlobInfo{
		{
			Digest:               "sha256:e692418e4cbaf90ca69d05a66403747baa33ee08806650b51fab815ad7fc331f",
			Size:                 32654,
			MediaType:            imgspecv1.MediaTypeImageLayerGzip,
			CompressionOperation: types.Compress,
			CompressionAlgorithm: &compression.Gzip,
		},
	})
	assert.Nil(t, err)

	updatedManifestBytes, err := manifest.Serialize()
	assert.Nil(t, err)

	bytes, err = os.ReadFile("fixtures/ociv1.nondistributable.gzip.manifest.json")
	assert.Nil(t, err)

	expectedManifest, err := OCI1FromManifest(bytes)
	assert.Nil(t, err)

	expectedManifestBytes, err := expectedManifest.Serialize()
	assert.Nil(t, err)

	assert.Equal(t, string(expectedManifestBytes), string(updatedManifestBytes))
}

func TestUpdateLayerInfosOCINondistributableToZstd(t *testing.T) {
	bytes, err := os.ReadFile("fixtures/ociv1.nondistributable.manifest.json")
	assert.Nil(t, err)

	manifest, err := OCI1FromManifest(bytes)
	assert.Nil(t, err)

	err = manifest.UpdateLayerInfos([]types.BlobInfo{
		{
			Digest:               "sha256:e692418e4cbaf90ca69d05a66403747baa33ee08806650b51fab815ad7fc331f",
			Size:                 32654,
			MediaType:            imgspecv1.MediaTypeImageLayerGzip,
			CompressionOperation: types.Compress,
			CompressionAlgorithm: &compression.Zstd,
		},
	})
	assert.Nil(t, err)

	updatedManifestBytes, err := manifest.Serialize()
	assert.Nil(t, err)

	bytes, err = os.ReadFile("fixtures/ociv1.nondistributable.zstd.manifest.json")
	assert.Nil(t, err)

	expectedManifest, err := OCI1FromManifest(bytes)
	assert.Nil(t, err)

	expectedManifestBytes, err := expectedManifest.Serialize()
	assert.Nil(t, err)

	assert.Equal(t, string(expectedManifestBytes), string(updatedManifestBytes))
}

func TestUpdateLayerInfosOCINondistributableGzipToUncompressed(t *testing.T) {
	bytes, err := os.ReadFile("fixtures/ociv1.nondistributable.gzip.manifest.json")
	assert.Nil(t, err)

	manifest, err := OCI1FromManifest(bytes)
	assert.Nil(t, err)

	err = manifest.UpdateLayerInfos([]types.BlobInfo{
		{
			Digest:               "sha256:e692418e4cbaf90ca69d05a66403747baa33ee08806650b51fab815ad7fc331f",
			Size:                 32654,
			MediaType:            imgspecv1.MediaTypeImageLayerGzip,
			CompressionOperation: types.Decompress,
		},
	})
	assert.Nil(t, err)

	updatedManifestBytes, err := manifest.Serialize()
	assert.Nil(t, err)

	bytes, err = os.ReadFile("fixtures/ociv1.nondistributable.manifest.json")
	assert.Nil(t, err)

	expectedManifest, err := OCI1FromManifest(bytes)
	assert.Nil(t, err)

	expectedManifestBytes, err := expectedManifest.Serialize()
	assert.Nil(t, err)

	assert.Equal(t, string(expectedManifestBytes), string(updatedManifestBytes))
}
