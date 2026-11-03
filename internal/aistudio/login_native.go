package aistudio

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

const (
	chromeIncognitoStartURL = "https://aistudio.google.com/prompts/new_chat"
	aiStudioBasePrefix      = "https://aistudio.google.com/"
	aiStudioOriginURL       = "https://aistudio.google.com"
)

var wizEmailPattern = regexp.MustCompile(`"oPEP7c":("(?:[^"\\]|\\.)*")`)

const pageInspectExpression = `(() => {
  const ready = document.readyState === 'complete' || document.readyState === 'interactive';
  let email = '';
  try {
    const wiz = window.WIZ_global_data && window.WIZ_global_data.oPEP7c;
    if (typeof wiz === 'string' && wiz.includes('@')) email = wiz.trim();
  } catch {}
  if (!email) {
    const values = [document.body?.innerText || ''];
    for (const el of document.querySelectorAll('[aria-label],[data-email]')) {
      values.push(el.getAttribute('aria-label') || '');
      values.push(el.getAttribute('data-email') || '');
    }
    email = values.join('\n').match(/[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}/i)?.[0] || '';
  }
  let ls = [];
  try {
    ls = Object.keys(localStorage).sort().map(name => ({name, value: localStorage.getItem(name) || ''}));
  } catch {}
  const hasPrompt = Boolean(document.querySelector('ms-prompt-box textarea, textarea'));
  const hasDialog = Boolean(document.querySelector('mat-dialog-container, ms-onboarding, [role="dialog"]'));
  return JSON.stringify({
    ready,
    email: (email || '').toLowerCase().trim(),
    hasPrompt,
    hasDialog,
    ls,
    href: location.href || ''
  });
})()`

// NativeLoginDriver 通过启动本机 Chrome/Edge/Brave 的独立无痕窗口（--incognito + 临时 --user-data-dir）完成 AI Studio 登录并自动拉取凭证
type NativeLoginDriver struct {
	chromePath string
	timeout    time.Duration
}

var _ IsolatedLoginDriver = (*NativeLoginDriver)(nil)

func NewNativeLoginDriver(browserPath string, timeout time.Duration) (*NativeLoginDriver, error) {
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	return &NativeLoginDriver{chromePath: strings.TrimSpace(browserPath), timeout: timeout}, nil
}

// FindLocalChromeBinary 查找本机已安装的 Chrome / Chromium / Edge / Brave 可执行文件
func FindLocalChromeBinary() (string, error) {
	for _, envKey := range []string{"AISTUDIO2API_CHROME_PATH", "CHROME_PATH", "BROWSER_PATH"} {
		if v := strings.TrimSpace(os.Getenv(envKey)); v != "" {
			if info, err := os.Stat(v); err == nil && !info.IsDir() {
				return v, nil
			}
		}
	}

	var candidates []string
	home, _ := os.UserHomeDir()

	switch runtime.GOOS {
	case "darwin":
		appRelPaths := []string{
			"Google Chrome.app/Contents/MacOS/Google Chrome",
			"Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary",
			"Chromium.app/Contents/MacOS/Chromium",
			"Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"Brave Browser.app/Contents/MacOS/Brave Browser",
		}
		for _, rel := range appRelPaths {
			candidates = append(candidates, filepath.Join("/Applications", rel))
			if home != "" {
				candidates = append(candidates, filepath.Join(home, "Applications", rel))
			}
		}
	case "windows":
		roots := []string{
			os.Getenv("ProgramFiles"),
			os.Getenv("ProgramFiles(x86)"),
			os.Getenv("LocalAppData"),
		}
		relPaths := []string{
			`Google\Chrome\Application\chrome.exe`,
			`Chromium\Application\chrome.exe`,
			`Microsoft\Edge\Application\msedge.exe`,
			`BraveSoftware\Brave-Browser\Application\brave.exe`,
		}
		for _, root := range roots {
			if strings.TrimSpace(root) == "" {
				continue
			}
			for _, rel := range relPaths {
				candidates = append(candidates, filepath.Join(root, rel))
			}
		}
	default:
		candidates = append(candidates,
			"/usr/bin/google-chrome",
			"/usr/bin/google-chrome-stable",
			"/usr/bin/chromium",
			"/usr/bin/chromium-browser",
			"/snap/bin/chromium",
			"/usr/bin/microsoft-edge",
			"/usr/bin/brave-browser",
		)
	}

	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c, nil
		}
	}

	for _, binName := range []string{
		"google-chrome",
		"google-chrome-stable",
		"chromium",
		"chromium-browser",
		"chrome",
		"msedge",
		"microsoft-edge",
		"brave-browser",
	} {
		if path, err := exec.LookPath(binName); err == nil && path != "" {
			return path, nil
		}
	}

	return "", errors.New("未找到本机 Chrome / Edge / Brave 浏览器，请安装 Chrome 或设置环境变量 AISTUDIO2API_CHROME_PATH")
}

