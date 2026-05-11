package upgrade

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallCLIDownloadsVerifiesAndInstallsTarAsset(t *testing.T) {
	archive := tarGzBinary(t, "dygo", []byte("#!/bin/sh\n"))
	sum := sha256.Sum256(archive)
	checksums := []byte(fmt.Sprintf("%x  dygo_v1.2.3_darwin_arm64.tar.gz\n", sum))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/dygo_v1.2.3_darwin_arm64.tar.gz":
			_, _ = w.Write(archive)
		case "/checksums.txt":
			_, _ = w.Write(checksums)
		default:
			t.Fatalf("unexpected download path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	installDir := t.TempDir()
	err := InstallCLI(context.Background(), InstallOptions{
		Release: Release{
			TagName: "v1.2.3",
			Assets: []Asset{
				{Name: "dygo_v1.2.3_darwin_arm64.tar.gz", DownloadURL: server.URL + "/dygo_v1.2.3_darwin_arm64.tar.gz"},
				{Name: "checksums.txt", DownloadURL: server.URL + "/checksums.txt"},
			},
		},
		InstallDir: installDir,
		GOOS:       "darwin",
		GOARCH:     "arm64",
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("InstallCLI() error = %v, want nil", err)
	}
	info, err := os.Stat(filepath.Join(installDir, "dygo"))
	if err != nil {
		t.Fatalf("Stat(installed dygo) error = %v, want installed binary", err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("installed dygo mode = %v, want executable", info.Mode().Perm())
	}
}

func TestInstallCLIRejectsChecksumMismatch(t *testing.T) {
	archive := tarGzBinary(t, "dygo", []byte("#!/bin/sh\n"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/dygo_v1.2.3_linux_amd64.tar.gz":
			_, _ = w.Write(archive)
		case "/checksums.txt":
			_, _ = w.Write([]byte("0000  dygo_v1.2.3_linux_amd64.tar.gz\n"))
		default:
			t.Fatalf("unexpected download path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	err := InstallCLI(context.Background(), InstallOptions{
		Release: Release{
			TagName: "v1.2.3",
			Assets: []Asset{
				{Name: "dygo_v1.2.3_linux_amd64.tar.gz", DownloadURL: server.URL + "/dygo_v1.2.3_linux_amd64.tar.gz"},
				{Name: "checksums.txt", DownloadURL: server.URL + "/checksums.txt"},
			},
		},
		InstallDir: t.TempDir(),
		GOOS:       "linux",
		GOARCH:     "amd64",
		HTTPClient: server.Client(),
	})
	if err == nil {
		t.Fatal("InstallCLI() error = nil, want checksum error")
	}
}

func tarGzBinary(t *testing.T, name string, data []byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	if err := tarWriter.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(data))}); err != nil {
		t.Fatalf("WriteHeader() error = %v", err)
	}
	if _, err := tarWriter.Write(data); err != nil {
		t.Fatalf("tar Write() error = %v", err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("tar Close() error = %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("gzip Close() error = %v", err)
	}
	return buffer.Bytes()
}
