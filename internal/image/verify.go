package image

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	crv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	protocommon "github.com/sigstore/protobuf-specs/gen/pb-go/common/v1"
	protorekor "github.com/sigstore/protobuf-specs/gen/pb-go/rekor/v1"
	"github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/verify"
	"github.com/sigstore/sigstore/pkg/signature"
)

const (
	// DefaultGitHubOIDCIssuer is the OIDC issuer URL for GitHub Actions.
	DefaultGitHubOIDCIssuer = "https://token.actions.githubusercontent.com"
	// DefaultNetdrillIdentityRegexp matches GitHub Actions workflows in xenOs76/netdrill.
	DefaultNetdrillIdentityRegexp = `^https://github\.com/(?i:xenos76)/netdrill/`

	cosignSigAnnotation   = "dev.cosignproject.cosign/signature"
	cosignCertAnnotation  = "dev.sigstore.cosign/certificate"
	cosignChainAnnotation = "dev.sigstore.cosign/chain"
	cosignBundleAnnot     = "dev.sigstore.cosign/bundle"
	sigstoreBundlePrefix  = "application/vnd.dev.sigstore.bundle"
)

// VerifyStatus describes the outcome of an image signature check.
type VerifyStatus int

const (
	// VerifyStatusVerified indicates at least one valid Cosign signature matched the policy.
	VerifyStatusVerified VerifyStatus = iota
	// VerifyStatusSkippedMissing indicates the image had no Cosign signatures in the registry.
	VerifyStatusSkippedMissing
)

// VerifyOptions configures Cosign signature verification for a container image.
type VerifyOptions struct {
	// Image is the container image reference (tag or digest).
	Image string
	// PublicKey is an optional file path or inline PEM public key for static-key verification.
	PublicKey string
	// Identity is the expected certificate SAN or regexp for keyless OIDC verification.
	Identity string
	// OIDCIssuer is the expected OIDC issuer URL or regexp for keyless OIDC verification.
	OIDCIssuer string
}

// VerifyResult contains the outcome and resolved digest reference from VerifyImage.
type VerifyResult struct {
	Status      VerifyStatus
	ResolvedRef string
	Digest      string
	Reason      string
}

type candidateSignature struct {
	bundle  *bundle.Bundle
	payload []byte
}

type legacyRekorPayload struct {
	Body           string `json:"body"`
	IntegratedTime int64  `json:"integratedTime"`
	LogIndex       int64  `json:"logIndex"`
	LogID          string `json:"logID"` //nolint:tagliatelle // Rekor wire format
}

type legacyRekorBundle struct {
	SignedEntryTimestamp string             `json:"SignedEntryTimestamp"` //nolint:tagliatelle // wire format
	Payload              legacyRekorPayload `json:"Payload"`              //nolint:tagliatelle // wire format
}

type simpleSigningImage struct {
	DockerManifestDigest string `json:"docker-manifest-digest"` //nolint:tagliatelle // SimpleSigning spec
}

type simpleSigningCritical struct {
	Image simpleSigningImage `json:"image"`
}

type simpleSigningPayload struct {
	Critical simpleSigningCritical `json:"critical"`
}

var fetchPublicTrustedRoot = func() (root.TrustedMaterial, error) {
	return root.FetchTrustedRoot()
}

