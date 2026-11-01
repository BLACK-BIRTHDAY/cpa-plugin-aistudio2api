package cpa

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Mag1cFall/AIStudio2API/internal/aistudio"
	"github.com/Mag1cFall/AIStudio2API/internal/api"
	"github.com/Mag1cFall/AIStudio2API/internal/app"
	"github.com/Mag1cFall/AIStudio2API/internal/pluginabi"
	"github.com/Mag1cFall/AIStudio2API/internal/pluginapi"
	"github.com/google/uuid"
)

type rpcIdentifierResponse struct {
	Identifier          string   `json:"identifier"`
	Identifiers         []string `json:"identifiers,omitempty"`
	CredentialProviders []string `json:"credential_providers,omitempty"`
}

type rpcAuthLoginStartRequest struct {
	pluginapi.AuthLoginStartRequest
	HostCallbackID string `json:"host_callback_id,omitempty"`
}

type rpcAuthLoginPollRequest struct {
	pluginapi.AuthLoginPollRequest
	HostCallbackID string `json:"host_callback_id,omitempty"`
}

type rpcAuthRefreshRequest struct {
	pluginapi.AuthRefreshRequest
	HostCallbackID string `json:"host_callback_id,omitempty"`
}

// AuthFilePayload is the JSON structure stored in CLIProxyAPI's auths/ directory for aistudio2api.
type AuthFilePayload struct {
	Type         string                 `json:"type"`
	Provider     string                 `json:"provider,omitempty"`
	AccountID    string                 `json:"account_id,omitempty"`
	Email        string                 `json:"email,omitempty"`
	Label        string                 `json:"label,omitempty"`
	Mode         string                 `json:"mode,omitempty"`
	Proxy        string                 `json:"proxy,omitempty"`
	Locale       string                 `json:"locale,omitempty"`
	Timezone     string                 `json:"timezone,omitempty"`
	AuthUser     int                    `json:"auth_user,omitempty"`
	Cookies      string                 `json:"cookies,omitempty"`
	CookieHeader string                 `json:"cookie_header,omitempty"`
	StorageState *aistudio.StorageState `json:"storage_state,omitempty"`
	Tier         string                 `json:"tier,omitempty"`
	State        string                 `json:"state,omitempty"`
	UpdatedAt    string                 `json:"updated_at,omitempty"`
}

func isAIStudioProvider(provider string) bool {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "aistudio2api", "aistudio", "google-aistudio", "makersuite":
		return true
	default:
		return false
	}
}

func (p *Plugin) handleAuthIdentifier() (any, error) {
	return rpcIdentifierResponse{
		Identifier:          ProviderID,
		Identifiers:         []string{"aistudio2api", "aistudio", "google-aistudio"},
		CredentialProviders: []string{"aistudio2api", "aistudio", "google-aistudio"},
	}, nil
}

func (p *Plugin) handleAuthParse(raw []byte) (any, error) {
	var req pluginapi.AuthParseRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, fmt.Errorf("decode auth.parse request: %w", err)
	}

	var payload AuthFilePayload
	if len(req.RawJSON) > 0 {
		_ = json.Unmarshal(req.RawJSON, &payload)
	}

	fileName := strings.ToLower(strings.TrimSpace(req.FileName))
	baseName := strings.ToLower(filepath.Base(fileName))
	slashPath := strings.ToLower(filepath.ToSlash(req.Path))
	switch baseName {
	case "account.json", "storage-state.json", "runtime-state.json", "camoufox-fingerprint.json", "catalog.json":
		return pluginapi.AuthParseResponse{Handled: false}, nil
	}
	if strings.Contains(slashPath, "/accounts/") || strings.Contains(slashPath, "/chrome-cache/") || strings.Contains(filepath.ToSlash(fileName), "/") {
		return pluginapi.AuthParseResponse{Handled: false}, nil
	}

	providerMatched := isAIStudioProvider(payload.Type) ||
		isAIStudioProvider(payload.Provider) ||
		strings.HasPrefix(baseName, "aistudio2api-") ||
		strings.HasPrefix(baseName, "aistudio-")

	if !providerMatched {
		return pluginapi.AuthParseResponse{Handled: false}, nil
	}

	authData, err := p.parseAndSyncAuthPayload(p.ctx, req.FileName, req.RawJSON, payload)
	if err != nil {
		return nil, err
	}
	return pluginapi.AuthParseResponse{
		Handled: true,
		Auth:    authData,
	}, nil
}

