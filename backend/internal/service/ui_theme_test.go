package service

import (
	"archive/zip"
	"bytes"
	"testing"
)

func TestCleanThemeZipPathKeepsRootSubdirectories(t *testing.T) {
	got, err := cleanThemeZipPath("pages/welcome.md")
	if err != nil {
		t.Fatalf("cleanThemeZipPath returned error: %v", err)
	}
	if got != "pages/welcome.md" {
		t.Fatalf("cleanThemeZipPath = %q, want pages/welcome.md", got)
	}
}

func TestCleanThemeZipPathStripsGitHubArchiveRoot(t *testing.T) {
	got, err := cleanThemeZipPath("theme-main/pages/welcome.md")
	if err != nil {
		t.Fatalf("cleanThemeZipPath returned error: %v", err)
	}
	if got != "pages/welcome.md" {
		t.Fatalf("cleanThemeZipPath = %q, want pages/welcome.md", got)
	}
}

func TestSanitizeThemeCSSRejectsUnsafeConstructs(t *testing.T) {
	if _, err := sanitizeThemeCSS([]byte(`body{background:url("https://example.com/a.png")}`)); err == nil {
		t.Fatalf("expected external url CSS to be rejected")
	}
	if _, err := sanitizeThemeCSS([]byte(`body{color:red}`)); err != nil {
		t.Fatalf("expected safe CSS to pass: %v", err)
	}
}

func TestReadManifestFromZip(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("theme-main/lightbridge-ui.json")
	if err != nil {
		t.Fatalf("create manifest: %v", err)
	}
	_, _ = w.Write([]byte(`{"id":"modern-light","name":"Modern Light","version":"1.0.0","entry_css":"style.css"}`))
	w, err = zw.Create("theme-main/style.css")
	if err != nil {
		t.Fatalf("create css: %v", err)
	}
	_, _ = w.Write([]byte(`:root{--x:red}`))
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	manifest, err := readManifestFromZip(buf.Bytes())
	if err != nil {
		t.Fatalf("readManifestFromZip returned error: %v", err)
	}
	if manifest.ID != "modern-light" {
		t.Fatalf("manifest ID = %q", manifest.ID)
	}
}
