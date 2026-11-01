//go:build !windows && !darwin

package chromeauth
import (
	"fmt"

	"github.com/Mag1cFall/AIStudio2API/internal/aistudio"
)

// DefaultChromeRoot 返回稳定版 Chrome User Data 目录
func DefaultChromeRoot() (string, error) {
	return "", fmt.Errorf("Chrome OAuth 导入仅支持 Windows 与 macOS")
}

func ensurePlatformImport() error {
	return fmt.Errorf("Chrome OAuth 导入仅支持 Windows 与 macOS")
}

// Discover 只读列出本机 Chrome Google 账号
func Discover(string) ([]Account, error) {
	return nil, fmt.Errorf("Chrome OAuth 导入仅支持 Windows 与 macOS")
}

func readTokenService(string, string, string) (string, []byte, []byte, error) {
	return "", nil, nil, fmt.Errorf("Chrome OAuth 导入仅支持 Windows 与 macOS")
}

func readProfileGoogleCookies(string, string, []byte) ([]aistudio.StateCookie, error) {
	return nil, fmt.Errorf("Chrome Cookie 导入仅支持 macOS")
}
