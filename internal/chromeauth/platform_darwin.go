//go:build darwin

package chromeauth

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/Mag1cFall/AIStudio2API/internal/aistudio"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/pbkdf2"
)

// DefaultChromeRoot 返回 macOS 稳定版 Chrome User Data 目录
func canReadChromeDir(root string) bool {
	if f, err := os.Open(filepath.Join(root, "Local State")); err != nil {
		return false
	} else {
		f.Close()
	}
	if f, err := os.Open(filepath.Join(root, "Default", "Preferences")); err != nil {
		return false
	} else {
		f.Close()
	}
	if f, err := os.Open(filepath.Join(root, "Default", "Web Data")); err != nil {
		if f2, err2 := os.Open(filepath.Join(root, "Default", "Cookies")); err2 != nil {
			return false
		} else {
			f2.Close()
		}
	} else {
		f.Close()
	}
	return true
}

func DefaultChromeRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return "", fmt.Errorf("无法获取 macOS 用户主目录")
	}
	primary := filepath.Join(home, "Library", "Application Support", "Google", "Chrome")
	if canReadChromeDir(primary) {
		return primary, nil
	}
	for _, cacheDir := range []string{
		filepath.Join(home, ".aistudio2api", "chrome-cache"),
		filepath.Join(home, ".cli-proxy-api", "aistudio2api", "chrome-cache"),
	} {
		if canReadChromeDir(cacheDir) {
			return cacheDir, nil
		}
	}
	return primary, nil
}

func ensurePlatformImport() error {
	return nil
}

// retrieveV20Key 在 macOS 上优先读取已缓存的 16 字节 v10 密钥，否则从系统钥匙串读取 "Chrome Safe Storage" 并经 PBKDF2-HMAC-SHA1 派生
func retrieveV20Key(chromeRoot string) ([]byte, error) {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		for _, candidate := range []string{
			filepath.Join(chromeRoot, "v10.key"),
			filepath.Join(home, ".aistudio2api", "chrome-cache", "v10.key"),
			filepath.Join(home, ".cli-proxy-api", "aistudio2api", "chrome-cache", "v10.key"),
		} {
			if raw, err := os.ReadFile(candidate); err == nil && len(raw) == 16 {
				return raw, nil
			}
		}
	}
	cmd := exec.Command("security", "find-generic-password", "-w", "-s", "Chrome Safe Storage", "-a", "Chrome")
	out, err := cmd.Output()
	if err != nil {
		cmd2 := exec.Command("security", "find-generic-password", "-w", "-s", "Chrome Safe Storage")
		out, err = cmd2.Output()
		if err != nil {
			return nil, fmt.Errorf("从 macOS 钥匙串读取 Chrome Safe Storage 失败 (请运行 make sync-chrome 或在钥匙串访问中授权): %w", err)
		}
	}
	password := strings.TrimRight(string(out), "\r\n")
	if password == "" {
		return nil, fmt.Errorf("macOS 钥匙串 Chrome Safe Storage 密码为空")
	}
	key := pbkdf2.Key([]byte(password), []byte("saltysalt"), 1003, 16, sha1.New)
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		cacheDir := filepath.Join(home, ".aistudio2api", "chrome-cache")
		_ = os.MkdirAll(cacheDir, 0o700)
		_ = os.WriteFile(filepath.Join(cacheDir, "v10.key"), key, 0o600)
	}
	return key, nil
}

