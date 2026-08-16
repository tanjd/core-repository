package handlers

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// coverMaxBytes is the maximum size accepted for a downloaded cover image (10 MiB).
const coverMaxBytes = 10 << 20

// allowedCoverHosts is the set of trusted external image hosts.
// Only URLs whose host matches one of these suffixes are fetched server-side,
// preventing SSRF attacks from user-supplied cover_url values.
var allowedCoverHosts = []string{
	"covers.openlibrary.org",
	"books.google.com",
	"books.googleusercontent.com",
	"cover.books.readmill.com",
}

// isCoverURLAllowed reports whether the given URL is safe to fetch.
func isCoverURLAllowed(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	for _, allowed := range allowedCoverHosts {
		if host == allowed || strings.HasSuffix(host, "."+allowed) {
			return true
		}
	}
	return false
}

// downloadCover fetches an external image URL and saves it to destDir.
// Returns the proxy-accessible path (/api/covers/<filename>) on success.
// Returns ("", nil) if externalURL is empty or already a local path (skip).
// On failure, returns ("", err); callers should log and keep the original URL.
func downloadCover(externalURL, destDir string) (string, error) {
	if externalURL == "" {
		return "", nil
	}
	// Already a local/proxy path — nothing to download.
	if strings.HasPrefix(externalURL, "/") {
		return externalURL, nil
	}

	if !isCoverURLAllowed(externalURL) {
		return "", fmt.Errorf("cover URL host not in allowlist: %s", externalURL)
	}

	// Deterministic filename: first 8 bytes of SHA-256(url) as hex.
	sum := sha256.Sum256([]byte(externalURL))
	baseName := fmt.Sprintf("%x", sum[:8])

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(externalURL) //nolint:noctx,gosec
	if err != nil {
		return "", err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("cover fetch returned %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "image/") {
		return "", fmt.Errorf("unexpected content-type %q", ct)
	}

	ext := ".jpg"
	if strings.Contains(ct, "png") {
		ext = ".png"
	} else if strings.Contains(ct, "webp") {
		ext = ".webp"
	}

	filename := baseName + ext
	destPath := filepath.Join(destDir, filename)

	// Already cached — skip re-download.
	if _, err := os.Stat(destPath); err == nil {
		return "/api/covers/" + filename, nil
	}

	f, err := os.Create(destPath) //nolint:gosec
	if err != nil {
		return "", err
	}
	defer f.Close() //nolint:errcheck

	if _, err := io.Copy(f, io.LimitReader(resp.Body, coverMaxBytes)); err != nil {
		_ = os.Remove(destPath) // clean up partial file
		return "", err
	}

	log.Info().Str("filename", filename).Msg("cover cached")
	return "/api/covers/" + filename, nil
}
