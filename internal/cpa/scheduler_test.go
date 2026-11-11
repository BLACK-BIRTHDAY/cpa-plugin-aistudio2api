package cpa

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/Mag1cFall/AIStudio2API/internal/aistudio"
	"github.com/Mag1cFall/AIStudio2API/internal/pluginabi"
	"github.com/Mag1cFall/AIStudio2API/internal/pluginapi"
)

func TestSchedulerDefersToHostAffinityAndRejectsShortCooldowns(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome)

	dataDir := filepath.ToSlash(filepath.Join(tmpHome, "data"))
	p := NewPlugin(nil)
	defer func() {
		_, _ = p.HandleMethod(pluginabi.MethodPluginShutdown, nil)
	}()

	cfgYAML := []byte("data_dir: \"" + dataDir + "\"\nwaa_backend: spidermonkey\nauto_start: false\nsync_auth_files: false\n")
	regReq, _ := json.Marshal(map[string]any{
		"config_yaml":    cfgYAML,
		"schema_version": pluginabi.SchemaVersion,
	})
	if _, err := p.HandleMethod(pluginabi.MethodPluginRegister, regReq); err != nil {
		t.Fatalf("plugin.register failed: %v", err)
	}

	rt, err := p.Runtime()
	if err != nil {
		t.Fatalf("Runtime failed: %v", err)
	}

	model := aistudio.Model{
		ID:           "gemini-4-argon",
		Methods:      []string{"generateContent"},
		Capabilities: map[string]bool{"chat_model": true},
	}
	for _, acc := range []*aistudio.Account{
		{ID: "a@example.com", Config: aistudio.AccountConfig{Enabled: true, Label: "A"}, State: aistudio.AccountReady},
		{ID: "b@example.com", Config: aistudio.AccountConfig{Enabled: true, Label: "B"}, State: aistudio.AccountReady},
	} {
		if err := rt.Pool().Add(acc); err != nil {
			t.Fatal(err)
		}
		if err := rt.Pool().SetCatalog(acc.ID, aistudio.BenefitTierFree, []aistudio.Model{model}); err != nil {
			t.Fatal(err)
		}
	}

	// Cool A for 30s while B remains eligible.
	if err := rt.Pool().MarkCooldownIfGeneration(
		"a@example.com",
		aistudio.ChannelCooldownScope(aistudio.ChannelPlayground, "gemini-4-argon"),
		rt.Pool().ModelAccessGeneration("a@example.com"),
		time.Now(),
		time.Now().Add(30*time.Second),
		"rate_limited",
	); err != nil {
		t.Fatal(err)
	}

	candidates := []pluginapi.SchedulerAuthCandidate{
		{ID: "aistudio2api-a@example.com.json", Provider: ProviderID, Attributes: map[string]string{"account_id": "a@example.com"}},
		{ID: "aistudio2api-b@example.com.json", Provider: ProviderID, Attributes: map[string]string{"account_id": "b@example.com"}},
	}

	// 1. Host-dispatched pick (SelectionIDMetadataKey present) defers to host selector when at least one candidate is eligible.
	hostPickRaw, _ := json.Marshal(pluginapi.SchedulerPickRequest{
		Model:      "gemini-4-argon",
		Candidates: candidates,
		Options: pluginapi.SchedulerOptions{
			Metadata: map[string]any{
				pluginapi.SelectionIDMetadataKey: "sel-123",
			},
		},
	})
	hostPickRes, err := p.HandleMethod(pluginabi.MethodSchedulerPick, hostPickRaw)
	if err != nil {
		t.Fatalf("scheduler.pick host-dispatched failed: %v", err)
	}
	if hostPickRes.(pluginapi.SchedulerPickResponse).Handled {
		t.Fatalf("expected Handled=false for host-dispatched request with eligible candidates, got %+v", hostPickRes)
	}

	// 2. Non-host-dispatched pick selects the eligible non-cooling account directly.
	directPickRaw, _ := json.Marshal(pluginapi.SchedulerPickRequest{
		Model:      "gemini-4-argon",
		Candidates: candidates,
	})
	directPickRes, err := p.HandleMethod(pluginabi.MethodSchedulerPick, directPickRaw)
	if err != nil {
		t.Fatalf("scheduler.pick direct failed: %v", err)
	}
	directResp := directPickRes.(pluginapi.SchedulerPickResponse)
	if !directResp.Handled || directResp.AuthID != "aistudio2api-b@example.com.json" {
		t.Fatalf("expected direct pick of account B, got %+v", directResp)
	}

	// 3. When all candidates are cooling even for <60s (30s), scheduler rejects immediately with rate_limit_exceeded.
	if err := rt.Pool().MarkCooldownIfGeneration(
		"b@example.com",
		aistudio.ChannelCooldownScope(aistudio.ChannelPlayground, "gemini-4-argon"),
		rt.Pool().ModelAccessGeneration("b@example.com"),
		time.Now(),
		time.Now().Add(30*time.Second),
		"rate_limited",
	); err != nil {
		t.Fatal(err)
	}
	allCoolingRes, err := p.HandleMethod(pluginabi.MethodSchedulerPick, hostPickRaw)
	if err != nil {
		t.Fatalf("scheduler.pick all-cooling failed: %v", err)
	}
	allCoolingResp := allCoolingRes.(pluginapi.SchedulerPickResponse)
	if !allCoolingResp.Handled || !allCoolingResp.Reject || allCoolingResp.RejectCode != "rate_limit_exceeded" {
		t.Fatalf("expected immediate rate_limit_exceeded rejection for 30s cooldown, got %+v", allCoolingResp)
	}
}
