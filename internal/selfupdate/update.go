package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const (
	DefaultRepoOwner       = "repomz"
	DefaultRepoName        = "rest"
	defaultOIDCIssuer      = "https://token.actions.githubusercontent.com"
	checksumsAssetName     = "checksums.txt"
	checksumsSignatureName = "checksums.txt.sig"
	checksumsCertName      = "checksums.txt.pem"
)

var runCosignVerifyBlob = defaultRunCosignVerifyBlob

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
	PreviousVersion   string
	Version           string
	ReleaseNotes      string
	ReleaseURL        string
	AssetName         string
	ExecutablePath    string
	Available         bool
	Updated           bool
	Checksum          string
	SignatureVerified bool
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
	checksums, err := selectChecksumsAsset(rel)
	if err != nil {
		return Result{}, err
	}
	signature, err := selectNamedAsset(rel, checksumsSignatureName)
	if err != nil {
		return Result{}, err
	}
	certificate, err := selectNamedAsset(rel, checksumsCertName)
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
	checksumsPath := filepath.Join(tmpDir, checksums.Name)
	if err := download(ctx, opts.Client, checksums.URL, checksumsPath); err != nil {
		return Result{}, err
	}
	signaturePath := filepath.Join(tmpDir, signature.Name)
	if err := download(ctx, opts.Client, signature.URL, signaturePath); err != nil {
		return Result{}, err
	}
	certificatePath := filepath.Join(tmpDir, certificate.Name)
	if err := download(ctx, opts.Client, certificate.URL, certificatePath); err != nil {
		return Result{}, err
	}
	if err := verifyChecksumsSignature(ctx, checksumsPath, signaturePath, certificatePath, opts); err != nil {
		return Result{}, err
	}
	checksum, err := verifyArchiveChecksum(archivePath, checksumsPath)
	if err != nil {
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
		PreviousVersion:   opts.CurrentVersion,
		Version:           rel.TagName,
		ReleaseNotes:      strings.TrimSpace(rel.Body),
		ReleaseURL:        rel.HTMLURL,
		AssetName:         selected.Name,
		ExecutablePath:    opts.ExecutablePath,
		Available:         true,
		Updated:           true,
		Checksum:          checksum,
		SignatureVerified: true,
	}, nil
}

// Check returns whether a newer release is available without downloading or installing assets.
func Check(ctx context.Context, opts Options) (Result, error) {
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
	available := current != latest
	if opts.Force {
		available = true
	}
	return Result{
		PreviousVersion: opts.CurrentVersion,
		Version:         rel.TagName,
		ReleaseNotes:    strings.TrimSpace(rel.Body),
		ReleaseURL:      rel.HTMLURL,
		ExecutablePath:  opts.ExecutablePath,
		Available:       available,
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

func selectChecksumsAsset(rel release) (asset, error) {
	return selectNamedAsset(rel, checksumsAssetName)
}

func selectNamedAsset(rel release, name string) (asset, error) {
	for _, candidate := range rel.Assets {
		if candidate.Name == name && candidate.URL != "" {
			return candidate, nil
		}
	}
	return asset{}, fmt.Errorf("release %s has no %s asset", rel.TagName, name)
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

func verifyChecksumsSignature(ctx context.Context, checksumsPath, signaturePath, certificatePath string, opts Options) error {
	identityRegexp := releaseCertificateIdentityRegexp(opts.RepoOwner, opts.RepoName)
	if err := runCosignVerifyBlob(ctx, checksumsPath, signaturePath, certificatePath, identityRegexp, defaultOIDCIssuer); err != nil {
		return fmt.Errorf("cosign verification failed for checksums.txt: %w", err)
	}
	return nil
}

func defaultRunCosignVerifyBlob(ctx context.Context, checksumsPath, signaturePath, certificatePath, identityRegexp, oidcIssuer string) error {
	cosignPath, err := exec.LookPath("cosign")
	if err != nil {
		return fmt.Errorf("cosign is required for strict release verification; install cosign and retry")
	}
	cmd := exec.CommandContext(ctx, cosignPath,
		"verify-blob",
		"--certificate", certificatePath,
		"--signature", signaturePath,
		"--certificate-identity-regexp", identityRegexp,
		"--certificate-oidc-issuer", oidcIssuer,
		checksumsPath,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("%s", message)
	}
	return nil
}

func releaseCertificateIdentityRegexp(owner, repo string) string {
	identity := fmt.Sprintf("https://github.com/%s/%s/.github/workflows/release.yml@refs/tags/v", owner, repo)
	return "^" + regexp.QuoteMeta(identity) + ".*$"
}

func verifyArchiveChecksum(archivePath, checksumsPath string) (string, error) {
	expected, err := expectedChecksum(archivePath, checksumsPath)
	if err != nil {
		return "", err
	}
	actual, err := fileSHA256(archivePath)
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(actual, expected) {
		return "", fmt.Errorf("checksum mismatch for %s: expected %s, got %s", filepath.Base(archivePath), expected, actual)
	}
	return actual, nil
}

func expectedChecksum(archivePath, checksumsPath string) (string, error) {
	content, err := os.ReadFile(checksumsPath)
	if err != nil {
		return "", err
	}
	archiveName := filepath.Base(archivePath)
	for _, line := range strings.Split(string(content), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := strings.TrimPrefix(fields[len(fields)-1], "*")
		if filepath.Base(name) == archiveName {
			checksum := strings.ToLower(fields[0])
			if len(checksum) != sha256.Size*2 {
				return "", fmt.Errorf("invalid SHA-256 checksum for %s", archiveName)
			}
			if _, err := hex.DecodeString(checksum); err != nil {
				return "", fmt.Errorf("invalid SHA-256 checksum for %s", archiveName)
			}
			return checksum, nil
		}
	}
	return "", fmt.Errorf("checksums.txt has no SHA-256 entry for %s", archiveName)
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
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