// VerifyImage resolves image to an immutable digest and verifies its Cosign signatures.
// If no signatures exist in the registry, it returns VerifyStatusSkippedMissing with nil error.
// If signatures exist but none pass verification, it returns an error.
func VerifyImage(ctx context.Context, opts VerifyOptions) (*VerifyResult, error) {
	if strings.TrimSpace(opts.Image) == "" {
		return nil, errors.New("image reference is empty")
	}

	ref, err := name.ParseReference(opts.Image)
	if err != nil {
		return nil, fmt.Errorf("parse image reference %q: %w", opts.Image, err)
	}

	remoteOpts := []remote.Option{
		remote.WithContext(ctx),
		remote.WithAuthFromKeychain(authn.DefaultKeychain),
	}

	desc, err := remote.Head(ref, remoteOpts...)
	if err != nil {
		return nil, fmt.Errorf("resolve image digest for %q: %w", opts.Image, err)
	}

	digestStr := desc.Digest.String()
	digestRef := ref.Context().Digest(digestStr)

	candidates, err := fetchCandidateSignatures(digestRef, digestStr, remoteOpts)
	if err != nil {
		return nil, err
	}

	if len(candidates) == 0 {
		return &VerifyResult{
			Status:      VerifyStatusSkippedMissing,
			ResolvedRef: digestRef.String(),
			Digest:      digestStr,
			Reason:      "no cosign signatures found",
		}, nil
	}

	verifier, policyOpts, err := buildVerifierAndPolicy(ref, opts)
	if err != nil {
		return nil, err
	}

	digestBytes, err := hex.DecodeString(desc.Digest.Hex)
	if err != nil {
		return nil, fmt.Errorf("decode digest hex: %w", err)
	}

	var lastErr error

	for _, cand := range candidates {
		artOpt := verify.WithArtifactDigest("sha256", digestBytes)
		if len(cand.payload) > 0 {
			artOpt = verify.WithArtifact(bytes.NewReader(cand.payload))
		}

		policy := verify.NewPolicy(artOpt, policyOpts...)

		_, err := verifier.Verify(cand.bundle, policy)
		if err == nil {
			return &VerifyResult{
				Status:      VerifyStatusVerified,
				ResolvedRef: digestRef.String(),
				Digest:      digestStr,
			}, nil
		}

		lastErr = err
	}

	return nil, fmt.Errorf("cosign signature verification failed: %w", lastErr)
}

func fetchCandidateSignatures(
	digestRef name.Digest,
	digestStr string,
	remoteOpts []remote.Option,
) ([]candidateSignature, error) {
	var candidates []candidateSignature

	refBundles, err := fetchReferrerBundles(digestRef, remoteOpts)
	if err != nil && !isNotFoundOrUnsupported(err) {
		return nil, fmt.Errorf("fetch OCI referrers: %w", err)
	}

	candidates = append(candidates, refBundles...)

	legacyCands, err := fetchLegacySigTag(digestRef, digestStr, remoteOpts)
	if err != nil && !isNotFoundOrUnsupported(err) {
		return nil, fmt.Errorf("fetch cosign .sig tag: %w", err)
	}

	candidates = append(candidates, legacyCands...)

	return candidates, nil
}

func readLayerBytes(layer crv1.Layer) ([]byte, error) {
	rc, err := layer.Uncompressed()
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(rc); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func fetchReferrerImageBundles(
	imgRef name.Digest,
	remoteOpts []remote.Option,
) ([]candidateSignature, error) {
	img, err := remote.Image(imgRef, remoteOpts...)
	if err != nil {
		return nil, err
	}

	layers, err := img.Layers()
	if err != nil {
		return nil, err
	}

	var out []candidateSignature

	for _, layer := range layers {
		mt, err := layer.MediaType()
		if err != nil || !strings.HasPrefix(string(mt), sigstoreBundlePrefix) {
			continue
		}

		raw, err := readLayerBytes(layer)
		if err != nil {
			return nil, err
		}

		var b bundle.Bundle
		if err := b.UnmarshalJSON(raw); err != nil {
			return nil, fmt.Errorf("unmarshal sigstore bundle: %w", err)
		}

		out = append(out, candidateSignature{bundle: &b})
	}

	return out, nil
}

func fetchReferrerBundles(digestRef name.Digest, remoteOpts []remote.Option) ([]candidateSignature, error) {
	idx, err := remote.Referrers(digestRef, remoteOpts...)
	if err != nil {
		return nil, err
	}

	idxManifest, err := idx.IndexManifest()
	if err != nil {
		return nil, err
	}

	var out []candidateSignature

	for _, desc := range idxManifest.Manifests {
		if !strings.HasPrefix(desc.ArtifactType, sigstoreBundlePrefix) &&
			!strings.HasPrefix(string(desc.MediaType), sigstoreBundlePrefix) {
			continue
		}

		cands, err := fetchReferrerImageBundles(digestRef.Context().Digest(desc.Digest.String()), remoteOpts)
		if err != nil {
			return nil, err
		}

		out = append(out, cands...)
	}

	return out, nil
}

func legacyLayerToCandidate(
	layer crv1.Layer,
	ann map[string]string,
	digestStr string,
) (candidateSignature, bool, error) {
	sigB64, ok := ann[cosignSigAnnotation]
	if !ok || sigB64 == "" {
		return candidateSignature{}, false, nil
	}

	payloadBytes, err := readLayerBytes(layer)
	if err != nil {
		return candidateSignature{}, false, err
	}

	if !payloadMatchesDigest(payloadBytes, digestStr) {
		return candidateSignature{}, false, nil
	}

	b, err := buildBundleFromLegacyAnnotations(payloadBytes, sigB64, ann)
	if err != nil {
		return candidateSignature{}, false, err
	}

	return candidateSignature{
		bundle:  b,
		payload: payloadBytes,
	}, true, nil
}

func fetchLegacySigTag(
	digestRef name.Digest,
	digestStr string,
	remoteOpts []remote.Option,
) ([]candidateSignature, error) {
	sigTagName := strings.Replace(digestStr, ":", "-", 1) + ".sig"

	sigRef, err := name.NewTag(digestRef.Context().Name() + ":" + sigTagName)
	if err != nil {
		return nil, err
	}

	sigImg, err := remote.Image(sigRef, remoteOpts...)
	if err != nil {
		return nil, err
	}

	manifest, err := sigImg.Manifest()
	if err != nil {
		return nil, err
	}

	layers, err := sigImg.Layers()
	if err != nil {
		return nil, err
	}

	var out []candidateSignature

	for i, layer := range layers {
		if i >= len(manifest.Layers) {
			break
		}

		cand, ok, err := legacyLayerToCandidate(layer, manifest.Layers[i].Annotations, digestStr)
		if err != nil {
			return nil, err
		}

		if ok {
			out = append(out, cand)
		}
	}

	return out, nil
}

func payloadMatchesDigest(payload []byte, expectedDigest string) bool {
	var parsed simpleSigningPayload
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return false
	}

	return parsed.Critical.Image.DockerManifestDigest == expectedDigest
}

