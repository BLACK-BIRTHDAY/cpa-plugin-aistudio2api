package app

import (
	"context"
	"fmt"
	"time"

	"github.com/Mag1cFall/AIStudio2API/internal/aistudio"
	"github.com/Mag1cFall/AIStudio2API/internal/camoufoxnative"
	"github.com/Mag1cFall/AIStudio2API/internal/config"
)

// prepareWAABackend 初始化进程内 SpiderMonkey WAA 后端与本机 Chrome 无痕登录驱动
func prepareWAABackend(_ context.Context, _ config.Config, requests *requestRegistry, accounts int, _ string) (string, aistudio.IsolatedLoginDriver, error) {
	requests.log("service", "INFO", fmt.Sprintf("运行时装配 | 2/3 | WAA 后端=spidermonkey | 账户=%d", accounts))
	driver, err := aistudio.NewNativeLoginDriver("", 5*time.Minute)
	if err != nil {
		return "", nil, err
	}
	return "", driver, nil
}

// newWAAWorker 启动账户的进程内 SpiderMonkey Worker
func newWAAWorker(ctx context.Context, accountID string, options camoufoxnative.Options) (*aistudio.NativeWorker, error) {
	return aistudio.NewSpiderMonkeyWorker(ctx, accountID, options)
}
