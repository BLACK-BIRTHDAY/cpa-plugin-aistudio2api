package camoufoxnative

import (
	"errors"
	"io"
	"net/http"
	"time"
)

// ErrAuthenticationRequired 表示账户需要恢复登录
var ErrAuthenticationRequired = errors.New("隔离登录态失效")

// ErrRegionNotSupported 表示 AI Studio 不支持当前出口地区或账户
var ErrRegionNotSupported = errors.New("AI Studio 不支持当前出口地区或账户，为该账户或全局代理换用支持地区的出口")

// ErrProxyUnreachable 表示配置的代理无法连接
var ErrProxyUnreachable = errors.New("代理无法连接")

// AvailableRegionsURL 是官网在地区或账户不受支持时跳转的页面
const AvailableRegionsURL = "https://aistudio.google.com/docs/available-regions"

// StartupStage 表示 WAA runtime 的启动阶段
type StartupStage string

const (
	StartupPreparingBrowser StartupStage = "preparing_browser"
	StartupLaunchingBrowser StartupStage = "launching_browser"
	StartupConnectingBiDi   StartupStage = "connecting_bidi"
	StartupLoadingAIStudio  StartupStage = "loading_ai_studio"
	StartupLocatingWAA      StartupStage = "locating_waa"
	StartupBootstrappingWAA StartupStage = "bootstrapping_waa"
)

// Options 定义单个 AI Studio 账户的 WAA runtime 配置
type Options struct {
	ExecutablePath   string
	StorageStatePath string
	Model            string
	Locale           string
	Timezone         string
	Proxy            string
	ProxyBypass      string
	Headless         bool
	AuthUser         int
	CacheDirectory   string
	ReadyTimeout     time.Duration
	Log              io.Writer
	StartupProgress  func(StartupStage)
}

// PageCatalog 是会话页面载入时 GetLoggingContext、GetAiStudioBenefitTier 与 ListModels 的响应正文
type PageCatalog struct {
	Logging []byte
	Tier    []byte
	Models  []byte
}

// State 返回 WAA runtime 的当前页面与 bootstrap 结果
type State struct {
	PID         int
	PageURL     string
	UserAgent   string
	SnapshotKey string
	Headers     http.Header
}

// Download 保留供管理状态 JSON 结构体序列化兼容（插件内始终使用进程内 SpiderMonkey，不下载外部浏览器）
type Download struct {
	Stage      string `json:"stage"`
	Downloaded int64  `json:"downloaded"`
	Total      int64  `json:"total"`
}

// LiveView 保留供管理接口签名兼容（插件模式仅支持粘贴 Cookie / StorageState 导入）
type LiveView struct{}

func NewLiveView() *LiveView {
	return &LiveView{}
}

func (v *LiveView) Close() {}

func HasDisplay() bool {
	return false
}
