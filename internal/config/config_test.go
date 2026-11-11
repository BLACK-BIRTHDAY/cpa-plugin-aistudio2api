package config

import (
	"testing"
	"time"
)

// TestValidateProxyCredentials 验证代理 URL 接受账号密码并拒绝空用户名
func TestValidateProxyCredentials(t *testing.T) {
	for _, value := range []string{"http://user:secret@127.0.0.1:8080", "socks5://user:secret@127.0.0.1:1080"} {
		if err := ValidateProxy(value); err != nil {
			t.Errorf("ValidateProxy(%q) = %v", value, err)
		}
	}
	if err := ValidateProxy("http://:secret@127.0.0.1:8080"); err == nil {
		t.Error("空用户名被接受")
	}
}

func TestParseFirstEventHedgeSupportsDurationStrings(t *testing.T) {
	for _, tc := range []struct {
		input string
		auto  bool
		after time.Duration
	}{
		{"auto", true, 0},
		{"off", false, 0},
		{"60", false, 60 * time.Second},
		{"60s", false, 60 * time.Second},
		{"6s", false, 6 * time.Second},
	} {
		auto, after, err := ParseFirstEventHedge(tc.input)
		if err != nil || auto != tc.auto || after != tc.after {
			t.Fatalf("ParseFirstEventHedge(%q) = (%v, %v, %v), want (%v, %v, nil)", tc.input, auto, after, err, tc.auto, tc.after)
		}
	}
}

func TestBuildStallTimeoutConfig(t *testing.T) {
	cfg := Default()
	if cfg.BuildStallTimeout != DefaultBuildStallTimeout || DefaultBuildStallTimeout != 2*time.Minute {
		t.Fatalf("Default().BuildStallTimeout = %v, want %v", cfg.BuildStallTimeout, DefaultBuildStallTimeout)
	}

	if err := configSetters["BUILD_STALL_TIMEOUT"](&cfg, "3m"); err != nil || cfg.BuildStallTimeout != 3*time.Minute {
		t.Fatalf("configSetters[BUILD_STALL_TIMEOUT](3m) err=%v, value=%v", err, cfg.BuildStallTimeout)
	}

	if err := configSetters["BUILD_STALL_TIMEOUT"](&cfg, "120s"); err != nil || cfg.BuildStallTimeout != 120*time.Second {
		t.Fatalf("configSetters[BUILD_STALL_TIMEOUT](120s) err=%v, value=%v", err, cfg.BuildStallTimeout)
	}

	for _, invalid := range []string{"0s", "-1s", "invalid", ""} {
		if err := configSetters["BUILD_STALL_TIMEOUT"](&cfg, invalid); err == nil {
			t.Fatalf("configSetters[BUILD_STALL_TIMEOUT](%q) expected error, got nil", invalid)
		}
	}

	if env := cfg.envValues()["BUILD_STALL_TIMEOUT"]; env != cfg.BuildStallTimeout.String() {
		t.Fatalf("envValues()[BUILD_STALL_TIMEOUT] = %q, want %q", env, cfg.BuildStallTimeout.String())
	}

	validCfg := Default()
	if err := validCfg.Validate(); err != nil {
		t.Fatalf("Default().Validate() = %v", err)
	}

	invalidCfg := Default()
	invalidCfg.BuildStallTimeout = 0
	if err := invalidCfg.Validate(); err == nil {
		t.Fatal("Validate() with BuildStallTimeout=0 expected error, got nil")
	}

	invalidCfg.BuildStallTimeout = -5 * time.Second
	if err := invalidCfg.Validate(); err == nil {
		t.Fatal("Validate() with negative BuildStallTimeout expected error, got nil")
	}
}
