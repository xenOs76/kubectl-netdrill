package cmd

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/xenos76/kubectl-netdrill/internal/image"
)

const verifyTimeout = 30 * time.Second

// verifyImageFn is overridable in tests.
var verifyImageFn = image.VerifyImage

// verifyAndPinImage verifies the container image Cosign signature when --verify is enabled.
// Missing signatures emit a warning and return the original image; invalid signatures fail.
func verifyAndPinImage(ctx context.Context, img string, errOut io.Writer) (string, error) {
	if !VerifyImage {
		return img, nil
	}

	verifyCtx, cancel := context.WithTimeout(ctx, verifyTimeout)
	defer cancel()

	res, err := verifyImageFn(verifyCtx, image.VerifyOptions{
		Image:      img,
		PublicKey:  CosignKey,
		Identity:   CosignIdentity,
		OIDCIssuer: CosignOIDCIssuer,
	})
	if err != nil {
		return "", fmt.Errorf("cosign verification failed for %s (use --verify=false to skip): %w", img, err)
	}

	if res.Status == image.VerifyStatusSkippedMissing {
		if errOut != nil {
			_, _ = fmt.Fprintf(errOut, "Warning: %s has no cosign signatures; skipping verification\n\n", img)
		}

		return img, nil
	}

	if errOut != nil {
		_, _ = fmt.Fprintf(errOut, "Verified cosign signature for %s (%s)\n\n", img, res.Digest)
	}

	return res.ResolvedRef, nil
}
