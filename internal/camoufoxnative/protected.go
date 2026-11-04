package camoufoxnative

import (
	"io"
	"net/http"
)

// ProtectedResponse 表示由固定指纹运行时流式返回的 MakerSuite 响应
type ProtectedResponse struct {
	StatusCode int
	Header     http.Header
	Body       io.ReadCloser
}
