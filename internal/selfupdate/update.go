package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	DefaultRepoOwner = "repomz"
	DefaultRepoName  = "rest"
)

type Options struct {
	CurrentVersion string
	TargetVersion  string
	Force          bool
	ExecutablePath string
	RepoOwner      string
	RepoName       string
	Client         *http.Client
	Stdout         io.Writer
}

type Result struct {
	PreviousVersion string
	Version         string
	ReleaseNotes    string
	ReleaseURL      string
	AssetName       string
	ExecutablePath  string
	Updated         bool
}

type release struct {
	TagName string  `json:"tag_name"`
	Body    string  `json:"body"`
	HTMLURL string  `json:"html_url"`
	Assets  []asset `json:"assets"`
}

type asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

func Update(ctx context.Context, opts Options) (Result, error) {
	opts = withDefaults(opts)
	rel, err := fetchRelease(ctx, opts)
	if err != nil {
		return Result{}, err
	}
	current := normalizeVersion(opts.CurrentVersion)
	latest := normalizeVersion(rel.TagName)
	if latest == "" {
		return Result{}, fmt.Errorf("release has empty tag name")
	}
	if current == latest && !opts.Force {
		return Result{
			PreviousVersion: opts.CurrentVersion,
			Version:         rel.TagName,
			ReleaseNotes:    strings.TrimSpace(rel.Body),
			ReleaseURL:      rel.HTMLURL,
			ExecutablePath:  opts.ExecutablePath,
		}, nil
	}
	selected, err := selectAsset(rel, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return Result{}, err
	}
	tmpDir, err := os.MkdirTemp("", "rest-update-*")
	if err != nil {
		return Result{}, err
	}
	defer os.RemoveAll(tmpDir)
	archivePath := filepath.Join(tmpDir, selected.Name)
	if err := download(ctx, opts.Client, selected.URL, archivePath); err != nil {
		return Result{}, err
	}
	binaryPath, err := extractBinary(archivePath, tmpDir)
	if err != nil {
		return Result{}, err
	}
	if err := installBinary(binaryPath, opts.ExecutablePath); err != nil {
		return Result{}, err
	}
	return Result{
		PreviousVersion: opts.CurrentVersion,
		Version:         rel.TagName,
		ReleaseNotes:    strings.TrimSpace(rel.Body),
		ReleaseURL:      rel.HTMLURL,
		AssetName:       selected.Name,
		ExecutablePath:  opts.ExecutablePath,
		Updated:         true,
	}, nil
}

// Changelog returns the release notes for a release without installing it.
func Changelog(ctx context.Context, opts Options) (Result, error) {
	opts = withDefaults(opts)
	rel, err := fetchRelease(ctx, opts)
	if err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(rel.TagName) == "" {
		return Result{}, fmt.Errorf("release has empty tag name")
	}
	return Result{
		Version:      rel.TagName,
		ReleaseNotes: strings.TrimSpace(rel.Body),
		ReleaseURL:   rel.HTMLURL,
	}, nil
}

func withDefaults(opts Options) Options {
	if opts.CurrentVersion == "" {
		opts.CurrentVersion = "dev"
	}
	if opts.RepoOwner == "" {
		opts.RepoOwner = DefaultRepoOwner
	}
	if opts.RepoName == "" {
		opts.RepoName = DefaultRepoName
	}
	if opts.Client == nil {
		opts.Client = &http.Client{Timeout: 60 * time.Second}
	}
	if opts.ExecutablePath == "" {
		if path, err := os.Executable(); err == nil {
			opts.ExecutablePath = path
		}
	}
	return opts
}

func fetchRelease(ctx context.Context, opts Options) (release, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", opts.RepoOwner, opts.RepoName)
	if opts.TargetVersion != "" {
		url = fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/tags/%s", opts.RepoOwner, opts.RepoName, opts.TargetVersion)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "rest-updater")
	resp, err := opts.Client.Do(req)
	if err != nil {
		return release{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return release{}, fmt.Errorf("github release request failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var rel release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return release{}, err
	}
	return rel, nil
}

func selectAsset(rel release, goos, goarch string) (asset, error) {
	ext := ".tar.gz"
	if goos == "windows" {
		ext = ".zip"
	}
	want := fmt.Sprintf("_%s_%s%s", goos, goarch, ext)
	for _, candidate := range rel.Assets {
		if strings.HasSuffix(candidate.Name, want) && candidate.URL != "" {
			return candidate, nil
		}
	}
	return asset{}, fmt.Errorf("release %s has no asset for %s/%s", rel.TagName, goos, goarch)
}

func download(ctx context.Context, client *http.Client, url, target string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "rest-updater")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s", resp.Status)
	}
	file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, resp.Body)
	return err
}

func extractBinary(archivePath, dir string) (string, error) {
	switch {
	case strings.HasSuffix(archivePath, ".tar.gz"):
		return extractTarGz(archivePath, dir)
	case strings.HasSuffix(archivePath, ".zip"):
		return extractZip(archivePath, dir)
	default:
		return "", fmt.Errorf("unsupported update archive: %s", archivePath)
	}
}

func extractTarGz(archivePath, dir string) (string, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return "", err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		name := filepath.Base(header.Name)
		if name != "rest" {
			continue
		}
		target := filepath.Join(dir, name)
		if err := writeExtractedFile(target, tr, 0o755); err != nil {
			return "", err
		}
		return target, nil
	}
	return "", fmt.Errorf("archive does not contain rest binary")
}

func extractZip(archivePath, dir string) (string, error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", err
	}
	defer reader.Close()
	for _, file := range reader.File {
		name := filepath.Base(file.Name)
		if name != "rest.exe" {
			continue
		}
		src, err := file.Open()
		if err != nil {
			return "", err
		}
		defer src.Close()
		target := filepath.Join(dir, name)
		if err := writeExtractedFile(target, src, 0o755); err != nil {
			return "", err
		}
		return target, nil
	}
	return "", fmt.Errorf("archive does not contain rest.exe binary")
}

func writeExtractedFile(target string, src io.Reader, mode os.FileMode) error {
	file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, src)
	return err
}

func installBinary(source, target string) error {
	if target == "" {
		return fmt.Errorf("cannot determine current executable path")
	}
	info, err := os.Stat(target)
	if err != nil {
		return err
	}
	tmp := target + ".new"
	backup := target + ".old"
	_ = os.Remove(tmp)
	_ = os.Remove(backup)
	content, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, content, info.Mode().Perm()|0o111); err != nil {
		return err
	}
	if err := os.Rename(target, backup); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Rename(backup, target)
		_ = os.Remove(tmp)
		return err
	}
	_ = os.Remove(backup)
	return nil
}

func normalizeVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" || version == "dev" {
		return version
	}
	return strings.TrimPrefix(version, "v")
}
