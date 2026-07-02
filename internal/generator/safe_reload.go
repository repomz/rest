package generator

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type safeReload struct {
	root   string
	stdin  io.Reader
	stdout io.Writer
}

func newSafeReload(root string, stdin io.Reader, stdout io.Writer) safeReload {
	if stdin == nil {
		stdin = os.Stdin
	}
	if stdout == nil {
		stdout = os.Stdout
	}
	return safeReload{root: root, stdin: stdin, stdout: stdout}
}

func ResolveSafeReload(root string, files []string, stdin io.Reader, stdout io.Writer) (map[string][]byte, error) {
	return newSafeReload(root, stdin, stdout).resolve(files)
}

func RestoreSafeReload(root string, files map[string][]byte) error {
	return newSafeReload(root, nil, nil).restore(files)
}

func SaveSafeReload(root string, files []string) error {
	return newSafeReload(root, nil, nil).save(files)
}

func (s safeReload) resolve(files []string) (map[string][]byte, error) {
	files = uniqueSorted(files)
	preserved := map[string][]byte{}
	reader := bufio.NewReader(s.stdin)
	all := ""
	for _, rel := range files {
		snapshot, err := os.ReadFile(s.snapshotPath(rel))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		current, err := os.ReadFile(filepath.Join(s.root, filepath.FromSlash(rel)))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if string(snapshot) == string(current) {
			continue
		}
		if all == "" {
			fmt.Fprintln(s.stdout, unifiedDiff(rel, string(snapshot), string(current)))
			fmt.Fprintf(s.stdout, "User changes detected in %s\n", rel)
			fmt.Fprint(s.stdout, "Choose: a) keep, b) overwrite, c) keep all, d) overwrite all: ")
			answer, err := reader.ReadString('\n')
			if err != nil && err != io.EOF {
				return nil, err
			}
			answer = strings.ToLower(strings.TrimSpace(answer))
			switch answer {
			case "a":
				preserved[rel] = current
			case "b":
			case "c":
				all = "keep"
				preserved[rel] = current
			case "d":
				all = "overwrite"
			default:
				return nil, fmt.Errorf("unsupported safe_reload answer %q", answer)
			}
			continue
		}
		if all == "keep" {
			preserved[rel] = current
		}
	}
	return preserved, nil
}

func (s safeReload) restore(files map[string][]byte) error {
	for rel, content := range files {
		path := filepath.Join(s.root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, content, fileMode(path)); err != nil {
			return err
		}
	}
	return nil
}

func (s safeReload) save(files []string) error {
	for _, rel := range uniqueSorted(files) {
		source := filepath.Join(s.root, filepath.FromSlash(rel))
		content, err := os.ReadFile(source)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		target := s.snapshotPath(rel)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, content, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func (s safeReload) snapshotPath(rel string) string {
	sum := sha256.Sum256([]byte(filepath.ToSlash(rel)))
	return filepath.Join(s.root, ".rest", "safe_reload", hex.EncodeToString(sum[:])+".snapshot")
}

func uniqueSorted(values []string) []string {
	seen := map[string]bool{}
	var result []string
	for _, value := range values {
		value = filepath.ToSlash(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func unifiedDiff(path, oldText, newText string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "diff --git a/%s b/%s\n", path, path)
	fmt.Fprintf(&b, "--- a/%s\n+++ b/%s\n", path, path)
	oldLines := strings.SplitAfter(oldText, "\n")
	newLines := strings.SplitAfter(newText, "\n")
	for _, line := range oldLines {
		if line != "" {
			b.WriteString("-" + line)
		}
	}
	for _, line := range newLines {
		if line != "" {
			b.WriteString("+" + line)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func fileMode(path string) os.FileMode {
	if strings.HasSuffix(path, ".sh") {
		return 0o755
	}
	if filepath.Base(path) == ".env" {
		return 0o600
	}
	return 0o644
}