// Login 启动本机 Chrome 无痕窗口引导用户登录 Google AI Studio，并在登录完成后自动拉取 Cookie 与 StorageState
func (driver *NativeLoginDriver) Login(ctx context.Context, request IsolatedLoginRequest) (IsolatedLoginResult, error) {
	if runtime.GOOS == "linux" && os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		return IsolatedLoginResult{}, errors.New("当前服务器无图形桌面环境（缺少 DISPLAY / WAYLAND_DISPLAY），无法弹出本机 Chrome 窗口，请粘贴 Cookie 导入")
	}

	chromeBin, err := FindLocalChromeBinary()
	if err != nil {
		return IsolatedLoginResult{}, err
	}

	timeout := driver.timeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	loginCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	userDataDir, err := os.MkdirTemp("", "aistudio2api-chrome-incognito-*")
	if err != nil {
		return IsolatedLoginResult{}, fmt.Errorf("创建临时 Chrome 无痕目录: %w", err)
	}
	defer os.RemoveAll(userDataDir)

	args := []string{
		"--user-data-dir=" + userDataDir,
		"--remote-debugging-port=0",
		"--disable-blink-features=AutomationControlled",
		"--exclude-switches=enable-automation",
		"--no-first-run",
		"--no-default-browser-check",
		"--new-window",
	}
	if proxy := strings.TrimSpace(request.Proxy); proxy != "" {
		args = append(args, "--proxy-server="+proxy)
	}
	if locale := strings.TrimSpace(request.Locale); locale != "" {
		args = append(args, "--lang="+locale)
	}
	args = append(args, chromeIncognitoStartURL)

	cmd := exec.CommandContext(loginCtx, chromeBin, args...)
	if err := cmd.Start(); err != nil {
		return IsolatedLoginResult{}, fmt.Errorf("启动本机 Chrome 无痕窗口失败: %w", err)
	}

	var exited atomic.Bool
	waitDone := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		exited.Store(true)
		close(waitDone)
	}()

	closeBrowser := func(pageWS string) error {
		if pageWS != "" {
			closeCtx, closeCancel := context.WithTimeout(context.Background(), 2*time.Second)
			_ = cdpCloseBrowser(closeCtx, pageWS)
			closeCancel()
		}
		if !exited.Load() && cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		select {
		case <-waitDone:
		case <-time.After(3 * time.Second):
		}
		return nil
	}
	defer func() {
		_ = closeBrowser("")
	}()

	debugPort, err := waitForDevToolsActivePort(loginCtx, userDataDir, &exited)
	if err != nil {
		return IsolatedLoginResult{}, err
	}

	// 阶段 1：轮询 /json/list，在用户于 accounts.google.com 输入账号密码期间不挂载任何 CDP WebSocket
	var activePageWS string
	var verifiedEmail string
	var capturedState StorageState
	var firstAIStudioSeen time.Time

	httpClient := &http.Client{Timeout: 3 * time.Second}
	for {
		if err := loginCtx.Err(); err != nil {
			return IsolatedLoginResult{}, fmt.Errorf("等待 Chrome 无痕登录超时或已取消: %w", err)
		}
		if exited.Load() {
			return IsolatedLoginResult{}, errors.New("Chrome 无痕窗口已关闭，登录已取消")
		}

		targets, err := listCDPTargets(loginCtx, httpClient, debugPort)
		if err != nil {
			if exited.Load() {
				return IsolatedLoginResult{}, errors.New("Chrome 无痕窗口已关闭，登录已取消")
			}
			select {
			case <-loginCtx.Done():
				return IsolatedLoginResult{}, loginCtx.Err()
			case <-time.After(400 * time.Millisecond):
				continue
			}
		}

		var aiStudioTarget *cdpPageTarget
		for i := range targets {
			t := &targets[i]
			if t.Type == "page" && t.WebSocketDebuggerURL != "" {
				ensureStealthOnTarget(loginCtx, t.WebSocketDebuggerURL)
				if strings.HasPrefix(t.URL, aiStudioBasePrefix) {
					aiStudioTarget = t
				}
			}
		}
		if aiStudioTarget == nil {
			firstAIStudioSeen = time.Time{}
			select {
			case <-loginCtx.Done():
				return IsolatedLoginResult{}, loginCtx.Err()
			case <-time.After(450 * time.Millisecond):
				continue
			}
		}
		if firstAIStudioSeen.IsZero() {
			firstAIStudioSeen = time.Now()
		}
		activePageWS = aiStudioTarget.WebSocketDebuggerURL
		// 阶段 2：用户已跳转回 https://aistudio.google.com/，读取无痕会话的 Cookie、localStorage 与登录邮箱
		state, email, ready, err := inspectAIStudioPage(loginCtx, activePageWS)
		if err == nil && len(state.Cookies) > 0 {
			if _, signErr := NewSigner().Authorization(state); signErr == nil {
				if email == "" && (ready || time.Since(firstAIStudioSeen) > 2*time.Second) {
					email, _ = resolveEmailViaHTTP(loginCtx, &state, request.Proxy)
				}
				if email != "" && strings.Contains(email, "@") {
					capturedState = state
					verifiedEmail = strings.ToLower(strings.TrimSpace(email))
					break
				}
			}
		}

		select {
		case <-loginCtx.Done():
			return IsolatedLoginResult{}, loginCtx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}

	closeErr := closeBrowser(activePageWS)
	capturedState.SetAuthExtension(AuthExtension{
		Source: AuthSource{
			Browser: "chrome-incognito",
			Email:   verifiedEmail,
		},
	})
	var closeErrText string
	if closeErr != nil {
		closeErrText = closeErr.Error()
	}
	return IsolatedLoginResult{
		StorageState: capturedState,
		Email:        verifiedEmail,
		VerifiedAt:   time.Now().UTC(),
		CloseError:   closeErrText,
	}, nil
}

