package cpa

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"github.com/Mag1cFall/AIStudio2API/internal/aistudio"
	"github.com/Mag1cFall/AIStudio2API/internal/api"
	"github.com/Mag1cFall/AIStudio2API/internal/app"
	"github.com/Mag1cFall/AIStudio2API/internal/pluginapi"
)

type rpcManagementRequest struct {
	pluginapi.ManagementRequest
	HostCallbackID string `json:"host_callback_id,omitempty"`
}

type rpcManagementRegistrationResponse struct {
	Routes    []pluginapi.ManagementRoute `json:"routes,omitempty"`
	Resources []pluginapi.ResourceRoute   `json:"resources,omitempty"`
}

func (p *Plugin) handleCLIRegister() (any, error) {
	return pluginapi.CommandLineRegistrationResponse{
		Flags: []pluginapi.CommandLineFlag{
			{
				Name:         "aistudio2api-login",
				Type:         "bool",
				DefaultValue: "false",
				Usage:        "Launch local Chrome Incognito window to sign in to Google AI Studio and import account",
			},
			{
				Name:         "aistudio2api-status",
				Type:         "bool",
				DefaultValue: "false",
				Usage:        "Print AIStudio2API plugin account pool and SpiderMonkey runtime status",
			},
		},
	}, nil
}

func (p *Plugin) handleCLIRun(raw []byte) (any, error) {
	var req pluginapi.CommandLineExecutionRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}
	rt, err := p.Runtime()
	if err != nil {
		return pluginapi.CommandLineExecutionResponse{
			ExitCode: 1,
			Stderr:   []byte("Failed to initialize AIStudio2API runtime: " + err.Error() + "\n"),
		}, nil
	}

	if flag, ok := req.TriggeredFlags["aistudio2api-login"]; ok && flag.Set && strings.EqualFold(flag.Value, "true") {
		created, err := rt.Admin.CreateAccount(p.ctx, api.AccountCreateInput{
			Proxy: p.appCfg.Proxy,
		}, nil)
		if err != nil {
			return pluginapi.CommandLineExecutionResponse{
				ExitCode: 1,
				Stderr:   []byte("Chrome Incognito login failed: " + err.Error() + "\n"),
			}, nil
		}
		_ = p.SyncAccountToHostAuth("", created)
		out, _ := json.MarshalIndent(created, "", "  ")
		return pluginapi.CommandLineExecutionResponse{
			ExitCode: 0,
			Stdout:   []byte("Successfully logged in and imported AI Studio account:\n" + string(out) + "\n"),
		}, nil
	}

	if flag, ok := req.TriggeredFlags["aistudio2api-status"]; ok && flag.Set && strings.EqualFold(flag.Value, "true") {
		status, _ := rt.Admin.Status(p.ctx)
		accounts, _ := rt.Admin.Accounts(p.ctx)
		out, _ := json.MarshalIndent(map[string]any{
			"status":   status,
			"accounts": accounts,
		}, "", "  ")
		return pluginapi.CommandLineExecutionResponse{
			ExitCode: 0,
			Stdout:   []byte(string(out) + "\n"),
		}, nil
	}

	return pluginapi.CommandLineExecutionResponse{ExitCode: 0}, nil
}

