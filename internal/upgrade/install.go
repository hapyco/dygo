package upgrade

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// InstallOptions configures CLI binary installation.
type InstallOptions struct {
	Release    Release
	InstallDir string
	GOOS       string
	GOARCH     string
	HTTPClient *http.Client
}

// InstallCLI downloads, verifies, and installs a dygo release binary.
func InstallCLI(ctx context.Context, options InstallOptions) error {
	goos := firstNonEmpty(options.GOOS, runtimeGOOS())
	goarch := firstNonEmpty(options.GOARCH, runtimeGOARCH())
	assetName, err := releaseAssetName(options.Release.TagName, goos, goarch)
	if err != nil {
		return err
	}
	asset, ok := options.Release.Asset(assetName)
	if !ok {
		return fmt.Errorf("release %s does not contain asset %s", options.Release.TagName, assetName)
	}
	checksums, ok := options.Release.Asset("checksums.txt")
	if !ok {
		return fmt.Errorf("release %s does not contain checksums.txt", options.Release.TagName)
	}

	client := options.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	tempDir, err := os.MkdirTemp("", "dygo-upgrade-*")
	if err != nil {
		return fmt.Errorf("create upgrade temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	archivePath := filepath.Join(tempDir, assetName)
	if err := downloadFile(ctx, client, asset.DownloadURL, archivePath); err != nil {
		return err
	}
	checksumPath := filepath.Join(tempDir, "checksums.txt")
	if err := downloadFile(ctx, client, checksums.DownloadURL, checksumPath); err != nil {
		return err
	}
	if err := verifyChecksum(archivePath, checksumPath, assetName); err != nil {
		return err
	}

	extracted := filepath.Join(tempDir, executableName(goos))
	if err := extractBinary(archivePath, extracted, goos); err != nil {
		return err
	}
	if err := os.MkdirAll(options.InstallDir, 0o755); err != nil {
		return fmt.Errorf("create install directory %s: %w", options.InstallDir, err)
	}
	target := filepath.Join(options.InstallDir, executableName(goos))
	tempTarget := target + ".tmp"
	if err := copyFile(extracted, tempTarget, 0o755); err != nil {
		return err
	}
	if err := os.Rename(tempTarget, target); err != nil {
		_ = os.Remove(tempTarget)
		return fmt.Errorf("install dygo binary to %s: %w", target, err)
	}
	return nil
}

func releaseAssetName(version string, goos string, goarch string) (string, error) {
	if goos == "" || goarch == "" {
		return "", fmt.Errorf("target platform is required")
	}
	switch goos {
	case "darwin", "linux":
		return fmt.Sprintf("dygo_%s_%s_%s.tar.gz", version, goos, goarch), nil
	case "windows":
		return fmt.Sprintf("dygo_%s_%s_%s.zip", version, goos, goarch), nil
	default:
		return "", fmt.Errorf("unsupported platform %s/%s", goos, goarch)
	}
}

func downloadFile(ctx context.Context, client *http.Client, url string, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create download request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download %s: returned %s", url, resp.Status)
	}
	out, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("create download file %s: %w", path, err)
	}
	defer out.Close()
	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("write download file %s: %w", path, err)
	}
	return nil
}

func verifyChecksum(archivePath string, checksumPath string, assetName string) error {
	data, err := os.ReadFile(checksumPath)
	if err != nil {
		return fmt.Errorf("read checksums: %w", err)
	}
	want := ""
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == assetName {
			want = fields[0]
			break
		}
	}
	if want == "" {
		return fmt.Errorf("checksums.txt is missing %s", assetName)
	}
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive for checksum: %w", err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("hash archive: %w", err)
	}
	got := hex.EncodeToString(hash.Sum(nil))
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("checksum mismatch for %s", assetName)
	}
	return nil
}

func extractBinary(archivePath string, destination string, goos string) error {
	if goos == "windows" {
		return extractZipBinary(archivePath, destination)
	}
	return extractTarBinary(archivePath, destination)
}

func extractTarBinary(archivePath string, destination string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive %s: %w", archivePath, err)
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("read gzip archive %s: %w", archivePath, err)
	}
	defer gzipReader.Close()
	reader := tar.NewReader(gzipReader)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar archive %s: %w", archivePath, err)
		}
		if header.Typeflag != tar.TypeReg || filepath.Base(header.Name) != "dygo" {
			continue
		}
		return writeReader(destination, reader, 0o755)
	}
	return fmt.Errorf("archive %s does not contain dygo binary", archivePath)
}

func extractZipBinary(archivePath string, destination string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("open zip archive %s: %w", archivePath, err)
	}
	defer reader.Close()
	for _, file := range reader.File {
		if file.FileInfo().IsDir() || filepath.Base(file.Name) != "dygo.exe" {
			continue
		}
		src, err := file.Open()
		if err != nil {
			return fmt.Errorf("open dygo.exe in archive: %w", err)
		}
		defer src.Close()
		return writeReader(destination, src, 0o755)
	}
	return fmt.Errorf("archive %s does not contain dygo.exe binary", archivePath)
}

func writeReader(destination string, reader io.Reader, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return fmt.Errorf("create destination directory: %w", err)
	}
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return fmt.Errorf("create %s: %w", destination, err)
	}
	defer out.Close()
	if _, err := io.Copy(out, reader); err != nil {
		return fmt.Errorf("write %s: %w", destination, err)
	}
	if err := out.Chmod(mode); err != nil {
		return fmt.Errorf("chmod %s: %w", destination, err)
	}
	return nil
}

func copyFile(source string, destination string, mode os.FileMode) error {
	src, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open %s: %w", source, err)
	}
	defer src.Close()
	return writeReader(destination, src, mode)
}