func buildBundleFromLegacyAnnotations(
	payload []byte,
	sigB64 string,
	ann map[string]string,
) (*bundle.Bundle, error) {
	sigBytes, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		return nil, fmt.Errorf("decode cosign signature: %w", err)
	}

	sum := sha256.Sum256(payload)

	pb := &protobundle.Bundle{
		MediaType:            "application/vnd.dev.sigstore.bundle+json;version=0.1",
		VerificationMaterial: &protobundle.VerificationMaterial{},
		Content: &protobundle.Bundle_MessageSignature{
			MessageSignature: &protocommon.MessageSignature{
				MessageDigest: &protocommon.HashOutput{
					Algorithm: protocommon.HashAlgorithm_SHA2_256,
					Digest:    sum[:],
				},
				Signature: sigBytes,
			},
		},
	}

	if certPEM := ann[cosignCertAnnotation]; certPEM != "" {
		certs, err := parsePEMCertChain(certPEM + "\n" + ann[cosignChainAnnotation])
		if err != nil {
			return nil, err
		}

		pb.VerificationMaterial.Content = &protobundle.VerificationMaterial_X509CertificateChain{
			X509CertificateChain: &protocommon.X509CertificateChain{
				Certificates: certs,
			},
		}
	} else {
		pb.VerificationMaterial.Content = &protobundle.VerificationMaterial_PublicKey{
			PublicKey: &protocommon.PublicKeyIdentifier{},
		}
	}

	if rekorJSON := ann[cosignBundleAnnot]; rekorJSON != "" {
		tlogEntry, err := parseLegacyRekorBundle(rekorJSON)
		if err != nil {
			return nil, err
		}

		pb.VerificationMaterial.TlogEntries = []*protorekor.TransparencyLogEntry{tlogEntry}
	}

	return bundle.NewBundle(pb)
}

func parsePEMCertChain(pemData string) ([]*protocommon.X509Certificate, error) {
	var certs []*protocommon.X509Certificate

	rest := []byte(pemData)
	for len(rest) > 0 {
		var block *pem.Block

		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}

		if block.Type != "CERTIFICATE" {
			continue
		}

		certs = append(certs, &protocommon.X509Certificate{RawBytes: block.Bytes})
	}

	if len(certs) == 0 {
		return nil, errors.New("no certificates found in cosign certificate annotation")
	}

	return certs, nil
}