func (p *Plugin) parseAndSyncAuthPayload(ctx context.Context, fileName string, rawJSON []byte, payload AuthFilePayload) (pluginapi.AuthData, error) {
	accountID := strings.ToLower(strings.TrimSpace(payload.AccountID))
	if accountID == "" {
		accountID = strings.ToLower(strings.TrimSpace(payload.Email))
	}
	if accountID == "" {
		accountID = strings.ToLower(strings.TrimSpace(payload.Label))
	}

	state, hasState := extractStorageStateFromPayload(rawJSON, payload)
	rt, rtErr := p.Runtime()
	if rtErr == nil && hasState {
		imported, err := p.upsertAccountInRuntime(ctx, rt, accountID, payload, state)
		if err == nil && imported.ID != "" {
			accountID = strings.ToLower(strings.TrimSpace(imported.ID))
			payload.AccountID = accountID
			payload.Email = accountID
			payload.Label = imported.Label
			payload.Tier = string(imported.BenefitTier)
			payload.State = string(imported.State)
		}
	}

	if accountID == "" {
		base := strings.TrimSuffix(filepath.Base(fileName), filepath.Ext(fileName))
		base = strings.TrimPrefix(base, "aistudio2api-")
		base = strings.TrimPrefix(base, "aistudio-")
		if strings.Contains(base, "@") {
			accountID = strings.ToLower(strings.TrimSpace(base))
		}
	}

	label := strings.TrimSpace(payload.Label)
	if label == "" {
		label = accountID
	}
	if label == "" {
		label = "aistudio2api-pool"
	}

	authID := fileName
	if authID == "" {
		if accountID != "" {
			authID = "aistudio2api-" + accountID + ".json"
		} else {
			authID = "aistudio2api-pool.json"
		}
	}

	payload.Type = ProviderID
	payload.Provider = ProviderID
	payload.AccountID = accountID
	payload.Label = label
	if hasState && payload.StorageState == nil {
		payload.StorageState = &state
	}
	if payload.UpdatedAt == "" {
		payload.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	storageBytes, err := json.Marshal(payload)
	if err != nil {
		storageBytes = rawJSON
	}

	metadata := map[string]any{
		"type":       ProviderID,
		"provider":   ProviderID,
		"account_id": accountID,
		"email":      accountID,
		"label":      label,
	}
	if payload.Tier != "" {
		metadata["tier"] = payload.Tier
	}
	if payload.State != "" {
		metadata["state"] = payload.State
	}

	attributes := map[string]string{
		"provider":   ProviderID,
		"account_id": accountID,
	}
	if payload.Tier != "" {
		attributes["tier"] = payload.Tier
	}

	return pluginapi.AuthData{
		ID:               authID,
		Provider:         ProviderID,
		FileName:         fileName,
		Label:            label,
		ProxyURL:         strings.TrimSpace(payload.Proxy),
		StorageJSON:      storageBytes,
		Metadata:         metadata,
		Attributes:       attributes,
		NextRefreshAfter: time.Now().Add(30 * time.Minute),
	}, nil
}

func extractStorageStateFromPayload(rawJSON []byte, payload AuthFilePayload) (aistudio.StorageState, bool) {
	if payload.StorageState != nil && len(payload.StorageState.Cookies) > 0 {
		if err := payload.StorageState.Validate(); err == nil {
			return *payload.StorageState, true
		}
	}
	cookieSource := strings.TrimSpace(payload.Cookies)
	if cookieSource == "" {
		cookieSource = strings.TrimSpace(payload.CookieHeader)
	}
	if cookieSource != "" {
		if state, err := aistudio.ParseCookieText(cookieSource); err == nil && len(state.Cookies) > 0 {
			return state, true
		}
	}
	if len(rawJSON) > 0 {
		if state, err := aistudio.ParseCookieText(string(rawJSON)); err == nil && len(state.Cookies) > 0 {
			return state, true
		}
	}
	return aistudio.StorageState{}, false
}

func (p *Plugin) upsertAccountInRuntime(
	ctx context.Context,
	rt *app.PluginRuntime,
	accountID string,
	payload AuthFilePayload,
	state aistudio.StorageState,
) (api.AdminAccount, error) {
	if err := state.Validate(); err != nil {
		return api.AdminAccount{}, err
	}
	// Check if account already exists in pool
	if accountID != "" {
		if existing, ok := rt.Pool().AccountByID(accountID); ok && existing != nil {
			accounts, _ := rt.Admin.Accounts(ctx)
			for _, acc := range accounts {
				if strings.EqualFold(acc.ID, accountID) {
					return acc, nil
				}
			}
		}
	}
	created, err := rt.Admin.ImportAccountState(ctx, api.AccountStateInput{
		Label:        accountID,
		AuthUser:     payload.AuthUser,
		Proxy:        payload.Proxy,
		Locale:       payload.Locale,
		Timezone:     payload.Timezone,
		StorageState: state,
	})
	if err != nil {
		// If account already exists under the discovered email, return it
		accounts, _ := rt.Admin.Accounts(ctx)
		for _, acc := range accounts {
			if accountID != "" && strings.EqualFold(acc.ID, accountID) {
				return acc, nil
			}
		}
		if len(accounts) > 0 && strings.Contains(err.Error(), "已存在") {
			return accounts[0], nil
		}
		return api.AdminAccount{}, err
	}
	return created, nil
}

func (p *Plugin) handleAuthLoginStart(raw []byte) (any, error) {
	var req rpcAuthLoginStartRequest
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &req)
	}
	rt, err := p.Runtime()
	if err != nil {
		return nil, err
	}
	initial := make(map[string]struct{})
	if accounts, err := rt.Admin.Accounts(p.ctx); err == nil {
		for _, acc := range accounts {
			initial[strings.ToLower(strings.TrimSpace(acc.ID))] = struct{}{}
		}
	}
	stateID := uuid.NewString()
	p.loginStateMu.Lock()
	p.loginStates[stateID] = loginSessionState{
		CreatedAt:       time.Now(),
		InitialAccounts: initial,
		Provider:        ProviderID,
	}
	p.loginStateMu.Unlock()

	loginURL := "/v0/resource/plugins/aistudio2api/accounts"
	return pluginapi.AuthLoginStartResponse{
		Provider:  ProviderID,
		URL:       loginURL,
		State:     stateID,
		ExpiresAt: time.Now().Add(10 * time.Minute),
		Metadata: map[string]any{
			"instructions": "Open /v0/resource/plugins/aistudio2api/accounts in your browser to import Google AI Studio Cookies, StorageState JSON, or Chrome profiles.",
			"resource_url": loginURL,
		},
	}, nil
}

