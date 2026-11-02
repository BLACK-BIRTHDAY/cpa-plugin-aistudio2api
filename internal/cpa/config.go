package cpa

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Mag1cFall/AIStudio2API/internal/config"
	"gopkg.in/yaml.v3"
)

const (
	PluginID      = "aistudio2api"
	PluginVersion = "1.0.1"
	ProviderID    = "aistudio2api"
)

// PluginConfig defines the YAML/JSON configuration under plugins.configs.aistudio2api in CLIProxyAPI's config.yaml.
type PluginConfig struct {
	DataDir               string   `json:"data_dir" yaml:"data_dir"`
	AccountsDir           string   `json:"accounts_dir" yaml:"accounts_dir"`
	Proxy                 string   `json:"proxy" yaml:"proxy"`
	WAABackend            string   `json:"waa_backend" yaml:"waa_backend"`
	WarmWorkers           *int     `json:"warm_workers" yaml:"warm_workers"`
	MaxActiveWorkers      *int     `json:"max_active_workers" yaml:"max_active_workers"`
	WarmStartupConc       *int     `json:"warm_startup_concurrency" yaml:"warm_startup_concurrency"`
	PerAccountConcurrency *int     `json:"per_account_concurrency" yaml:"per_account_concurrency"`
	UpstreamChannels      []string `json:"upstream_channels" yaml:"upstream_channels"`
	ModelChannels         string   `json:"model_channels" yaml:"model_channels"`
	ModelAliases          string   `json:"model_aliases" yaml:"model_aliases"`
	BuildNativeNonstream  *bool    `json:"build_native_nonstream" yaml:"build_native_nonstream"`
	NonstreamKeepalive    *bool    `json:"nonstream_keepalive" yaml:"nonstream_keepalive"`
	FakeStream            *bool    `json:"fake_stream" yaml:"fake_stream"`
	TemporaryChat         *bool    `json:"temporary_chat" yaml:"temporary_chat"`
	YouTubeLinks          *bool    `json:"youtube_links" yaml:"youtube_links"`
	FirstEventHedge       string   `json:"first_event_hedge" yaml:"first_event_hedge"`
	SearchModelAliases    *bool    `json:"search_model_aliases" yaml:"search_model_aliases"`
	GoogleSearch          *bool    `json:"google_search" yaml:"google_search"`
	ContinuationRewrite   *bool    `json:"continuation_rewrite" yaml:"continuation_rewrite"`
	ContinuationPrompt    string   `json:"continuation_prompt" yaml:"continuation_prompt"`
	AutoStart             *bool    `json:"auto_start" yaml:"auto_start"`
	PinExecutorAccount    *bool    `json:"pin_executor_account" yaml:"pin_executor_account"`
	RouteUnprefixedModels *bool    `json:"route_unprefixed_models" yaml:"route_unprefixed_models"`
	SyncAuthFiles         *bool    `json:"sync_auth_files" yaml:"sync_auth_files"`
	ModelPrefix           string        `json:"model_prefix" yaml:"model_prefix"`
	Options               *PluginConfig `json:"options,omitempty" yaml:"options,omitempty"`
}