// Discover 只读列出本机 macOS Chrome Google 账号
func Discover(chromeRoot string) ([]Account, error) {
	_, _ = retrieveV20Key(chromeRoot)
	data, err := os.ReadFile(filepath.Join(chromeRoot, "Local State"))
	if err != nil {
		return nil, fmt.Errorf("读取 Chrome Local State: %w", err)
	}
	var state struct {
		Variations struct {
			SafeSeedLocale string `json:"safe_seed_locale"`
		} `json:"variations"`
		Profile struct {
			InfoCache map[string]struct {
				Name     string `json:"name"`
				UserName string `json:"user_name"`
			} `json:"info_cache"`
		} `json:"profile"`
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("解析 Chrome Local State: %w", err)
	}
	if state.Profile.InfoCache == nil {
		return nil, fmt.Errorf("Chrome Local State 缺少 profile.info_cache")
	}

	profiles := make([]string, 0, len(state.Profile.InfoCache))
	for profile := range state.Profile.InfoCache {
		profiles = append(profiles, profile)
	}
	sort.Slice(profiles, func(i int, j int) bool {
		return naturalLess(profiles[i], profiles[j])
	})
	accounts := make([]Account, 0, len(profiles))
	for _, profile := range profiles {
		info := state.Profile.InfoCache[profile]
		locale := profileLocale(chromeRoot, profile)
		if locale == "" {
			locale = state.Variations.SafeSeedLocale
		}
		hasCookies := profileHasGoogleCookies(chromeRoot, profile)
		identities, err := profileIdentities(chromeRoot, profile)
		if err != nil || len(identities) == 0 {
			if info.UserName != "" || hasCookies {
				email := info.UserName
				if email == "" {
					email = strings.ToLower(strings.ReplaceAll(profile, " ", "_")) + "@chrome.local"
				}
				accounts = append(accounts, Account{
					ID:          profile,
					Profile:     profile,
					DisplayName: info.Name,
					Email:       email,
					Locale:      locale,
					Importable:  hasCookies,
				})
			}
			continue
		}
		records, recordsErr := readTokenServices(chromeRoot, profile)
		seen := map[string]bool{}
		for _, identity := range identities {
			if identity.Gaia == "" || seen[identity.Gaia] {
				continue
			}
			seen[identity.Gaia] = true
			record, found := records[identity.Gaia]
			hasV10Token := recordsErr == nil && found && strings.HasPrefix(string(record.encryptedToken), "v10")
			accounts = append(accounts, Account{
				ID:          profile + "/" + identity.Gaia,
				GaiaID:      identity.Gaia,
				Profile:     profile,
				DisplayName: info.Name,
				Email:       identity.Email,
				Locale:      locale,
				Importable:  hasV10Token || hasCookies,
			})
		}
	}
	return accounts, nil
}

type profileIdentity struct {
	Gaia  string `json:"gaia"`
	Email string `json:"email"`
}

func profileIdentities(chromeRoot string, profile string) ([]profileIdentity, error) {
	data, err := os.ReadFile(filepath.Join(chromeRoot, profile, "Preferences"))
	if err != nil {
		return nil, fmt.Errorf("读取 %s Preferences: %w", profile, err)
	}
	var preferences struct {
		Accounts []profileIdentity `json:"account_info"`
	}
	if err := json.Unmarshal(data, &preferences); err != nil {
		return nil, fmt.Errorf("解析 %s Preferences: %w", profile, err)
	}
	return preferences.Accounts, nil
}

func profileLocale(chromeRoot string, profile string) string {
	locale, _, _ := strings.Cut(profileLanguages(chromeRoot, profile), ",")
	return strings.TrimSpace(locale)
}

func copyFile(src string, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func openSQLiteCopy(srcPath string, prefix string) (database *sql.DB, closeDatabase func(), err error) {
	tempDir, err := os.MkdirTemp("", prefix)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if err != nil {
			os.RemoveAll(tempDir)
		}
	}()
	baseName := filepath.Base(srcPath)
	copyPath := filepath.Join(tempDir, baseName)
	if err = copyFile(srcPath, copyPath); err != nil {
		return nil, nil, err
	}
	_ = copyFile(srcPath+"-wal", copyPath+"-wal")
	_ = copyFile(srcPath+"-shm", copyPath+"-shm")
	_ = copyFile(srcPath+"-journal", copyPath+"-journal")
	uri := url.URL{Scheme: "file", Path: "/" + filepath.ToSlash(copyPath), RawQuery: "mode=ro"}
	if database, err = sql.Open("sqlite3", uri.String()); err != nil {
		return nil, nil, err
	}
	database.SetMaxOpenConns(1)
	return database, func() {
		database.Close()
		os.RemoveAll(tempDir)
	}, nil
}

