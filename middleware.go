package golog

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"time"
)

const maxBodySize = 64 << 10

var sensitiveHeaders = []string{"Authorization", "Cookie", "Set-Cookie", "Proxy-Authorization"}

func (g *Golog) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == g.cfg.Path || strings.HasPrefix(r.URL.Path, g.cfg.Path+"/") {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		id := newID()
		ctx := WithRequestID(r.Context(), id)
		r = r.WithContext(ctx)

		var reqBody []byte
		if r.Body != nil {
			reqBody, _ = io.ReadAll(io.LimitReader(r.Body, maxBodySize))
			r.Body = struct {
				io.Reader
				io.Closer
			}{io.MultiReader(bytes.NewReader(reqBody), r.Body), r.Body}
		}

		rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}

		defer func() {
			p := recover()
			if p != nil && p != http.ErrAbortHandler {
				g.Record(ctx, TypeException, ExceptionContent{
					Message: fmt.Sprint(p),
					Stack:   string(debug.Stack()),
				})
				if !rec.wroteHeader {
					rec.status = http.StatusInternalServerError
				}
			}

			g.save(Entry{
				ID:        id,
				Type:      TypeRequest,
				RequestID: id,
				CreatedAt: start,
				Content: RequestContent{
					Method:          r.Method,
					Path:            r.URL.Path,
					Query:           r.URL.RawQuery,
					IP:              clientIP(r),
					Status:          rec.status,
					DurationMs:      float64(time.Since(start).Microseconds()) / 1000,
					Headers:         maskHeaders(r.Header),
					Body:            string(reqBody),
					ResponseHeaders: maskHeaders(rec.Header()),
					ResponseBody:    rec.body.String(),
				},
			})

			if p != nil {
				panic(p)
			}
		}()

		next.ServeHTTP(rec, r)
	})
}

type responseRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
	body        bytes.Buffer
}

func (r *responseRecorder) WriteHeader(code int) {
	if !r.wroteHeader {
		r.status = code
		r.wroteHeader = true
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.wroteHeader = true
	if room := maxBodySize - r.body.Len(); room > 0 {
		r.body.Write(b[:min(len(b), room)])
	}
	return r.ResponseWriter.Write(b)
}

func (r *responseRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (r *responseRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

func maskHeaders(h http.Header) http.Header {
	c := h.Clone()
	for _, k := range sensitiveHeaders {
		if c.Get(k) != "" {
			c.Set(k, "********")
		}
	}
	return c
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