func parseLegacyRekorBundle(raw string) (*protorekor.TransparencyLogEntry, error) {
	var rb legacyRekorBundle
	if err := json.Unmarshal([]byte(raw), &rb); err != nil {
		return nil, fmt.Errorf("parse cosign rekor bundle: %w", err)
	}

	logIDBytes, err := hex.DecodeString(rb.Payload.LogID)
	if err != nil {
		return nil, fmt.Errorf("decode rekor logID: %w", err)
	}

	setBytes, err := base64.StdEncoding.DecodeString(rb.SignedEntryTimestamp)
	if err != nil {
		return nil, fmt.Errorf("decode rekor SET: %w", err)
	}

	bodyBytes, err := base64.StdEncoding.DecodeString(rb.Payload.Body)
	if err != nil {
		return nil, fmt.Errorf("decode rekor body: %w", err)
	}

	return &protorekor.TransparencyLogEntry{
		LogIndex: rb.Payload.LogIndex,
		LogId: &protocommon.LogId{
			KeyId: logIDBytes,
		},
		KindVersion: &protorekor.KindVersion{
			Kind:    "hashedrekord",
			Version: "0.0.1",
		},
		IntegratedTime: rb.Payload.IntegratedTime,
		InclusionPromise: &protorekor.InclusionPromise{
			SignedEntryTimestamp: setBytes,
		},
		CanonicalizedBody: bodyBytes,
	}, nil
}

func buildVerifierAndPolicy(
	ref name.Reference,
	opts VerifyOptions,
) (*verify.Verifier, []verify.PolicyOption, error) {
	if opts.PublicKey != "" {
		pubKey, err := loadPublicKey(opts.PublicKey)
		if err != nil {
			return nil, nil, err
		}

		tm := root.NewTrustedPublicKeyMaterial(func(string) (root.TimeConstrainedVerifier, error) {
			return loadPublicKeyVerifier(pubKey)
		})

		sev, err := verify.NewVerifier(tm, verify.WithNoObserverTimestamps())
		if err != nil {
			return nil, nil, err
		}

		return sev, []verify.PolicyOption{verify.WithKey()}, nil
	}

	trustedRoot, err := fetchPublicTrustedRoot()
	if err != nil {
		return nil, nil, fmt.Errorf("fetch sigstore trusted root: %w", err)
	}

	issuer := opts.OIDCIssuer
	if issuer == "" {
		issuer = DefaultGitHubOIDCIssuer
	}

	identity := opts.Identity
	if identity == "" {
		if strings.EqualFold(ref.Context().Name(), DefaultRepo) {
			identity = DefaultNetdrillIdentityRegexp
		} else {
			identity = ".*"
		}
	}

	certID, err := verify.NewShortCertificateIdentity("", issuer, "", identity)
	if err != nil {
		return nil, nil, fmt.Errorf("build certificate identity policy: %w", err)
	}

	sev, err := verify.NewVerifier(
		trustedRoot,
		verify.WithSignedCertificateTimestamps(1),
		verify.WithObserverTimestamps(1),
		verify.WithTransparencyLog(1),
	)
	if err != nil {
		return nil, nil, err
	}

	return sev, []verify.PolicyOption{verify.WithCertificateIdentity(certID)}, nil
}

func loadPublicKey(keyInput string) (crypto.PublicKey, error) {
	var pemBytes []byte

	if strings.Contains(keyInput, "-----BEGIN ") {
		pemBytes = []byte(keyInput)
	} else {
		var err error

		pemBytes, err = os.ReadFile(keyInput)
		if err != nil {
			return nil, fmt.Errorf("read public key %q: %w", keyInput, err)
		}
	}

	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("failed to decode public key PEM")
	}

	pubKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}

	return pubKey, nil
}

type nonExpiringVerifier struct {
	signature.Verifier
}

func (*nonExpiringVerifier) ValidAtTime(_ time.Time) bool {
	return true
}

func loadPublicKeyVerifier(pk crypto.PublicKey) (root.TimeConstrainedVerifier, error) {
	var hashAlg crypto.Hash

	switch k := pk.(type) {
	case *ecdsa.PublicKey:
		switch k.Curve {
		case elliptic.P384():
			hashAlg = crypto.SHA384
		case elliptic.P521():
			hashAlg = crypto.SHA512
		default:
			hashAlg = crypto.SHA256
		}
	case ed25519.PublicKey:
		hashAlg = crypto.Hash(0)
	case *rsa.PublicKey:
		hashAlg = crypto.SHA256
	default:
		return nil, fmt.Errorf("unsupported public key type %T", pk)
	}

	v, err := signature.LoadVerifier(pk, hashAlg)
	if err != nil {
		return nil, err
	}

	return &nonExpiringVerifier{v}, nil
}

func isNotFoundOrUnsupported(err error) bool {
	var te *transport.Error
	if errors.As(err, &te) {
		return te.StatusCode == http.StatusNotFound || te.StatusCode == http.StatusMethodNotAllowed
	}

	return false
}