type tokenRecord struct {
	encryptedToken []byte
	bindingKey     []byte
}

func readTokenServices(chromeRoot string, profile string) (map[string]tokenRecord, error) {
	databasePath := filepath.Join(chromeRoot, profile, "Web Data")
	database, closeDatabase, err := openSQLiteCopy(databasePath, "aistudio2api-webdata-*")
	if err != nil {
		return nil, err
	}
	defer closeDatabase()
	rows, err := database.Query("SELECT service, encrypted_token, binding_key FROM token_service WHERE service LIKE 'AccountId-%'")
	if err != nil {
		return nil, fmt.Errorf("读取 %s token_service: %w", profile, err)
	}
	defer rows.Close()
	records := map[string]tokenRecord{}
	for rows.Next() {
		var service string
		var record tokenRecord
		if err := rows.Scan(&service, &record.encryptedToken, &record.bindingKey); err != nil {
			return nil, fmt.Errorf("解析 %s token_service: %w", profile, err)
		}
		records[strings.TrimPrefix(service, "AccountId-")] = record
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("读取 %s token_service: %w", profile, err)
	}
	return records, nil
}

func readTokenService(chromeRoot string, profile string, gaiaID string) (string, []byte, []byte, error) {
	records, err := readTokenServices(chromeRoot, profile)
	if err != nil {
		return "", nil, nil, err
	}
	record, found := records[gaiaID]
	if !found {
		return "", nil, nil, fmt.Errorf("%s 的 token_service 没有 %s 的记录", profile, gaiaID)
	}
	if len(record.encryptedToken) == 0 {
		return "", nil, nil, fmt.Errorf("%s 的 token_service 缺少认证材料", profile)
	}
	return gaiaID, record.encryptedToken, record.bindingKey, nil
}

func resolveCookiesDBPath(chromeRoot string, profile string) string {
	p1 := filepath.Join(chromeRoot, profile, "Cookies")
	if _, err := os.Stat(p1); err == nil {
		return p1
	}
	p2 := filepath.Join(chromeRoot, profile, "Network", "Cookies")
	if _, err := os.Stat(p2); err == nil {
		return p2
	}
	return p1
}

func profileHasGoogleCookies(chromeRoot string, profile string) bool {
	dbPath := resolveCookiesDBPath(chromeRoot, profile)
	db, closeDB, err := openSQLiteCopy(dbPath, "aistudio2api-cookies-*")
	if err != nil {
		return false
	}
	defer closeDB()
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM cookies WHERE (host_key LIKE '%google.com') AND name IN ('SAPISID', '__Secure-1PSID')").Scan(&count)
	return err == nil && count >= 2
}

