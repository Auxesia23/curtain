package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindProjectConfigWalkUp(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "services", "api")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	if path, cfg, err := findProjectConfig(nested); err != nil || path != "" || cfg != nil {
		t.Fatalf("tanpa file: got path=%q cfg=%v err=%v", path, cfg, err)
	}

	original := &projectConfig{ProjectID: 7, ProjectName: "my-app", Environment: "development"}
	if err := saveProjectConfig(filepath.Join(root, ProjectConfigName), original); err != nil {
		t.Fatalf("saveProjectConfig: %v", err)
	}

	path, cfg, err := findProjectConfig(nested)
	if err != nil {
		t.Fatalf("findProjectConfig: %v", err)
	}
	if path != filepath.Join(root, ProjectConfigName) {
		t.Fatalf("path: got %q", path)
	}
	if cfg.ProjectID != 7 || cfg.Environment != "development" {
		t.Fatalf("cfg tidak sesuai: %+v", cfg)
	}
}

func TestLoadProjectConfigValidation(t *testing.T) {
	dir := t.TempDir()

	broken := filepath.Join(dir, ProjectConfigName)
	if err := os.WriteFile(broken, []byte("{bukan json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadProjectConfig(broken); err == nil {
		t.Fatal("JSON rusak seharusnya error")
	}

	incomplete := filepath.Join(dir, "incomplete.json")
	if err := os.WriteFile(incomplete, []byte(`{"version":1,"project_id":0,"environment":""}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadProjectConfig(incomplete); err == nil {
		t.Fatal("config tanpa project_id/environment seharusnya error")
	}

	valid := filepath.Join(dir, "valid.json")
	if err := os.WriteFile(valid, []byte(`{"version":1,"project_id":3,"environment":"production"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadProjectConfig(valid)
	if err != nil {
		t.Fatalf("config valid: %v", err)
	}
	if cfg.ProjectID != 3 || cfg.Environment != "production" {
		t.Fatalf("cfg tidak sesuai: %+v", cfg)
	}
}

func TestNormalizeServer(t *testing.T) {
	cases := map[string]string{
		"https://vault.example.com/": "https://vault.example.com",
		"http://localhost:8080":      "http://localhost:8080",
		"localhost:8080/":            "http://localhost:8080",
		"  https://x.dev  ":          "https://x.dev",
	}
	for input, want := range cases {
		if got := normalizeServer(input); got != want {
			t.Errorf("normalizeServer(%q) = %q, want %q", input, got, want)
		}
	}
}