func ParsePluginConfig(raw []byte) (PluginConfig, config.Config, error) {
	var pcfg PluginConfig
	trimmed := strings.TrimSpace(string(raw))
	if trimmed != "" && trimmed != "null" && trimmed != "{}" {
		if err := yaml.Unmarshal(raw, &pcfg); err != nil {
			_ = json.Unmarshal(raw, &pcfg)
		}
		if pcfg.Options != nil {
			opt := *pcfg.Options
			if pcfg.DataDir == "" {
				pcfg.DataDir = opt.DataDir
			}
			if pcfg.AccountsDir == "" {
				pcfg.AccountsDir = opt.AccountsDir
			}
			if pcfg.Proxy == "" {
				pcfg.Proxy = opt.Proxy
			}
			if pcfg.WAABackend == "" {
				pcfg.WAABackend = opt.WAABackend
			}
			if pcfg.WarmWorkers == nil {
				pcfg.WarmWorkers = opt.WarmWorkers
			}
			if pcfg.MaxActiveWorkers == nil {
				pcfg.MaxActiveWorkers = opt.MaxActiveWorkers
			}
			if pcfg.WarmStartupConc == nil {
				pcfg.WarmStartupConc = opt.WarmStartupConc
			}
			if pcfg.PerAccountConcurrency == nil {
				pcfg.PerAccountConcurrency = opt.PerAccountConcurrency
			}
			if len(pcfg.UpstreamChannels) == 0 {
				pcfg.UpstreamChannels = opt.UpstreamChannels
			}
			if pcfg.ModelPrefix == "" {
				pcfg.ModelPrefix = opt.ModelPrefix
			}
			if pcfg.RouteUnprefixedModels == nil {
				pcfg.RouteUnprefixedModels = opt.RouteUnprefixedModels
			}
			if pcfg.SyncAuthFiles == nil {
				pcfg.SyncAuthFiles = opt.SyncAuthFiles
			}
		}
	}

	dataDir := strings.TrimSpace(pcfg.DataDir)
	if dataDir == "" {
		dataDir = strings.TrimSpace(os.Getenv("AISTUDIO2API_DATA_DIR"))
	}
	if dataDir == "" {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			dataDir = filepath.Join(home, ".aistudio2api")
		} else {
			dataDir = filepath.Join(".", "data", "aistudio2api")
		}
	}
	if strings.HasPrefix(dataDir, "~/") {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			dataDir = filepath.Join(home, dataDir[2:])
		}
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		legacyInAuthDir := filepath.Join(home, ".cli-proxy-api", "aistudio2api")
		if filepath.Clean(dataDir) == filepath.Clean(legacyInAuthDir) {
			dataDir = filepath.Join(home, ".aistudio2api")
		}
	}
	accountsDir := strings.TrimSpace(pcfg.AccountsDir)
	if accountsDir == "" {
		accountsDir = strings.TrimSpace(os.Getenv("AISTUDIO_AUTH_STATES"))
	}
	if accountsDir == "" {
		accountsDir = filepath.Join(dataDir, "accounts")
	}
	if strings.HasPrefix(accountsDir, "~/") {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			accountsDir = filepath.Join(home, accountsDir[2:])
		}
	}
	_ = os.MkdirAll(accountsDir, 0o700)

	waaBackend := strings.ToLower(strings.TrimSpace(pcfg.WAABackend))
	if waaBackend == "" {
		waaBackend = strings.ToLower(strings.TrimSpace(os.Getenv("WAA_BACKEND")))
	}
	if waaBackend != config.WAABackendCamoufox && waaBackend != config.WAABackendSpiderMonkey {
		waaBackend = config.WAABackendSpiderMonkey
	}

	warmWorkers := 2
	if pcfg.WarmWorkers != nil && *pcfg.WarmWorkers >= 1 {
		warmWorkers = *pcfg.WarmWorkers
	}
	maxActive := 5
	if pcfg.MaxActiveWorkers != nil && *pcfg.MaxActiveWorkers >= 1 {
		maxActive = *pcfg.MaxActiveWorkers
	}
	if maxActive < warmWorkers {
		maxActive = warmWorkers
	}
	warmStartup := 2
	if pcfg.WarmStartupConc != nil && *pcfg.WarmStartupConc >= 1 {
		warmStartup = *pcfg.WarmStartupConc
	}
	perAccountConc := 2
	if pcfg.PerAccountConcurrency != nil && *pcfg.PerAccountConcurrency >= 1 {
		perAccountConc = *pcfg.PerAccountConcurrency
	}

	channels := pcfg.UpstreamChannels
	if len(channels) == 0 {
		channels = []string{"playground", "build"}
	}

	hedge := strings.TrimSpace(pcfg.FirstEventHedge)
	if hedge == "" {
		hedge = config.FirstEventHedgeAuto
	}

	cfg := config.Default()
	cfg.AuthStates = accountsDir
	cfg.Proxy = strings.TrimSpace(pcfg.Proxy)
	cfg.WAABackend = waaBackend
	cfg.InitTimeout = 2 * time.Minute
	cfg.RequestTimeout = 10 * time.Minute
	cfg.WarmWorkerLimit = warmWorkers
	cfg.MaxActiveWorkers = maxActive
	cfg.WarmStartupConcurrency = warmStartup
	cfg.PerAccountConcurrency = perAccountConc
	cfg.UpstreamChannels = channels
	cfg.ModelChannels = strings.TrimSpace(pcfg.ModelChannels)
	cfg.ModelMapping = strings.TrimSpace(pcfg.ModelAliases)
	cfg.BuildNativeNonstream = boolOrDefault(pcfg.BuildNativeNonstream, true)
	cfg.NonstreamKeepalive = boolOrDefault(pcfg.NonstreamKeepalive, false)
	cfg.FakeStream = boolOrDefault(pcfg.FakeStream, false)
	cfg.TemporaryChat = boolOrDefault(pcfg.TemporaryChat, true)
	cfg.YouTubeLinks = boolOrDefault(pcfg.YouTubeLinks, true)
	cfg.FirstEventHedge = hedge
	cfg.SearchModelAliases = boolOrDefault(pcfg.SearchModelAliases, true)
	cfg.GoogleSearch = boolOrDefault(pcfg.GoogleSearch, false)
	cfg.ContinuationRewrite = boolOrDefault(pcfg.ContinuationRewrite, true)
	if strings.TrimSpace(pcfg.ContinuationPrompt) != "" {
		cfg.ContinuationPrompt = strings.TrimSpace(pcfg.ContinuationPrompt)
	}
	cfg.AutoStart = boolOrDefault(pcfg.AutoStart, true)
	cfg.MediaOutput = config.MediaOutputInline

	if pcfg.ModelPrefix == "" {
		pcfg.ModelPrefix = "aistudio/"
	}
	return pcfg, cfg, nil
}

func boolOrDefault(ptr *bool, def bool) bool {
	if ptr == nil {
		return def
	}
	return *ptr
}
