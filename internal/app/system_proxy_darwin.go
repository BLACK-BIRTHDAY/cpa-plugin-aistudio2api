//go:build darwin

package app

// osProxy 在插件模式下不调用外部系统命令，由宿主配置或环境变量提供代理
func osProxy() string {
	return ""
}
