package waa

import (
	"testing"

	"github.com/Mag1cFall/AIStudio2API/internal/waa/mozjs"
)

func TestCamouLocaleSanitizationAndContextInit(t *testing.T) {
	cases := []struct {
		input string
		want  [3]string
	}{
		{"zh-CN", [3]string{"zh", "CN", "zh-CN"}},
		{"zh_CN.UTF-8", [3]string{"zh", "CN", "zh-CN"}},
		{"zh-CN\u200b", [3]string{"zh", "CN", "zh-CN"}},
		{"\ufeffen-US\u200e", [3]string{"en", "US", "en-US"}},
		{"zh-Hans-CN", [3]string{"zh", "US", "zh-US"}},
		{"", [3]string{"en", "US", "en-US"}},
		{"C", [3]string{"en", "US", "en-US"}},
		{"invalid_locale_string", [3]string{"en", "US", "en-US"}},
	}
	for _, tc := range cases {
		got := camouLocale(tc.input)
		if got != tc.want {
			t.Fatalf("camouLocale(%q) = %v, want %v", tc.input, got, tc.want)
		}
		mozjs.SetCamouLocale(got[0], got[1], got[2])
		cx, err := mozjs.NewContext(mozjs.Options{HeapBytes: 32 << 20, Locale: got[2]})
		if err != nil {
			t.Fatalf("NewContext(%v) error: %v", got, err)
		}
		cx.Interrupt()
		cx.Close()
		cx.Interrupt()
	}
}