func (p *Plugin) handleManagementRegister() (any, error) {
	return rpcManagementRegistrationResponse{
		Routes: []pluginapi.ManagementRoute{
			{Method: http.MethodGet, Path: "/v0/management/aistudio2api/status", Description: "Get AIStudio2API runtime status, accounts, models, and cooldowns"},
			{Method: http.MethodPost, Path: "/v0/management/aistudio2api/service/start", Description: "Start AIStudio2API generation service and SpiderMonkey WAA workers"},
			{Method: http.MethodPost, Path: "/v0/management/aistudio2api/service/stop", Description: "Stop AIStudio2API generation service"},
			{Method: http.MethodGet, Path: "/v0/management/aistudio2api/accounts", Description: "List AIStudio2API accounts"},
			{Method: http.MethodPost, Path: "/v0/management/aistudio2api/accounts", Description: "Import AIStudio2API account from Cookie header or StorageState JSON"},
			{Method: http.MethodPost, Path: "/v0/management/aistudio2api/accounts/login-browser", Description: "Launch local Chrome Incognito window to sign in to Google AI Studio and auto-import account"},
			{Method: http.MethodPost, Path: "/v0/management/aistudio2api/accounts/relogin", Description: "Re-authenticate an existing AIStudio2API account via local Chrome Incognito window"},
			{Method: http.MethodPost, Path: "/v0/management/aistudio2api/accounts/discover-cookies", Description: "Discover Google accounts inside pasted Cookie text"},
			{Method: http.MethodPost, Path: "/v0/management/aistudio2api/accounts/refresh", Description: "Refresh an account's model catalog and benefit tier"},
			{Method: http.MethodPost, Path: "/v0/management/aistudio2api/accounts/update", Description: "Update account settings (enabled, proxy, locale, timezone)"},
			{Method: http.MethodPost, Path: "/v0/management/aistudio2api/accounts/delete", Description: "Delete an account"},
			{Method: http.MethodPost, Path: "/v0/management/aistudio2api/accounts/sync-auth", Description: "Sync all AIStudio2API accounts to CLIProxyAPI auths/ directory"},
			{Method: http.MethodPost, Path: "/v0/management/aistudio2api/cooldowns/clear", Description: "Clear account cooldowns"},
			{Method: http.MethodGet, Path: "/v0/management/aistudio2api/models", Description: "List discovered AI Studio models"},
			{Method: http.MethodGet, Path: "/v0/management/aistudio2api/logs", Description: "List recent AIStudio2API logs"},
		},
		Resources: []pluginapi.ResourceRoute{
			{
				Path:        "/accounts",
				Menu:        "AI Studio (WAA)",
				Description: "AI Studio 账户、SpiderMonkey WAA 与冷却管理",
			},
		},
	}, nil
}

