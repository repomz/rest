package selfupdate

import (
	"archive/tar"
	"compress/gzip"
	"context"
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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
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
