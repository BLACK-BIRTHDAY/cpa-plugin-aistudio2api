package aistudio

import (
	"context"
	"errors"
	"time"
)

// NativeLoginDriver 为兼容管理接口保留的占位实现（插件模式仅支持 Cookie / StorageState 导入）
type NativeLoginDriver struct {
	camoufox string
	timeout  time.Duration
}

var _ IsolatedLoginDriver = (*NativeLoginDriver)(nil)

func NewNativeLoginDriver(camoufoxPath string, timeout time.Duration) (*NativeLoginDriver, error) {
	return &NativeLoginDriver{camoufox: camoufoxPath, timeout: timeout}, nil
}

func (driver *NativeLoginDriver) Login(context.Context, IsolatedLoginRequest) (IsolatedLoginResult, error) {
	return IsolatedLoginResult{}, errors.New("插件模式不支持外部浏览器登录，请粘贴 Cookie 或 StorageState 导入账户")
}
