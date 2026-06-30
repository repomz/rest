package appgen

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerationStateDetectsUnchangedFingerprint(t *testing.T) {
	projectDir := t.TempDir()
	const fingerprint = "abc123"
	if err := saveGenerationFingerprint(projectDir, fingerprint); err != nil {
		t.Fatal(err)
	}
	unchanged, err := generationUnchanged(projectDir, fingerprint)
	if err != nil {
		t.Fatal(err)
	}
	if !unchanged {
		t.Fatal("saved fingerprint must be detected as unchanged")
	}
	if _, err := os.Stat(filepath.Join(projectDir, ".rest", "generation.json")); err != nil {
		t.Fatal(err)
	}
}