func (p *Plugin) handleAuthLoginPoll(raw []byte) (any, error) {
	var req rpcAuthLoginPollRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}
	rt, err := p.Runtime()
	if err != nil {
		return nil, err
	}
	p.loginStateMu.Lock()
	session, exists := p.loginStates[strings.TrimSpace(req.State)]
	p.loginStateMu.Unlock()

	accounts, err := rt.Admin.Accounts(p.ctx)
	if err != nil {
		return nil, err
	}
	for _, acc := range accounts {
		id := strings.ToLower(strings.TrimSpace(acc.ID))
		if exists {
			if _, wasPresent := session.InitialAccounts[id]; wasPresent {
				continue
			}
		}
		authData, err := p.buildAuthDataForAccount(rt, acc)
		if err != nil {
			continue
		}
		p.loginStateMu.Lock()
		delete(p.loginStates, strings.TrimSpace(req.State))
		p.loginStateMu.Unlock()
		return pluginapi.AuthLoginPollResponse{
			Status:  "success",
			Message: "AI Studio account imported: " + acc.Label,
			Auth:    authData,
		}, nil
	}
	return pluginapi.AuthLoginPollResponse{
		Status:  "pending",
		Message: "Waiting for account import at /v0/resource/plugins/aistudio2api/accounts",
	}, nil
}

