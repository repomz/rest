package generator

import (
	"strings"
	"testing"
)

func TestWorkflowTemplatesRenderGitHubExpressions(t *testing.T) {
	data := renderData{}

	ci, err := renderTemplateForTest(t, "ci.yaml", ciWorkflowTemplate, data)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"name: CI", "go-version: ${{ matrix.go-version }}", "go test -race ./...", "govulncheck@latest ./...", "go vet ./...", "go build ./cmd"} {
		if !strings.Contains(string(ci), expected) {
			t.Fatalf("CI workflow missing %q:\n%s", expected, ci)
		}
	}

	cd, err := renderTemplateForTest(t, "cd.yaml", cdWorkflowTemplate, data)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"name: CD", "${{ github.actor }}", "${{ secrets.GITHUB_TOKEN }}", "ghcr.io/${{ github.repository }}:${{ github.ref_name }}"} {
		if !strings.Contains(string(cd), expected) {
			t.Fatalf("CD workflow missing %q:\n%s", expected, cd)
		}
	}
}
