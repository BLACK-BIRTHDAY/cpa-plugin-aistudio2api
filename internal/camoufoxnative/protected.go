package camoufoxnative

import (
	"context"
	"io"
	"net/http"
	"time"
)

// ProtectedResponse 表示由固定指纹运行时流式返回的 MakerSuite 响应
type ProtectedResponse struct {
	StatusCode int
	Header     http.Header
	Body       io.ReadCloser
}

// FollowUp 是受保护请求发出 Delay 后以相同请求头发出的伴随请求，响应被丢弃
type FollowUp struct {
	URL   string        `json:"url"`
	Body  []byte        `json:"body"`
	Delay time.Duration `json:"delay"`
}

// followUpKey 是上下文中伴随请求的键
type followUpKey struct{}

// WithFollowUp 返回携带伴随请求的上下文，SendProtected 发出受保护请求时同时安排它
func WithFollowUp(ctx context.Context, followUp FollowUp) context.Context {
	return context.WithValue(ctx, followUpKey{}, followUp)
}

// FollowUpFrom 返回上下文中的伴随请求
func FollowUpFrom(ctx context.Context) (FollowUp, bool) {
	followUp, ok := ctx.Value(followUpKey{}).(FollowUp)
	return followUp, ok && followUp.URL != ""
}