func (p *Plugin) handleAuthRefresh(raw []byte) (any, error) {
	var req rpcAuthRefreshRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}
	rt, err := p.Runtime()
	if err != nil {
		return nil, err
	}

	accountID := extractAccountIDFromAuth(req.StorageJSON, req.Metadata, req.Attributes)
	if accountID == "" {
		accounts, _ := rt.Admin.Accounts(p.ctx)
		if len(accounts) > 0 {
			accountID = accounts[0].ID
		}
	}
	if accountID == "" {
		return pluginapi.AuthRefreshResponse{
			Auth: pluginapi.AuthData{
				ID:               req.AuthID,
				Provider:         ProviderID,
				StorageJSON:      req.StorageJSON,
				Metadata:         req.Metadata,
				Attributes:       req.Attributes,
				NextRefreshAfter: time.Now().Add(30 * time.Minute),
			},
			NextRefreshAfter: time.Now().Add(30 * time.Minute),
		}, nil
	}

	_, _ = rt.RefreshAccountModels(p.ctx, accountID)
	accounts, _ := rt.Admin.Accounts(p.ctx)
	for _, acc := range accounts {
		if strings.EqualFold(acc.ID, accountID) {
			authData, err := p.buildAuthDataForAccount(rt, acc)
			if err == nil {
				if req.AuthID != "" {
					authData.ID = req.AuthID
				}
				return pluginapi.AuthRefreshResponse{
					Auth:             authData,
					NextRefreshAfter: authData.NextRefreshAfter,
				}, nil
			}
		}
	}

	return pluginapi.AuthRefreshResponse{
		Auth: pluginapi.AuthData{
			ID:               req.AuthID,
			Provider:         ProviderID,
			StorageJSON:      req.StorageJSON,
			Metadata:         req.Metadata,
			Attributes:       req.Attributes,
			NextRefreshAfter: time.Now().Add(30 * time.Minute),
		},
		NextRefreshAfter: time.Now().Add(30 * time.Minute),
	}, nil
}

