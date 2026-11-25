package image

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	crv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func pushTestImage(t *testing.T, srvURL, repoTag string) (name.Reference, crv1.Hash) {
	t.Helper()

	host := strings.TrimPrefix(srvURL, "http://")
	ref, err := name.ParseReference(host + "/" + repoTag)
	require.NoError(t, err)

	layer := static.NewLayer([]byte("hello netdrill"), types.DockerLayer)
	img, err := mutate.AppendLayers(empty.Image, layer)
	require.NoError(t, err)

	require.NoError(t, remote.Write(ref, img))

	digest, err := img.Digest()
	require.NoError(t, err)

	return ref, digest
}

func generateECDSAKeyPair(t *testing.T) (*ecdsa.PrivateKey, string) {
	t.Helper()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	der, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	require.NoError(t, err)

	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: der,
	})

	return priv, string(pubPEM)
}

func pushLegacyCosignSig(
	t *testing.T,
	imgRef name.Reference,
	digest crv1.Hash,
	priv *ecdsa.PrivateKey,
	mutateSig func([]byte),
) {
	t.Helper()

	payloadObj := map[string]any{
		"critical": map[string]any{
			"identity": map[string]any{
				"docker-reference": imgRef.Context().Name(),
			},
			"image": map[string]any{
				"docker-manifest-digest": digest.String(),
			},
			"type": "cosign container image signature",
		},
	}

	payloadBytes, err := json.Marshal(payloadObj)
	require.NoError(t, err)

	sum := sha256.Sum256(payloadBytes)
	sigBytes, err := ecdsa.SignASN1(rand.Reader, priv, sum[:])
	require.NoError(t, err)

	if mutateSig != nil {
		mutateSig(sigBytes)
	}

	sigLayer := static.NewLayer(payloadBytes, types.MediaType("application/vnd.dev.cosign.simplesigning.v1+json"))
	sigImg, err := mutate.Append(empty.Image, mutate.Addendum{
		Layer: sigLayer,
		Annotations: map[string]string{
			cosignSigAnnotation: base64.StdEncoding.EncodeToString(sigBytes),
		},
	})
	require.NoError(t, err)

	sigTagName := strings.Replace(digest.String(), ":", "-", 1) + ".sig"
	sigRef, err := name.NewTag(imgRef.Context().Name() + ":" + sigTagName)
	require.NoError(t, err)

	require.NoError(t, remote.Write(sigRef, sigImg))
}

func TestVerifyImage_UnsignedSkipsWithWarningStatus(t *testing.T) {
	srv := httptest.NewServer(registry.New())
	t.Cleanup(srv.Close)

	ref, digest := pushTestImage(t, srv.URL, "test/unsigned:v1.0.0")

	res, err := VerifyImage(context.Background(), VerifyOptions{
		Image: ref.String(),
	})
	require.NoError(t, err)
	assert.Equal(t, VerifyStatusSkippedMissing, res.Status)
	assert.Equal(t, digest.String(), res.Digest)
	assert.Contains(t, res.ResolvedRef, "@"+digest.String())
}

func TestVerifyImage_StaticKeyValidAndInvalid(t *testing.T) {
	srv := httptest.NewServer(registry.New())
	t.Cleanup(srv.Close)

	priv, pubPEM := generateECDSAKeyPair(t)
	_, wrongPubPEM := generateECDSAKeyPair(t)

	t.Run("valid signature passes and pins digest", func(t *testing.T) {
		ref, digest := pushTestImage(t, srv.URL, "test/signed-ok:v1.0.0")
		pushLegacyCosignSig(t, ref, digest, priv, nil)

		res, err := VerifyImage(context.Background(), VerifyOptions{
			Image:     ref.String(),
			PublicKey: pubPEM,
		})
		require.NoError(t, err)
		assert.Equal(t, VerifyStatusVerified, res.Status)
		assert.Equal(t, digest.String(), res.Digest)
		assert.Equal(t, ref.Context().Digest(digest.String()).String(), res.ResolvedRef)
	})

	t.Run("wrong public key fails", func(t *testing.T) {
		ref, digest := pushTestImage(t, srv.URL, "test/signed-wrong-key:v1.0.0")
		pushLegacyCosignSig(t, ref, digest, priv, nil)

		_, err := VerifyImage(context.Background(), VerifyOptions{
			Image:     ref.String(),
			PublicKey: wrongPubPEM,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "verification failed")
	})

	t.Run("corrupted signature fails", func(t *testing.T) {
		ref, digest := pushTestImage(t, srv.URL, "test/signed-corrupt:v1.0.0")
		pushLegacyCosignSig(t, ref, digest, priv, func(b []byte) {
			b[0] ^= 0xff
		})

		_, err := VerifyImage(context.Background(), VerifyOptions{
			Image:     ref.String(),
			PublicKey: pubPEM,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "verification failed")
	})
}

func TestVerifyImage_EmptyRef(t *testing.T) {
	_, err := VerifyImage(context.Background(), VerifyOptions{Image: ""})
	require.Error(t, err)
}