package selfupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSelectAsset(t *testing.T) {
	rel := release{
		TagName: "v0.2.0",
		Assets: []asset{
			{Name: "rest_v0.2.0_linux_amd64.tar.gz", URL: "https://example.test/linux"},
			{Name: "rest_v0.2.0_darwin_arm64.tar.gz", URL: "https://example.test/darwin"},
		},
	}
	got, err := selectAsset(rel, "darwin", "arm64")
	if err != nil {
		t.Fatal(err)
	}
	if got.URL != "https://example.test/darwin" {
		t.Fatalf("asset URL = %q", got.URL)
	}
}

func TestSelectAssetRejectsMissingPlatform(t *testing.T) {
	_, err := selectAsset(release{TagName: "v0.2.0"}, "linux", "amd64")
	if err == nil {
		t.Fatal("expected missing asset error")
	}
}

func TestSelectChecksumsAsset(t *testing.T) {
	got, err := selectChecksumsAsset(release{
		TagName: "v0.2.0",
		Assets: []asset{
			{Name: "rest_v0.2.0_linux_amd64.tar.gz", URL: "https://example.test/archive"},
			{Name: "checksums.txt", URL: "https://example.test/checksums.txt"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.URL != "https://example.test/checksums.txt" {
		t.Fatalf("checksums URL = %q", got.URL)
	}
}

func TestSelectChecksumsAssetRejectsMissingChecksums(t *testing.T) {
	_, err := selectChecksumsAsset(release{TagName: "v0.2.0"})
	if err == nil {
		t.Fatal("expected missing checksums error")
	}
}

func TestVerifyArchiveChecksum(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "rest_v0.2.0_linux_amd64.tar.gz")
	content := []byte("archive")
	if err := os.WriteFile(archivePath, content, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(content)
	checksumsPath := filepath.Join(dir, "checksums.txt")
	if err := os.WriteFile(checksumsPath, []byte(fmt.Sprintf("%x  %s\n", sum, filepath.Base(archivePath))), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := verifyArchiveChecksum(archivePath, checksumsPath)
	if err != nil {
		t.Fatal(err)
	}
	if got != fmt.Sprintf("%x", sum) {
		t.Fatalf("checksum = %q", got)
	}
}

func TestVerifyArchiveChecksumRejectsMismatch(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "rest_v0.2.0_linux_amd64.tar.gz")
	if err := os.WriteFile(archivePath, []byte("archive"), 0o644); err != nil {
		t.Fatal(err)
	}
	checksumsPath := filepath.Join(dir, "checksums.txt")
	if err := os.WriteFile(checksumsPath, []byte(strings.Repeat("0", 64)+"  "+filepath.Base(archivePath)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := verifyArchiveChecksum(archivePath, checksumsPath); err == nil {
		t.Fatal("expected checksum mismatch error")
	}
}

func TestExtractTarGz(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "rest_v0.2.0_"+runtime.GOOS+"_"+runtime.GOARCH+".tar.gz")
	if err := writeTarGz(archivePath, "rest", "new binary"); err != nil {
		t.Fatal(err)
	}
	extracted, err := extractBinary(archivePath, dir)
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(extracted)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "new binary" {
		t.Fatalf("extracted content = %q", content)
	}
}

func TestInstallBinaryReplacesExecutable(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "rest")
	source := filepath.Join(dir, "new-rest")
	if err := os.WriteFile(target, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("new"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := installBinary(source, target); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "new" {
		t.Fatalf("installed content = %q", content)
	}
	if _, err := os.Stat(target + ".old"); !os.IsNotExist(err) {
		t.Fatalf("expected backup cleanup, got %v", err)
	}
}

func TestChangelogReturnsGitHubReleaseNotes(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if !strings.HasSuffix(req.URL.Path, "/releases/tags/v0.2.0") {
				t.Fatalf("unexpected release path: %s", req.URL.Path)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(`{
					"tag_name": "v0.2.0",
					"body": "  Features:\n\n- Add changelog output.  ",
					"html_url": "https://github.com/repomz/rest/releases/tag/v0.2.0"
				}`)),
				Header: make(http.Header),
			}, nil
		}),
	}

	result, err := Changelog(context.Background(), Options{
		TargetVersion: "v0.2.0",
		Client:        client,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Version != "v0.2.0" {
		t.Fatalf("version = %q", result.Version)
	}
	if result.ReleaseNotes != "Features:\n\n- Add changelog output." {
		t.Fatalf("release notes = %q", result.ReleaseNotes)
	}
	if result.ReleaseURL != "https://github.com/repomz/rest/releases/tag/v0.2.0" {
		t.Fatalf("release URL = %q", result.ReleaseURL)
	}
}

func TestCheckReportsAvailableVersion(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if !strings.HasSuffix(req.URL.Path, "/releases/latest") {
				t.Fatalf("unexpected release path: %s", req.URL.Path)
			}
			return jsonResponse(`{
				"tag_name": "v0.2.0",
				"body": "Release notes",
				"html_url": "https://github.com/repomz/rest/releases/tag/v0.2.0"
			}`), nil
		}),
	}
	result, err := Check(context.Background(), Options{
		CurrentVersion: "v0.1.0",
		Client:         client,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Available || result.Version != "v0.2.0" {
		t.Fatalf("unexpected check result: %+v", result)
	}
}

func TestUpdateVerifiesChecksumBeforeInstall(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("tar.gz update fixture is for Unix-like platforms")
	}
	archiveName := "rest_v0.2.0_" + runtime.GOOS + "_" + runtime.GOARCH + ".tar.gz"
	archiveBytes, err := tarGzBytes("rest", "new binary")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(archiveBytes)
	checksums := fmt.Sprintf("%x  %s\n", sum, archiveName)
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch req.URL.String() {
			case "https://api.github.com/repos/repomz/rest/releases/latest":
				return jsonResponse(fmt.Sprintf(`{
					"tag_name": "v0.2.0",
					"body": "Release notes",
					"html_url": "https://github.com/repomz/rest/releases/tag/v0.2.0",
					"assets": [
						{"name": %q, "browser_download_url": "https://downloads.test/%s"},
						{"name": "checksums.txt", "browser_download_url": "https://downloads.test/checksums.txt"}
					]
				}`, archiveName, archiveName)), nil
			case "https://downloads.test/" + archiveName:
				return bytesResponse(archiveBytes), nil
			case "https://downloads.test/checksums.txt":
				return bytesResponse([]byte(checksums)), nil
			default:
				t.Fatalf("unexpected URL: %s", req.URL.String())
				return nil, nil
			}
		}),
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "rest")
	if err := os.WriteFile(target, []byte("old binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	result, err := Update(context.Background(), Options{
		CurrentVersion: "v0.1.0",
		Client:         client,
		ExecutablePath: target,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Updated || result.Checksum != fmt.Sprintf("%x", sum) {
		t.Fatalf("unexpected update result: %+v", result)
	}
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "new binary" {
		t.Fatalf("installed content = %q", content)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func bytesResponse(body []byte) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(body)),
		Header:     make(http.Header),
	}
}

func writeTarGz(path, name, content string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	gz := gzip.NewWriter(file)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()
	header := &tar.Header{
		Name: name,
		Mode: 0o755,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	_, err = tw.Write([]byte(content))
	return err
}

func tarGzBytes(name, content string) ([]byte, error) {
	var buffer bytes.Buffer
	gz := gzip.NewWriter(&buffer)
	tw := tar.NewWriter(gz)
	header := &tar.Header{
		Name: name,
		Mode: 0o755,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(header); err != nil {
		return nil, err
	}
	if _, err := tw.Write([]byte(content)); err != nil {
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