func (p *Plugin) buildAuthDataForAccount(rt *app.PluginRuntime, acc api.AdminAccount) (pluginapi.AuthData, error) {
	accountID := strings.ToLower(strings.TrimSpace(acc.ID))
	fileName := fmt.Sprintf("aistudio2api-%s.json", sanitizeFileComponent(accountID))

	payload := AuthFilePayload{
		Type:      ProviderID,
		Provider:  ProviderID,
		AccountID: accountID,
		Email:     accountID,
		Label:     acc.Label,
		Proxy:     acc.Proxy,
		Locale:    acc.Locale,
		Timezone:  acc.Timezone,
		Tier:      string(acc.BenefitTier),
		State:     string(acc.State),
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	if poolAcc, ok := rt.Pool().AccountByID(accountID); ok && poolAcc != nil && poolAcc.Directory != "" {
		statePath := filepath.Join(poolAcc.Directory, "storage-state.json")
		if rawState, err := os.ReadFile(statePath); err == nil {
			var st aistudio.StorageState
			if json.Unmarshal(rawState, &st) == nil && len(st.Cookies) > 0 {
				payload.StorageState = &st
			}
		}
	}

	storageBytes, err := json.Marshal(payload)
	if err != nil {
		return pluginapi.AuthData{}, err
	}

	return pluginapi.AuthData{
		ID:               fileName,
		Provider:         ProviderID,
		FileName:         fileName,
		Label:            acc.Label,
		Disabled:         !acc.Enabled,
		ProxyURL:         acc.Proxy,
		StorageJSON:      storageBytes,
		NextRefreshAfter: time.Now().Add(30 * time.Minute),
		Metadata: map[string]any{
			"type":       ProviderID,
			"provider":   ProviderID,
			"account_id": accountID,
			"email":      accountID,
			"label":      acc.Label,
			"tier":       string(acc.BenefitTier),
			"state":      string(acc.State),
		},
		Attributes: map[string]string{
			"provider":   ProviderID,
			"account_id": accountID,
			"tier":       string(acc.BenefitTier),
		},
	}, nil
}

func (p *Plugin) SyncAccountToHostAuth(callbackID string, acc api.AdminAccount) error {
	if !boolOrDefault(p.Config().SyncAuthFiles, true) {
		return nil
	}
	rt, err := p.Runtime()
	if err != nil {
		return err
	}
	authData, err := p.buildAuthDataForAccount(rt, acc)
	if err != nil {
		return err
	}
	if err := writeAuthDataToDefaultHostDir(authData); err == nil && strings.TrimSpace(callbackID) == "" {
		return nil
	}
	req := map[string]any{
		"host_callback_id": strings.TrimSpace(callbackID),
		"name":             authData.FileName,
		"json":             json.RawMessage(authData.StorageJSON),
		"metadata":         authData.Metadata,
		"attributes":       authData.Attributes,
	}
	if _, err := p.callHost(pluginabi.MethodHostAuthSave, req); err != nil {
		return writeAuthDataToDefaultHostDir(authData)
	}
	return nil
}

func writeAuthDataToDefaultHostDir(authData pluginapi.AuthData) error {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return fmt.Errorf("resolve home dir: %w", err)
	}
	authDir := filepath.Join(home, ".cli-proxy-api")
	if info, err := os.Stat(authDir); err != nil || !info.IsDir() {
		return fmt.Errorf("host auth dir %s not found", authDir)
	}
	target := filepath.Join(authDir, filepath.Base(authData.FileName))
	return os.WriteFile(target, authData.StorageJSON, 0o600)
}

func (p *Plugin) SyncAllAccountsToHostDir(rt *app.PluginRuntime) {
	if rt == nil {
		return
	}
	accounts, err := rt.Admin.Accounts(p.ctx)
	if err != nil {
		return
	}
	for _, acc := range accounts {
		if authData, err := p.buildAuthDataForAccount(rt, acc); err == nil {
			_ = writeAuthDataToDefaultHostDir(authData)
		}
	}
}

func (p *Plugin) RemoveAccountFromHostAuth(accountID string) {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return
	}
	fileName := fmt.Sprintf("aistudio2api-%s.json", sanitizeFileComponent(strings.ToLower(strings.TrimSpace(accountID))))
	_ = os.Remove(filepath.Join(home, ".cli-proxy-api", fileName))
}

func extractAccountIDFromAuth(storageJSON []byte, metadata map[string]any, attributes map[string]string) string {
	if attributes != nil {
		if id := strings.ToLower(strings.TrimSpace(attributes["account_id"])); id != "" {
			return id
		}
		if id := strings.ToLower(strings.TrimSpace(attributes["email"])); id != "" {
			return id
		}
	}
	if metadata != nil {
		if id, ok := metadata["account_id"].(string); ok && strings.TrimSpace(id) != "" {
			return strings.ToLower(strings.TrimSpace(id))
		}
		if id, ok := metadata["email"].(string); ok && strings.TrimSpace(id) != "" {
			return strings.ToLower(strings.TrimSpace(id))
		}
	}
	if len(storageJSON) > 0 {
		var payload AuthFilePayload
		if json.Unmarshal(storageJSON, &payload) == nil {
			if id := strings.ToLower(strings.TrimSpace(payload.AccountID)); id != "" {
				return id
			}
			if id := strings.ToLower(strings.TrimSpace(payload.Email)); id != "" {
				return id
			}
			if id := strings.ToLower(strings.TrimSpace(payload.Label)); strings.Contains(id, "@") {
				return id
			}
		}
	}
	return ""
}

func sanitizeFileComponent(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '@' || r == '.' || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "account"
	}
	return b.String()
}
