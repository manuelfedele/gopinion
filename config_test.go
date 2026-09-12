package gopinion

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfigAppliesSecureDefaults(t *testing.T) {
	path := writeTestConfig(t, "version: 1\n")

	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if cfg.Authentication.Mode != policyRequired {
		t.Fatalf("authentication mode = %q, want %q", cfg.Authentication.Mode, policyRequired)
	}
	if cfg.Pagination.Mode != policyRequired {
		t.Fatalf("pagination mode = %q, want %q", cfg.Pagination.Mode, policyRequired)
	}
	if cfg.Pagination.DefaultSize != 25 || cfg.Pagination.MaximumSize != 100 {
		t.Fatalf("pagination defaults = %d/%d, want 25/100", cfg.Pagination.DefaultSize, cfg.Pagination.MaximumSize)
	}
}

func TestLoadConfigRejectsUnknownFields(t *testing.T) {
	path := writeTestConfig(t, "version: 1\nauthentcation:\n  mode: disabled\n")

	_, err := loadConfig(path)
	if err == nil || !strings.Contains(err.Error(), "field authentcation not found") {
		t.Fatalf("loadConfig() error = %v, want unknown field error", err)
	}
}

func TestLoadConfigRequiresExplicitVersion(t *testing.T) {
	for _, content := range []string{"{}\n", "version: null\n"} {
		_, err := loadConfig(writeTestConfig(t, content))
		if err == nil || !strings.Contains(err.Error(), "version is required") {
			t.Fatalf("loadConfig(%q) error = %v, want required version error", content, err)
		}
	}
}

func TestLoadConfigRejectsMultipleDocuments(t *testing.T) {
	path := writeTestConfig(t, "version: 1\n---\nversion: 1\n")

	_, err := loadConfig(path)
	if err == nil || !strings.Contains(err.Error(), "multiple YAML documents") {
		t.Fatalf("loadConfig() error = %v, want multiple document error", err)
	}
}

func TestLoadConfigValidatesPolicyAndPagination(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "unknown authentication mode",
			content: "version: 1\nauthentication:\n  mode: optional\n",
			want:    "authentication.mode must be",
		},
		{
			name:    "default larger than maximum",
			content: "version: 1\npagination:\n  default_size: 101\n  maximum_size: 100\n",
			want:    "maximum_size must be greater",
		},
		{
			name:    "invalid duration",
			content: "version: 1\nserver:\n  shutdown_timeout: never\n",
			want:    "server.shutdown_timeout is invalid",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := loadConfig(writeTestConfig(t, test.content))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("loadConfig() error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func writeTestConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gopinion.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write configuration: %v", err)
	}
	return path
}
