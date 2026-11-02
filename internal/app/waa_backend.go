package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/Mag1cFall/AIStudio2API/internal/aistudio"
	"github.com/Mag1cFall/AIStudio2API/internal/camoufoxnative"
	"github.com/Mag1cFall/AIStudio2API/internal/config"
)

// prepareWAABackend 初始化进程内 SpiderMonkey WAA 后端（插件模式不使用也不下载外部浏览器）
func prepareWAABackend(_ context.Context, _ config.Config, requests *requestRegistry, accounts int, _ string) (string, aistudio.IsolatedLoginDriver, error) {
	requests.log("service", "INFO", fmt.Sprintf("运行时装配 | 2/3 | WAA 后端=spidermonkey | 账户=%d", accounts))
	return "", &disabledLoginDriver{}, nil
}

// newWAAWorker 启动账户的进程内 SpiderMonkey Worker
func newWAAWorker(ctx context.Context, accountID string, options camoufoxnative.Options) (*aistudio.NativeWorker, error) {
	return aistudio.NewSpiderMonkeyWorker(ctx, accountID, options)
}

type disabledLoginDriver struct{}

func (*disabledLoginDriver) Login(context.Context, aistudio.IsolatedLoginRequest) (aistudio.IsolatedLoginResult, error) {
	return aistudio.IsolatedLoginResult{}, errors.New("插件模式不支持启动外部浏览器登录，请粘贴 Cookie 或 StorageState 导入账户")
}
