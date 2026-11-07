//go:build darwin

package cpa_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mag1cFall/AIStudio2API/internal/cpa"
)

func TestDynamicLibraryDLOpenABI(t *testing.T) {
	dylibPath, err := filepath.Abs(filepath.Join("..", "..", "dist", "plugins", "darwin", "arm64", "aistudio2api.dylib"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dylibPath); err != nil {
		t.Skipf("dylib not built at %s: %v", dylibPath, err)
	}

	tmpDir := t.TempDir()
	cfgYAML := []byte("data_dir: " + tmpDir + "\nauto_start: false\nsync_auth_files: false\n")
	payloadBytes, _ := json.Marshal(map[string]any{
		"schema_version": 10,
		"config_yaml":    cfgYAML,
	})
	got, err := cpa.CallSharedLibraryABI(dylibPath, "plugin.register", string(payloadBytes))
	_, _ = cpa.CallSharedLibraryABI(dylibPath, "plugin.shutdown", `{}`)
	if err != nil {
		t.Fatalf("CallSharedLibraryABI failed: %v", err)
	}
	if !strings.Contains(got, `"ok":true`) || !strings.Contains(got, `"Name":"aistudio2api"`) {
		t.Fatalf("unexpected dlopen response from plugin.register: %s", got)
	}
	t.Logf("dlopen C ABI plugin.register response: %s", got)
}
