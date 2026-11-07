package waa

import (
	"strings"
	"unicode"
)

// camouLocale 返回账户语言对应的 Camoufox 语言、地区与 navigator.language
func camouLocale(locale string) [3]string {
	locale = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
			return -1
		}
		if r == '_' {
			return '-'
		}
		return r
	}, locale)
	locale = strings.TrimSpace(locale)
	if idx := strings.IndexAny(locale, ".;@ "); idx >= 0 {
		locale = locale[:idx]
	}
	language, region, _ := strings.Cut(locale, "-")
	if idx := strings.IndexByte(region, '-'); idx >= 0 {
		region = region[:idx]
	}
	if !isAlphaASCII(language, 2, 3) {
		return [3]string{"en", "US", "en-US"}
	}
	language = strings.ToLower(language)
	if !isAlphaNumASCII(region, 2, 3) {
		return [3]string{language, "US", language + "-US"}
	}
	region = strings.ToUpper(region)
	return [3]string{language, region, language + "-" + region}
}

func isAlphaASCII(s string, minLen, maxLen int) bool {
	if len(s) < minLen || len(s) > maxLen {
		return false
	}
	for i := range len(s) {
		c := s[i]
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') {
			return false
		}
	}
	return true
}

func isAlphaNumASCII(s string, minLen, maxLen int) bool {
	if len(s) < minLen || len(s) > maxLen {
		return false
	}
	for i := range len(s) {
		c := s[i]
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') {
			return false
		}
	}
	return true
}
