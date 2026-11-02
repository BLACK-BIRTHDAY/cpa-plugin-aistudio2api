package cpa_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mag1cFall/AIStudio2API/internal/cpa"
	"github.com/Mag1cFall/AIStudio2API/internal/pluginabi"
	"github.com/Mag1cFall/AIStudio2API/internal/pluginapi"
	"github.com/Mag1cFall/AIStudio2API/internal/waa"
)

func TestPluginLifecycleAndCapabilities(t *testing.T) {
	tmpDir := t.TempDir()
	var hostCalls []string
	p := cpa.NewPlugin(func(method string, payload []byte) ([]byte, error) {
		hostCalls = append(hostCalls, method)
		return json.Marshal(pluginabi.Envelope{
			OK:     true,
			Result: json.RawMessage(`{}`),
		})
	})
	defer func() {
		_, _ = p.HandleMethod(pluginabi.MethodPluginShutdown, nil)
	}()

	cfgYAML := []byte("data_dir: " + tmpDir + "\nwaa_backend: spidermonkey\nauto_start: false\n")
	regReq, _ := json.Marshal(map[string]any{
		"config_yaml":    cfgYAML,
		"schema_version": pluginabi.SchemaVersion,
	})

	// 1. plugin.register
	regRes, err := p.HandleMethod(pluginabi.MethodPluginRegister, regReq)
	if err != nil {
		t.Fatalf("plugin.register failed: %v", err)
	}
	regJSON, _ := json.Marshal(regRes)
	if !strings.Contains(string(regJSON), `"Name":"aistudio2api"`) {
		t.Fatalf("unexpected registration metadata: %s", string(regJSON))
	}
	if !strings.Contains(string(regJSON), `"executor":true`) || !strings.Contains(string(regJSON), `"auth_provider":true`) {
		t.Fatalf("missing expected capabilities: %s", string(regJSON))
	}

	// 2. auth.identifier & auth.parse
	authIDRes, err := p.HandleMethod(pluginabi.MethodAuthIdentifier, nil)
	if err != nil {
		t.Fatalf("auth.identifier failed: %v", err)
	}
	authIDJSON, _ := json.Marshal(authIDRes)
	if !strings.Contains(string(authIDJSON), `"aistudio2api"`) {
		t.Fatalf("unexpected auth.identifier: %s", string(authIDJSON))
	}

	// Unrelated provider should return Handled: false
	unrelatedParseReq, _ := json.Marshal(pluginapi.AuthParseRequest{
		Provider: "openai",
		FileName: "openai-key.json",
		RawJSON:  []byte(`{"type":"openai"}`),
	})
	unrelatedRes, err := p.HandleMethod(pluginabi.MethodAuthParse, unrelatedParseReq)
	if err != nil {
		t.Fatalf("auth.parse unrelated failed: %v", err)
	}
	if unrelatedRes.(pluginapi.AuthParseResponse).Handled {
		t.Fatalf("expected Handled=false for unrelated provider")
	}

	// Matching aistudio2api auth file
	matchingParseReq, _ := json.Marshal(pluginapi.AuthParseRequest{
		Provider: "aistudio2api",
		FileName: "aistudio2api-user@example.com.json",
		RawJSON:  []byte(`{"type":"aistudio2api","account_id":"user@example.com","tier":"PRO"}`),
	})
	matchingRes, err := p.HandleMethod(pluginabi.MethodAuthParse, matchingParseReq)
	if err != nil {
		t.Fatalf("auth.parse matching failed: %v", err)
	}
	parsedAuth := matchingRes.(pluginapi.AuthParseResponse)
	if !parsedAuth.Handled || parsedAuth.Auth.Provider != "aistudio2api" || parsedAuth.Auth.Attributes["account_id"] != "user@example.com" {
		t.Fatalf("unexpected parsed auth: %+v", parsedAuth)
	}

	// 3. model.static & model.for_auth & model.route
	staticRes, err := p.HandleMethod(pluginabi.MethodModelStatic, []byte(`{}`))
	if err != nil {
		t.Fatalf("model.static failed: %v", err)
	}
	modelResp := staticRes.(pluginapi.ModelResponse)
	if modelResp.Provider != "aistudio2api" || len(modelResp.Models) == 0 {
		t.Fatalf("expected non-empty models from model.static, got: %+v", modelResp)
	}
	hasPrefixed := false
	hasUnprefixed := false
	for _, m := range modelResp.Models {
		if m.ID == "gemini-2.5-pro" {
			hasUnprefixed = true
		}
		if m.ID == "aistudio/gemini-2.5-pro" {
			hasPrefixed = true
		}
	}
	if !hasUnprefixed || !hasPrefixed {
		t.Fatalf("expected both gemini-2.5-pro and aistudio/gemini-2.5-pro in catalog")
	}

	routeReq, _ := json.Marshal(pluginapi.ModelRouteRequest{
		RequestedModel: "aistudio/gemini-2.5-pro",
		Body:           []byte(`{"model":"aistudio/gemini-2.5-pro","messages":[{"role":"user","content":"hi"}]}`),
	})
	routeRes, err := p.HandleMethod(pluginabi.MethodModelRoute, routeReq)
	if err != nil {
		t.Fatalf("model.route failed: %v", err)
	}
	rResp := routeRes.(pluginapi.ModelRouteResponse)
	if !rResp.Handled || rResp.TargetKind != pluginapi.ModelRouteTargetSelf || rResp.TargetModel != "gemini-2.5-pro" {
		t.Fatalf("unexpected model.route response: %+v", rResp)
	}

	// 4. scheduler.pick
	schedReq, _ := json.Marshal(pluginapi.SchedulerPickRequest{
		Provider: "aistudio2api",
		Model:    "gemini-2.5-pro",
		Candidates: []pluginapi.SchedulerAuthCandidate{
			{
				ID:         "aistudio2api-user@example.com.json",
				Provider:   "aistudio2api",
				Status:     "active",
				Attributes: map[string]string{"provider": "aistudio2api", "account_id": "user@example.com"},
			},
		},
	})
	schedRes, err := p.HandleMethod(pluginabi.MethodSchedulerPick, schedReq)
	if err != nil {
		t.Fatalf("scheduler.pick failed: %v", err)
	}
	sResp := schedRes.(pluginapi.SchedulerPickResponse)
	if !sResp.Handled {
		t.Fatalf("expected scheduler.pick Handled=true, got: %+v", sResp)
	}

	// 5. quota.describe & quota.fetch
	qDesc, err := p.HandleMethod(pluginabi.MethodQuotaDescribe, nil)
	if err != nil {
		t.Fatalf("quota.describe failed: %v", err)
	}
	if !qDesc.(pluginapi.QuotaDescribeResponse).SupportsReset {
		t.Fatalf("expected SupportsReset=true")
	}

	qFetch, err := p.HandleMethod(pluginabi.MethodQuotaFetch, []byte(`{"provider":"aistudio2api"}`))
	if err != nil {
		t.Fatalf("quota.fetch failed: %v", err)
	}
	if qFetch.(pluginapi.QuotaFetchResponse).Subscription == nil {
		t.Fatalf("expected Subscription in quota.fetch")
	}

	// 6. management.register & management.handle (Web UI & Status API)
	mgmtReg, err := p.HandleMethod(pluginabi.MethodManagementRegister, nil)
	if err != nil {
		t.Fatalf("management.register failed: %v", err)
	}
	mgmtRegJSON, _ := json.Marshal(mgmtReg)
	if !strings.Contains(string(mgmtRegJSON), `"/accounts"`) {
		t.Fatalf("missing resource route in management.register: %s", string(mgmtRegJSON))
	}

	uiReq, _ := json.Marshal(pluginapi.ManagementRequest{
		Method: http.MethodGet,
		Path:   "/v0/resource/plugins/aistudio2api/accounts",
	})
	uiRes, err := p.HandleMethod(pluginabi.MethodManagementHandle, uiReq)
	if err != nil {
		t.Fatalf("management.handle UI failed: %v", err)
	}
	uiResp := uiRes.(pluginapi.ManagementResponse)
	if uiResp.StatusCode != http.StatusOK || !strings.Contains(string(uiResp.Body), "AIStudio2API 插件控制台") {
		t.Fatalf("unexpected UI response: status=%d len=%d", uiResp.StatusCode, len(uiResp.Body))
	}

	statusReq, _ := json.Marshal(pluginapi.ManagementRequest{
		Method: http.MethodGet,
		Path:   "/v0/management/aistudio2api/status",
	})
	statusRes, err := p.HandleMethod(pluginabi.MethodManagementHandle, statusReq)
	if err != nil {
		t.Fatalf("management.handle status failed: %v", err)
	}
	statusResp := statusRes.(pluginapi.ManagementResponse)
	if statusResp.StatusCode != http.StatusOK || !strings.Contains(string(statusResp.Body), `"waa_backend":"spidermonkey"`) {
		t.Fatalf("unexpected status response: %s", string(statusResp.Body))
	}

}