type cdpPageTarget struct {
	ID                   string `json:"id"`
	Type                 string `json:"type"`
	Title                string `json:"title"`
	URL                  string `json:"url"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

func waitForDevToolsActivePort(ctx context.Context, userDataDir string, exited *atomic.Bool) (int, error) {
	portFile := filepath.Join(userDataDir, "DevToolsActivePort")
	for {
		if err := ctx.Err(); err != nil {
			return 0, fmt.Errorf("等待 Chrome 调试端口超时: %w", err)
		}
		if exited.Load() {
			return 0, errors.New("Chrome 无痕窗口启动后立即退出（请确认本机 Chrome 可正常打开）")
		}
		data, err := os.ReadFile(portFile)
		if err == nil {
			line, _, _ := strings.Cut(strings.TrimSpace(string(data)), "\n")
			if port, convErr := strconv.Atoi(strings.TrimSpace(line)); convErr == nil && port > 0 && port <= 65535 {
				return port, nil
			}
		}
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(150 * time.Millisecond):
		}
	}
}

func listCDPTargets(ctx context.Context, client *http.Client, port int) ([]cdpPageTarget, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/json/list", port), nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	var targets []cdpPageTarget
	if err := json.Unmarshal(body, &targets); err != nil {
		return nil, err
	}
	return targets, nil
}

type cdpPageInspection struct {
	Ready     bool               `json:"ready"`
	Email     string             `json:"email"`
	HasPrompt bool               `json:"hasPrompt"`
	HasDialog bool               `json:"hasDialog"`
	LS        []StorageItem `json:"ls"`
	Href      string             `json:"href"`
}

func inspectAIStudioPage(ctx context.Context, pageWS string) (StorageState, string, bool, error) {
	callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	dialer := websocket.Dialer{HandshakeTimeout: 3 * time.Second}
	conn, _, err := dialer.DialContext(callCtx, pageWS, nil)
	if err != nil {
		return StorageState{}, "", false, err
	}
	defer conn.Close()
	stop := context.AfterFunc(callCtx, func() { _ = conn.Close() })
	defer stop()

	var rawCookies struct {
		Cookies []struct {
			Name         string  `json:"name"`
			Value        string  `json:"value"`
			Domain       string  `json:"domain"`
			Path         string  `json:"path"`
			Expires      float64 `json:"expires"`
			HTTPOnly     bool    `json:"httpOnly"`
			Secure       bool    `json:"secure"`
			SameSite     string  `json:"sameSite"`
			PartitionKey any     `json:"partitionKey"`
		} `json:"cookies"`
	}
	if err := cdpCall(callCtx, conn, 1, "Network.getAllCookies", map[string]any{}, &rawCookies); err != nil {
		return StorageState{}, "", false, err
	}

	cookies := make([]StateCookie, 0, len(rawCookies.Cookies))
	for _, item := range rawCookies.Cookies {
		if item.Name == "" || item.Domain == "" {
			continue
		}
		path := item.Path
		if path == "" {
			path = "/"
		}
		expires := item.Expires
		if expires <= 0 {
			expires = -1
		}
		sameSite := strings.ToLower(strings.TrimSpace(item.SameSite))
		switch sameSite {
		case "none":
			sameSite = "None"
		case "lax":
			sameSite = "Lax"
		case "strict":
			sameSite = "Strict"
		default:
			sameSite = ""
		}
		partKey, _ := item.PartitionKey.(string)
		cookies = append(cookies, StateCookie{
			Name:         item.Name,
			Value:        item.Value,
			Domain:       item.Domain,
			Path:         path,
			Expires:      expires,
			HTTPOnly:     item.HTTPOnly,
			Secure:       item.Secure,
			SameSite:     sameSite,
			PartitionKey: partKey,
		})
	}
	sort.SliceStable(cookies, func(i, j int) bool {
		if cookies[i].Domain != cookies[j].Domain {
			return cookies[i].Domain < cookies[j].Domain
		}
		if cookies[i].Name != cookies[j].Name {
			return cookies[i].Name < cookies[j].Name
		}
		return cookies[i].Path < cookies[j].Path
	})

	var evalResp struct {
		Result struct {
			Type  string `json:"type"`
			Value string `json:"value"`
		} `json:"result"`
	}
	var info cdpPageInspection
	if err := cdpCall(callCtx, conn, 2, "Runtime.evaluate", map[string]any{
		"expression":    pageInspectExpression,
		"returnByValue": true,
	}, &evalResp); err == nil && evalResp.Result.Value != "" {
		_ = json.Unmarshal([]byte(evalResp.Result.Value), &info)
	}

	var origins []StorageOrigin
	if len(info.LS) > 0 {
		origins = []StorageOrigin{{
			Origin:       aiStudioOriginURL,
			LocalStorage: info.LS,
		}}
	}
	state := StorageState{
		Cookies: cookies,
		Origins: origins,
	}
	pageReady := info.HasPrompt || info.HasDialog || info.Ready
	return state, strings.ToLower(strings.TrimSpace(info.Email)), pageReady, nil
}

func resolveEmailViaHTTP(ctx context.Context, state *StorageState, proxy string) (string, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	client, err := NewProxyHTTPClient(proxy)
	if err != nil {
		return "", err
	}
	defer client.CloseIdleConnections()
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	cookieHeader, err := state.CookieHeader(aiStudioOriginURL+"/prompts/new_chat", time.Now())
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, aiStudioOriginURL+"/prompts/new_chat", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Cookie", cookieHeader)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", err
	}
	match := wizEmailPattern.FindSubmatch(body)
	if len(match) < 2 {
		return "", errors.New("email not found in WIZ_global_data")
	}
	var email string
	if err := json.Unmarshal(match[1], &email); err != nil {
		return "", err
	}
	return strings.ToLower(strings.TrimSpace(email)), nil
}

func cdpCloseBrowser(ctx context.Context, pageWS string) error {
	dialer := websocket.Dialer{HandshakeTimeout: 2 * time.Second}
	conn, _, err := dialer.DialContext(ctx, pageWS, nil)
	if err != nil {
		return err
	}
	defer conn.Close()
	var out map[string]any
	return cdpCall(ctx, conn, 99, "Browser.close", map[string]any{}, &out)
}

var stealthInjectedTargets sync.Map

const stealthInitScript = `(() => {
  try {
    Object.defineProperty(navigator, 'webdriver', { get: () => undefined });
  } catch {}
  try {
    delete Object.getPrototypeOf(navigator).webdriver;
  } catch {}
  try {
    for (const k of Object.keys(window)) {
      if (k.startsWith('cdc_') || k.startsWith('$cdc_')) {
        try { delete window[k]; } catch {}
      }
    }
  } catch {}
})();`

func ensureStealthOnTarget(ctx context.Context, pageWS string) {
	if pageWS == "" {
		return
	}
	if _, loaded := stealthInjectedTargets.LoadOrStore(pageWS, struct{}{}); loaded {
		return
	}
	callCtx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()
	dialer := websocket.Dialer{HandshakeTimeout: 1 * time.Second}
	conn, _, err := dialer.DialContext(callCtx, pageWS, nil)
	if err != nil {
		stealthInjectedTargets.Delete(pageWS)
		return
	}
	defer conn.Close()
	var out map[string]any
	_ = cdpCall(callCtx, conn, 1, "Page.enable", map[string]any{}, &out)
	_ = cdpCall(callCtx, conn, 2, "Page.addScriptToEvaluateOnNewDocument", map[string]any{
		"source": stealthInitScript,
	}, &out)
	_ = cdpCall(callCtx, conn, 3, "Runtime.evaluate", map[string]any{
		"expression": stealthInitScript,
	}, &out)
}

func cdpCall(ctx context.Context, conn *websocket.Conn, id int, method string, params any, result any) error {
	msg := map[string]any{
		"id":     id,
		"method": method,
	}
	if params != nil {
		msg["params"] = params
	}
	if err := conn.WriteJSON(msg); err != nil {
		return err
	}
	for {
		var resp struct {
			ID     int             `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := conn.ReadJSON(&resp); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
		if resp.ID != id {
			continue
		}
		if resp.Error != nil {
			return errors.New(resp.Error.Message)
		}
		if result != nil && len(resp.Result) > 0 {
			return json.Unmarshal(resp.Result, result)
		}
		return nil
	}
}