func (p *Plugin) handleManagementHandle(raw []byte) (any, error) {
	var req rpcManagementRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}

	rawPath := strings.TrimSpace(req.Path)
	path := rawPath
	var pathQuery url.Values
	if idx := strings.IndexByte(rawPath, '?'); idx >= 0 {
		path = rawPath[:idx]
		pathQuery, _ = url.ParseQuery(rawPath[idx+1:])
	}
	method := strings.ToUpper(strings.TrimSpace(req.Method))
	if method == "" {
		method = http.MethodGet
	}

	queryParam := func(key string) string {
		if req.Query != nil {
			if v := strings.TrimSpace(req.Query.Get(key)); v != "" {
				return v
			}
		}
		if pathQuery != nil {
			if v := strings.TrimSpace(pathQuery.Get(key)); v != "" {
				return v
			}
		}
		if req.Headers != nil {
			if v := strings.TrimSpace(req.Headers.Get("X-AIStudio-" + key)); v != "" {
				return v
			}
		}
		return ""
	}

	// Serve Web Resource UI or read-only JSON snapshots (matches both relative "/accounts" and full "/v0/resource/plugins/aistudio2api/accounts")
	if path == "/accounts" || path == "/" || strings.HasPrefix(path, "/v0/resource/plugins/aistudio2api") {
		switch strings.ToLower(queryParam("view")) {
		case "status":
			rt, err := p.Runtime()
			if err != nil {
				return jsonManagementResponse(http.StatusInternalServerError, map[string]any{"error": err.Error()}), nil
			}
			return p.buildStatusManagementResponse(rt), nil
		case "logs":
			rt, err := p.Runtime()
			if err != nil {
				return jsonManagementResponse(http.StatusInternalServerError, map[string]any{"error": err.Error()}), nil
			}
			return jsonManagementResponse(http.StatusOK, map[string]any{"logs": rt.RecentLogs()}), nil
		}
		return pluginapi.ManagementResponse{
			StatusCode: http.StatusOK,
			Headers: http.Header{
				"Content-Type":  []string{"text/html; charset=utf-8"},
				"Cache-Control": []string{"no-store"},
			},
			Body: []byte(webUIDashboardHTML),
		}, nil
	}
	if strings.HasPrefix(path, "/aistudio2api/") {
		path = "/v0/management" + path
	}
	rt, err := p.Runtime()
	if err != nil {
		return jsonManagementResponse(http.StatusInternalServerError, map[string]any{
			"error": err.Error(),
		}), nil
	}

	switch {
	case method == http.MethodGet && path == "/v0/management/aistudio2api/status":
		return p.buildStatusManagementResponse(rt), nil
	case method == http.MethodPost && path == "/v0/management/aistudio2api/service/start":
		status, err := rt.Admin.StartService(p.ctx)
		if err != nil {
			return jsonManagementResponse(http.StatusBadGateway, map[string]any{"error": err.Error(), "status": status}), nil
		}
		return jsonManagementResponse(http.StatusOK, map[string]any{"status": status}), nil

	case method == http.MethodPost && path == "/v0/management/aistudio2api/service/stop":
		status, err := rt.Admin.StopService(p.ctx)
		if err != nil {
			return jsonManagementResponse(http.StatusBadGateway, map[string]any{"error": err.Error(), "status": status}), nil
		}
		return jsonManagementResponse(http.StatusOK, map[string]any{"status": status}), nil

	case method == http.MethodGet && path == "/v0/management/aistudio2api/accounts":
		accounts, err := rt.Admin.Accounts(p.ctx)
		if err != nil {
			return jsonManagementResponse(http.StatusInternalServerError, map[string]any{"error": err.Error()}), nil
		}
		return jsonManagementResponse(http.StatusOK, map[string]any{"accounts": accounts}), nil


	case method == http.MethodPost && path == "/v0/management/aistudio2api/accounts/login-browser":
		if !p.chromeLogin.TryLock() {
			return jsonManagementResponse(http.StatusConflict, map[string]any{"error": "已有一个本机 Chrome 无痕登录窗口正在进行中，请在该窗口完成登录或关闭它后重试"}), nil
		}
		defer p.chromeLogin.Unlock()
		var input api.AccountCreateInput
		if len(req.Body) > 0 {
			_ = json.Unmarshal(req.Body, &input)
		}
		created, err := rt.Admin.CreateAccount(p.ctx, input, nil)
		if err != nil {
			return jsonManagementResponse(http.StatusBadRequest, map[string]any{"error": err.Error()}), nil
		}
		_ = p.SyncAccountToHostAuth(req.HostCallbackID, created)
		go func() { _ = rt.EnsureRunning(p.ctx) }()
		return jsonManagementResponse(http.StatusCreated, map[string]any{"account": created}), nil
	case method == http.MethodPost && path == "/v0/management/aistudio2api/accounts/relogin":
		if !p.chromeLogin.TryLock() {
			return jsonManagementResponse(http.StatusConflict, map[string]any{"error": "已有一个本机 Chrome 无痕登录窗口正在进行中，请在该窗口完成登录或关闭它后重试"}), nil
		}
		defer p.chromeLogin.Unlock()
		var body struct {
			AccountID string `json:"account_id"`
			Proxy     string `json:"proxy"`
		}
		if len(req.Body) > 0 {
			_ = json.Unmarshal(req.Body, &body)
		}
		accountID := strings.TrimSpace(body.AccountID)
		var updated api.AdminAccount
		var err error
		if accountID != "" {
			updated, err = rt.Admin.LoginAccount(p.ctx, accountID, nil)
		} else {
			updated, err = rt.Admin.CreateAccount(p.ctx, api.AccountCreateInput{Proxy: body.Proxy}, nil)
		}
		if err != nil {
			return jsonManagementResponse(http.StatusBadRequest, map[string]any{"error": err.Error()}), nil
		}
		_ = p.SyncAccountToHostAuth(req.HostCallbackID, updated)
		go func() { _ = rt.EnsureRunning(p.ctx) }()
		return jsonManagementResponse(http.StatusOK, map[string]any{"account": updated}), nil
	case method == http.MethodPost && path == "/v0/management/aistudio2api/accounts/discover-cookies":
		var input api.CookieImportInput
		if err := json.Unmarshal(req.Body, &input); err != nil {
			return jsonManagementResponse(http.StatusBadRequest, map[string]any{"error": err.Error()}), nil
		}
		discovered, err := rt.Admin.DiscoverCookieAccounts(p.ctx, input)
		if err != nil {
			return jsonManagementResponse(http.StatusBadRequest, map[string]any{"error": err.Error()}), nil
		}
		return jsonManagementResponse(http.StatusOK, map[string]any{"accounts": discovered}), nil

	case method == http.MethodPost && path == "/v0/management/aistudio2api/accounts":
		var body struct {
			Label        string                 `json:"label"`
			Email        string                 `json:"email"`
			AuthUser     int                    `json:"auth_user"`
			Proxy        string                 `json:"proxy"`
			Locale       string                 `json:"locale"`
			Timezone     string                 `json:"timezone"`
			Cookies      string                 `json:"cookies"`
			StorageState *aistudio.StorageState `json:"storage_state"`
		}
		if err := json.Unmarshal(req.Body, &body); err != nil {
			return jsonManagementResponse(http.StatusBadRequest, map[string]any{"error": "invalid JSON body: " + err.Error()}), nil
		}
		var state aistudio.StorageState
		if body.StorageState != nil && len(body.StorageState.Cookies) > 0 {
			state = *body.StorageState
		} else if strings.TrimSpace(body.Cookies) != "" {
			parsed, err := aistudio.ParseCookieText(body.Cookies)
			if err != nil {
				return jsonManagementResponse(http.StatusBadRequest, map[string]any{"error": "failed to parse cookies: " + err.Error()}), nil
			}
			state = parsed
		} else {
			return jsonManagementResponse(http.StatusBadRequest, map[string]any{"error": "cookies or storage_state is required"}), nil
		}

		label := strings.TrimSpace(body.Label)
		if label == "" {
			label = strings.TrimSpace(body.Email)
		}
		created, err := rt.Admin.ImportAccountState(p.ctx, api.AccountStateInput{
			Label:        label,
			AuthUser:     body.AuthUser,
			Proxy:        body.Proxy,
			Locale:       body.Locale,
			Timezone:     body.Timezone,
			StorageState: state,
		})
		if err != nil {
			return jsonManagementResponse(http.StatusBadRequest, map[string]any{"error": err.Error()}), nil
		}
		_ = p.SyncAccountToHostAuth(req.HostCallbackID, created)
		go func() { _ = rt.EnsureRunning(p.ctx) }()
		return jsonManagementResponse(http.StatusCreated, map[string]any{"account": created}), nil

	case method == http.MethodPost && path == "/v0/management/aistudio2api/accounts/refresh":
		var body struct {
			AccountID string `json:"account_id"`
		}
		_ = json.Unmarshal(req.Body, &body)
		if strings.TrimSpace(body.AccountID) == "" {
			return jsonManagementResponse(http.StatusBadRequest, map[string]any{"error": "account_id is required"}), nil
		}
		models, err := rt.RefreshAccountModels(p.ctx, body.AccountID)
		if err != nil {
			return jsonManagementResponse(http.StatusBadGateway, map[string]any{"error": err.Error()}), nil
		}
		if accounts, err := rt.Admin.Accounts(p.ctx); err == nil {
			for _, acc := range accounts {
				if strings.EqualFold(acc.ID, body.AccountID) {
					_ = p.SyncAccountToHostAuth(req.HostCallbackID, acc)
				}
			}
		}
		return jsonManagementResponse(http.StatusOK, map[string]any{"models_count": len(models), "models": models}), nil

	case method == http.MethodPost && path == "/v0/management/aistudio2api/accounts/update":
		var body struct {
			AccountID string `json:"account_id"`
			Enabled   bool   `json:"enabled"`
			Proxy     string `json:"proxy"`
			Locale    string `json:"locale"`
			Timezone  string `json:"timezone"`
		}
		if err := json.Unmarshal(req.Body, &body); err != nil {
			return jsonManagementResponse(http.StatusBadRequest, map[string]any{"error": err.Error()}), nil
		}
		updated, err := rt.Admin.UpdateAccount(p.ctx, body.AccountID, api.AccountInput{
			Label:    body.AccountID,
			Enabled:  body.Enabled,
			Proxy:    body.Proxy,
			Locale:   body.Locale,
			Timezone: body.Timezone,
		})
		if err != nil {
			return jsonManagementResponse(http.StatusBadRequest, map[string]any{"error": err.Error()}), nil
		}
		_ = p.SyncAccountToHostAuth(req.HostCallbackID, updated)
		return jsonManagementResponse(http.StatusOK, map[string]any{"account": updated}), nil

	case method == http.MethodPost && path == "/v0/management/aistudio2api/accounts/delete":
		var body struct {
			AccountID string `json:"account_id"`
		}
		if err := json.Unmarshal(req.Body, &body); err != nil {
			return jsonManagementResponse(http.StatusBadRequest, map[string]any{"error": err.Error()}), nil
		}
		if err := rt.Admin.DeleteAccount(p.ctx, body.AccountID); err != nil {
			return jsonManagementResponse(http.StatusBadRequest, map[string]any{"error": err.Error()}), nil
		}
		p.RemoveAccountFromHostAuth(body.AccountID)
		return jsonManagementResponse(http.StatusOK, map[string]any{"deleted": body.AccountID}), nil

	case method == http.MethodPost && path == "/v0/management/aistudio2api/accounts/sync-auth":
		accounts, err := rt.Admin.Accounts(p.ctx)
		if err != nil {
			return jsonManagementResponse(http.StatusInternalServerError, map[string]any{"error": err.Error()}), nil
		}
		synced := 0
		var syncErrors []string
		for _, acc := range accounts {
			if err := p.SyncAccountToHostAuth(req.HostCallbackID, acc); err != nil {
				syncErrors = append(syncErrors, fmt.Sprintf("%s: %v", acc.ID, err))
			} else {
				synced++
			}
		}
		return jsonManagementResponse(http.StatusOK, map[string]any{
			"synced": synced,
			"errors": syncErrors,
		}), nil

	case method == http.MethodPost && path == "/v0/management/aistudio2api/cooldowns/clear":
		var body struct {
			AccountID string `json:"account_id"`
			Scope     string `json:"scope"`
		}
		_ = json.Unmarshal(req.Body, &body)
		if strings.TrimSpace(body.AccountID) != "" {
			if err := rt.Admin.ClearCooldown(p.ctx, body.AccountID, body.Scope); err != nil {
				return jsonManagementResponse(http.StatusBadRequest, map[string]any{"error": err.Error()}), nil
			}
		} else {
			for _, st := range rt.Pool().Status() {
				_ = rt.Admin.ClearCooldown(p.ctx, st.ID, body.Scope)
			}
		}
		return jsonManagementResponse(http.StatusOK, map[string]any{"cleared": true}), nil

	case method == http.MethodGet && path == "/v0/management/aistudio2api/models":
		models, err := rt.Admin.CatalogModels(p.ctx)
		if err != nil || len(models) == 0 {
			models = p.collectRawCatalogModels("")
		}
		return jsonManagementResponse(http.StatusOK, map[string]any{"models": models}), nil

	case method == http.MethodGet && path == "/v0/management/aistudio2api/logs":
		logs := rt.RecentLogs()
		return jsonManagementResponse(http.StatusOK, map[string]any{"logs": logs}), nil

	default:
		return jsonManagementResponse(http.StatusNotFound, map[string]any{
			"error": fmt.Sprintf("route %s %s not found", method, path),
		}), nil
	}
}