func TestSpiderMonkeyBotGuardVMIntegration(t *testing.T) {
	// Verify that the embedded SpiderMonkey C++ engine initializes the Firefox DOM, loads a BotGuard VM, and produces a WAA proof snapshot
	rt, err := waa.NewRuntime(t.Context(), waa.Options{
		Challenge: waa.Challenge{
			InterpreterHash: "test_hash",
			Program:         "test_program_bytes",
			GlobalName:      "_bg_test",
		},
		Interpreter: `window._bg_test = { a: function(prog, cb) { cb(function(deliver) { deliver("spidermonkey_waa_proof_" + prog); }, function(){}); } };`,
	})
	if err != nil {
		t.Fatalf("waa.NewRuntime failed: %v", err)
	}
	defer rt.Close()

	proof, err := rt.Proof(t.Context(), "sha256_digest", "hello world")
	if err != nil {
		t.Fatalf("rt.Snapshot failed: %v", err)
	}
	if proof != "spidermonkey_waa_proof_test_program_bytes" {
		t.Fatalf("unexpected WAA proof from SpiderMonkey: %q", proof)
	}
}

func TestBuiltDynamicLibraryExists(t *testing.T) {
	dylibPath := filepath.Join("..", "..", "dist", "plugins", "darwin", "arm64", "aistudio2api.dylib")
	info, err := os.Stat(dylibPath)
	if err != nil {
		t.Skipf("dylib not found at %s: %v", dylibPath, err)
	}
	if info.Size() < 10*1024*1024 {
		t.Fatalf("expected static-linked SpiderMonkey dylib > 10MB, got %d bytes", info.Size())
	}
}
