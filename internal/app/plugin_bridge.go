package app

import (
	"context"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/Mag1cFall/AIStudio2API/internal/aistudio"
	"github.com/Mag1cFall/AIStudio2API/internal/api"
	"github.com/Mag1cFall/AIStudio2API/internal/config"
)

type pinnedAccountContextKey struct{}

// WithPinnedAccountID attaches a specific AI Studio account ID (email) to the request context.
func WithPinnedAccountID(ctx context.Context, accountID string) context.Context {
	accountID = strings.ToLower(strings.TrimSpace(accountID))
	if accountID == "" {
		return ctx
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, pinnedAccountContextKey{}, accountID)
}

// ContextWithPinnedAccountID is an alias for WithPinnedAccountID.
func ContextWithPinnedAccountID(ctx context.Context, accountID string) context.Context {
	return WithPinnedAccountID(ctx, accountID)
}

// PinnedAccountIDFromContext returns the pinned account ID from context when set.
func PinnedAccountIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(pinnedAccountContextKey{}).(string)
	return strings.ToLower(strings.TrimSpace(v))
}

// PluginRuntime wraps runtimeManager and api.NewHandler for CLIProxyAPI plugin execution.
type PluginRuntime struct {
	Manager *runtimeManager
	Admin   api.AdminService
	Handler http.Handler
	Config  config.Config
}

// NewPluginRuntime creates and initializes a PluginRuntime from config.Config.
func NewPluginRuntime(ctx context.Context, cfg config.Config) (*PluginRuntime, error) {
	authStates := cfg.AuthStates
	proxy := cfg.Proxy
	overrides := dataConfigOverrides{
		authStates: &authStates,
		proxy:      &proxy,
	}
	mgr, err := newRuntimeManager(ctx, "", cfg, overrides)
	if err != nil {
		return nil, err
	}
	mediaDir := ""
	if cfg.AuthStates != "" {
		firstDir := strings.TrimSpace(strings.Split(cfg.AuthStates, ",")[0])
		if firstDir != "" {
			mediaDir = filepath.Join(firstDir, ".media")
		}
	}
	handler := api.NewHandler(mgr, api.Config{
		Admin:          mgr,
		MediaDir:       mediaDir,
		MediaLinks:     mgr.mediaLinks,
		ResponseModes:  mgr.responseModes,
		SearchSettings: mgr.searchSettings,
		WaitLaunch:     mgr.WaitLaunch,
		EgressProxy:    mgr.egressProxy,
	})
	rt := &PluginRuntime{
		Manager: mgr,
		Admin:   mgr,
		Handler: handler,
		Config:  cfg,
	}
	if cfg.AutoStart {
		go func() {
			_ = rt.EnsureRunning(ctx)
		}()
	}
	return rt, nil
}

// PublicHandler returns the HTTP handler for OpenAI, Responses, Anthropic, and Gemini endpoints.
func (rt *PluginRuntime) PublicHandler() http.Handler {
	return rt.Handler
}

// Close stops and releases the runtime.
func (rt *PluginRuntime) Close() error {
	if rt == nil || rt.Manager == nil {
		return nil
	}
	return rt.Manager.Close()
}

// EnsureRunning starts the generation service if it is currently STOPPED and at least one account exists.
func (rt *PluginRuntime) EnsureRunning(ctx context.Context) error {
	rt.Manager.mu.RLock()
	state := rt.Manager.current.service.State()
	admin := rt.Manager.current.admin
	rt.Manager.mu.RUnlock()
	if state == "RUNNING" || state == "LAUNCHING" {
		return nil
	}
	if pool := rt.Pool(); pool != nil {
		if _, eligible := pool.EnabledAccounts(); eligible == 0 {
			return aistudio.ErrNoEligibleAccount
		}
	}
	_, err := admin.StartService(ctx)
	return err
}

// Pool returns the current AccountPool.
func (rt *PluginRuntime) Pool() *aistudio.AccountPool {
	rt.Manager.mu.RLock()
	defer rt.Manager.mu.RUnlock()
	if ts, ok := rt.Manager.current.service.(*trackedService); ok {
		return ts.pool
	}
	return nil
}

// Store returns the current AccountStore.
func (rt *PluginRuntime) Store() *aistudio.AccountStore {
	rt.Manager.mu.RLock()
	defer rt.Manager.mu.RUnlock()
	if ra, ok := rt.Manager.current.admin.(*runtimeAdmin); ok {
		return ra.store
	}
	return nil
}

// RefreshAccountModels refreshes or loads the live model catalog for a single account.
func (rt *PluginRuntime) RefreshAccountModels(ctx context.Context, accountID string) ([]aistudio.Model, error) {
	rt.Manager.mu.RLock()
	ts, _ := rt.Manager.current.service.(*trackedService)
	rt.Manager.mu.RUnlock()
	if ts == nil {
		return nil, aistudio.ErrNoEligibleAccount
	}
	return ts.syncAccountModels(ctx, accountID)
}

// CachedModels returns the currently cached models across accounts.
func (rt *PluginRuntime) CachedModels() []aistudio.Model {
	rt.Manager.mu.RLock()
	ts, _ := rt.Manager.current.service.(*trackedService)
	rt.Manager.mu.RUnlock()
	if ts == nil {
		return nil
	}
	return ts.catalog.CachedModels()
}

// RecentLogs returns recent runtime logs.
func (rt *PluginRuntime) RecentLogs() []api.AdminLog {
	if rt == nil || rt.Manager == nil || rt.Manager.requests == nil {
		return nil
	}
	rt.Manager.requests.mu.Lock()
	defer rt.Manager.requests.mu.Unlock()
	return append([]api.AdminLog(nil), rt.Manager.requests.logs...)
}