// readProfileGoogleCookies 从 macOS Chrome Profile 的 Cookies SQLite 数据库直接解密 .google.com Cookie
func readProfileGoogleCookies(chromeRoot string, profile string, masterKey []byte) ([]aistudio.StateCookie, error) {
	dbPath := resolveCookiesDBPath(chromeRoot, profile)
	db, closeDB, err := openSQLiteCopy(dbPath, "aistudio2api-cookies-*")
	if err != nil {
		return nil, fmt.Errorf("打开 %s Cookies 数据库: %w", profile, err)
	}
	defer closeDB()

	metaVersion := 0
	_ = db.QueryRow("SELECT CAST(value AS INTEGER) FROM meta WHERE key='version'").Scan(&metaVersion)

	rows, err := db.Query(`
		SELECT host_key, name, value, encrypted_value, path, expires_utc, is_secure, is_httponly, has_expires, samesite
		FROM cookies
		WHERE host_key = 'google.com' OR host_key LIKE '%.google.com'
	`)
	if err != nil {
		return nil, fmt.Errorf("查询 %s Cookies: %w", profile, err)
	}
	defer rows.Close()

	var result []aistudio.StateCookie
	now := time.Now()
	for rows.Next() {
		var (
			hostKey    string
			name       string
			plainValue string
			encValue   []byte
			path       string
			expiresUTC int64
			isSecure   int
			isHTTPOnly int
			hasExpires int
			sameSite   int
		)
		if err := rows.Scan(&hostKey, &name, &plainValue, &encValue, &path, &expiresUTC, &isSecure, &isHTTPOnly, &hasExpires, &sameSite); err != nil {
			continue
		}
		val := plainValue
		if val == "" && len(encValue) > 3 && string(encValue[:3]) == "v10" {
			decrypted, err := decryptMacOSV10Bytes(masterKey, encValue)
			if err != nil {
				continue
			}
			if metaVersion >= 24 && len(decrypted) >= 32 {
				decrypted = decrypted[32:]
			}
			val = string(decrypted)
		}
		if name == "" || val == "" {
			continue
		}
		expires := float64(-1)
		if hasExpires != 0 && expiresUTC > 0 {
			unixSec := (expiresUTC / 1000000) - 11644473600
			if unixSec > now.Unix() {
				maxExp := now.Add(maxCookieAge).Unix()
				if unixSec > maxExp {
					unixSec = maxExp
				}
				expires = float64(unixSec)
			}
		}
		if path == "" {
			path = "/"
		}
		var sameSiteStr string
		switch sameSite {
		case 0:
			sameSiteStr = "None"
		case 1:
			sameSiteStr = "Lax"
		case 2:
			sameSiteStr = "Strict"
		}
		result = append(result, aistudio.StateCookie{
			Name:     name,
			Value:    val,
			Domain:   hostKey,
			Path:     path,
			Expires:  expires,
			HTTPOnly: isHTTPOnly != 0,
			Secure:   isSecure != 0,
			SameSite: sameSiteStr,
		})
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("%s Cookies 数据库中未找到有效的 Google Cookie", profile)
	}
	return result, nil
}

func decryptMacOSV10Bytes(masterKey []byte, encrypted []byte) ([]byte, error) {
	if len(encrypted) <= 3 || string(encrypted[:3]) != "v10" {
		return nil, fmt.Errorf("非 macOS v10 密文")
	}
	ciphertext := encrypted[3:]
	if len(ciphertext) == 0 || len(ciphertext)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("v10 密文长度不是 AES 块大小的整数倍")
	}
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, err
	}
	iv := bytes.Repeat([]byte{' '}, aes.BlockSize)
	plaintext := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plaintext, ciphertext)
	padLen := int(plaintext[len(plaintext)-1])
	if padLen == 0 || padLen > aes.BlockSize || padLen > len(plaintext) {
		return nil, fmt.Errorf("v10 PKCS#7 padding 无效")
	}
	for i := len(plaintext) - padLen; i < len(plaintext); i++ {
		if plaintext[i] != byte(padLen) {
			return nil, fmt.Errorf("v10 PKCS#7 padding 校验失败")
		}
	}
	return plaintext[:len(plaintext)-padLen], nil
}

func naturalLess(left string, right string) bool {
	leftParts := splitNatural(left)
	rightParts := splitNatural(right)
	for index := 0; index < len(leftParts) && index < len(rightParts); index++ {
		leftNumber, leftErr := strconv.Atoi(leftParts[index])
		rightNumber, rightErr := strconv.Atoi(rightParts[index])
		if leftErr == nil && rightErr == nil {
			if leftNumber != rightNumber {
				return leftNumber < rightNumber
			}
			continue
		}
		comparison := strings.Compare(strings.ToLower(leftParts[index]), strings.ToLower(rightParts[index]))
		if comparison != 0 {
			return comparison < 0
		}
	}
	return len(leftParts) < len(rightParts)
}

func splitNatural(value string) []string {
	if value == "" {
		return nil
	}
	parts := make([]string, 0)
	start := 0
	lastDigit := unicode.IsDigit(rune(value[0]))
	for index, character := range value {
		isDigit := unicode.IsDigit(character)
		if index != 0 && isDigit != lastDigit {
			parts = append(parts, value[start:index])
			start = index
		}
		lastDigit = isDigit
	}
	return append(parts, value[start:])
}
