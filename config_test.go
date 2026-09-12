package gopinion

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfigAppliesSecureDefaults(t *testing.T) {
	path := writeTestConfig(t, "version: 3\n")

	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if cfg.Authentication.Mode != policyRequired {
		t.Fatalf("authentication mode = %q, want %q", cfg.Authentication.Mode, policyRequired)
	}
	if cfg.Authorization.Mode != policyRequired {
		t.Fatalf("authorization mode = %q, want %q", cfg.Authorization.Mode, policyRequired)
	}
	if cfg.Pagination.Mode != policyRequired {
		t.Fatalf("pagination mode = %q, want %q", cfg.Pagination.Mode, policyRequired)
	}
	if cfg.Pagination.DefaultLimit != 25 || cfg.Pagination.MaximumLimit != 100 {
		t.Fatalf("pagination defaults = %d/%d, want 25/100", cfg.Pagination.DefaultLimit, cfg.Pagination.MaximumLimit)
	}
	if cfg.Pagination.MaximumOffset != 10000 {
		t.Fatalf("maximum offset = %d, want 10000", cfg.Pagination.MaximumOffset)
	}
}

func TestNewLoadsCanonicalConfigFile(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile(configFileName, []byte("version: 3\n"), 0o600); err != nil {
		t.Fatalf("write canonical configuration: %v", err)
	}

	if _, err := New(WithAuthenticator(validTestAuthenticator()), WithAuthorizer(allowAllAuthorizer())); err != nil {
		t.Fatalf("New() error = %v", err)
	}
}

func TestNewFailsWithoutCanonicalConfigFile(t *testing.T) {
	t.Chdir(t.TempDir())

	_, err := New()
	if err == nil || !strings.Contains(err.Error(), configFileName) {
		t.Fatalf("New() error = %v, want missing canonical configuration error", err)
	}
}

func TestLoadConfigRejectsUnknownFields(t *testing.T) {
	tests := []struct {
		name    string
		content string
		field   string
	}{
		{name: "misspelled field", content: "version: 3\nauthentcation:\n  mode: disabled\n", field: "authentcation"},
		{name: "legacy pagination field", content: "version: 3\npagination:\n  default_size: 25\n", field: "default_size"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := loadConfig(writeTestConfig(t, test.content))
			if err == nil || !strings.Contains(err.Error(), "field "+test.field+" not found") {
				t.Fatalf("loadConfig() error = %v, want unknown field error", err)
			}
		})
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

func TestLoadConfigRejectsLegacyVersion(t *testing.T) {
	_, err := loadConfig(writeTestConfig(t, "version: 2\n"))
	if err == nil || !strings.Contains(err.Error(), "version must be 3") {
		t.Fatalf("loadConfig() error = %v, want version 3 error", err)
	}
}

func TestLoadConfigRejectsMultipleDocuments(t *testing.T) {
	path := writeTestConfig(t, "version: 3\n---\nversion: 3\n")

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
			content: "version: 3\nauthentication:\n  mode: optional\n",
			want:    "authentication.mode must be",
		},
		{
			name:    "unknown authorization mode",
			content: "version: 3\nauthorization:\n  mode: optional\n",
			want:    "authorization.mode must be",
		},
		{
			name:    "authorization without authentication",
			content: "version: 3\nauthentication:\n  mode: disabled\n",
			want:    "authorization.mode cannot be required",
		},
		{
			name:    "default larger than maximum",
			content: "version: 3\npagination:\n  default_limit: 101\n  maximum_limit: 100\n",
			want:    "maximum_limit must be greater",
		},
		{
			name:    "negative maximum offset",
			content: "version: 3\npagination:\n  maximum_offset: -1\n",
			want:    "maximum_offset must not be negative",
		},
		{
			name:    "offset and limit overflow",
			content: fmt.Sprintf("version: 3\npagination:\n  maximum_offset: %d\n", int(^uint(0)>>1)),
			want:    "maximum_offset plus pagination.maximum_limit is too large",
		},
		{
			name:    "invalid duration",
			content: "version: 3\nserver:\n  shutdown_timeout: never\n",
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
