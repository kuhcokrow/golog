package capture

import (
	"net/http"
	"time"

	"github.com/kuhcokrow/golog/internal/entry"
)

const maxBodySize = 64 << 10

var sensitiveHeaders = []string{"Authorization", "Cookie", "Set-Cookie", "Proxy-Authorization"}

// RequestEntry builds a request entry, masking sensitive headers, truncating
// bodies and setting the duration since start.
func RequestEntry(id string, start time.Time, c entry.RequestContent) entry.Entry {
	c.DurationMs = float64(time.Since(start).Microseconds()) / 1000
	c.Headers = maskHeaders(c.Headers)
	c.ResponseHeaders = maskHeaders(c.ResponseHeaders)
	c.Body = truncate(c.Body)
	c.ResponseBody = truncate(c.ResponseBody)
	return entry.Entry{
		ID:        id,
		Type:      entry.TypeRequest,
		RequestID: id,
		CreatedAt: start,
		Content:   c,
	}
}

func maskHeaders(h http.Header) http.Header {
	c := h.Clone()
	if c == nil {
		c = http.Header{}
	}
	for _, k := range sensitiveHeaders {
		if c.Get(k) != "" {
			c.Set(k, "********")
		}
	}
	return c
}

func truncate(s string) string {
	if len(s) > maxBodySize {
		return s[:maxBodySize]
	}
	return s
}