func (p *Plugin) buildStatusManagementResponse(rt *app.PluginRuntime) pluginapi.ManagementResponse {
	status, _ := rt.Admin.Status(p.ctx)
	accounts, _ := rt.Admin.Accounts(p.ctx)
	cooldowns, _ := rt.Admin.Cooldowns(p.ctx)
	activeReqs, _ := rt.Admin.Requests(p.ctx)
	models, _ := rt.Admin.CatalogModels(p.ctx)
	chromeBin, _ := aistudio.FindLocalChromeBinary()
	return jsonManagementResponse(http.StatusOK, map[string]any{
		"status":           status,
		"accounts":         accounts,
		"cooldowns":        cooldowns,
		"active_requests":  activeReqs,
		"models_count":     len(models),
		"waa_backend":      p.appCfg.WAABackend,
		"data_dir":         p.pluginCfg.DataDir,
		"accounts_dir":     p.appCfg.AuthStates,
		"chrome_available": chromeBin != "",
		"chrome_binary":    chromeBin,
	})
}

func jsonManagementResponse(status int, payload any) pluginapi.ManagementResponse {
	raw, _ := json.Marshal(payload)
	return pluginapi.ManagementResponse{
		StatusCode: status,
		Headers: http.Header{
			"Content-Type": []string{"application/json; charset=utf-8"},
		},
		Body: raw,
	}
}
