package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// configFileName is stored in the user's non-secret config directory.
	configFileName = "config.json"
	// ProjectConfigName links a local directory to a server project and environment.
	// It contains no secrets and is safe to commit.
	ProjectConfigName = ".curtain.json"

	configVersion = 1
	defaultServer = "http://localhost:8080"
)

// fileConfig is the user's global config (~/.config/curtain/config.json).
// It contains only non-sensitive data; tokens and keys stay in the OS keyring.
type fileConfig struct {
	Version int    `json:"version"`
	Server  string `json:"server"`
	Email   string `json:"email"`
}

// projectConfig is the content of .curtain.json.
type projectConfig struct {
	Version     int    `json:"version"`
	ProjectID   uint   `json:"project_id"`
	ProjectName string `json:"project_name,omitempty"`
	Environment string `json:"environment"`
}

// normalizeServer trims whitespace and trailing slashes, and adds http:// when
// the user enters a hostname without a scheme.
func normalizeServer(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.TrimRight(s, "/")
	if s != "" && !strings.Contains(s, "://") {
		s = "http://" + s
	}
	return s
}

// userConfigDir returns this user's Curtain config directory.
func userConfigDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine config directory: %w", err)
	}
	return filepath.Join(dir, "curtain"), nil
}

// loadFileConfig reads config.json and returns (nil, nil) when it does not exist.
func loadFileConfig() (*fileConfig, error) {
	dir, err := userConfigDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, configFileName)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}
	var cfg fileConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("corrupt config file (%s): %w", path, err)
	}
	return &cfg, nil
}

// saveFileConfig writes config.json with mode 0600.
func saveFileConfig(cfg *fileConfig) error {
	dir, err := userConfigDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	cfg.Version = configVersion
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	path := filepath.Join(dir, configFileName)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	// Preserve mode 0600 even if the file already existed.
	if err := os.Chmod(path, 0o600); err != nil {
		return err
	}
	return nil
}

// findProjectConfig searches for .curtain.json from startDir up through parent
// directories, similar to Git's config lookup. It returns ("", nil) if missing.
func findProjectConfig(startDir string) (string, *projectConfig, error) {
	dir := startDir
	for {
		path := filepath.Join(dir, ProjectConfigName)
		if _, err := os.Stat(path); err == nil {
			cfg, err := loadProjectConfig(path)
			if err != nil {
				return "", nil, err
			}
			return path, cfg, nil
		} else if !os.IsNotExist(err) {
			return "", nil, err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil, nil
		}
		dir = parent
	}
}

// loadProjectConfig reads one .curtain.json file.
func loadProjectConfig(path string) (*projectConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg projectConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("file %s is corrupt: %w", path, err)
	}
	if cfg.ProjectID == 0 || strings.TrimSpace(cfg.Environment) == "" {
		return nil, fmt.Errorf("file %s is incomplete (project_id and environment required): re-run `curtain init`", path)
	}
	return &cfg, nil
}

// saveProjectConfig writes .curtain.json.
func saveProjectConfig(path string, cfg *projectConfig) error {
	cfg.Version = configVersion
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	return nil
}
