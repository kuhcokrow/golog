package golog

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"
)

type EntryType string

const (
	TypeRequest   EntryType = "request"
	TypeQuery     EntryType = "query"
	TypeLog       EntryType = "log"
	TypeException EntryType = "exception"
	TypeCache     EntryType = "cache"
)

type Entry struct {
	ID        string    `json:"id"`
	Type      EntryType `json:"type"`
	RequestID string    `json:"requestId,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	Content   any       `json:"content"`
}

type RequestContent struct {
	Method          string      `json:"method"`
	Path            string      `json:"path"`
	Query           string      `json:"query,omitempty"`
	IP              string      `json:"ip"`
	Status          int         `json:"status"`
	DurationMs      float64     `json:"durationMs"`
	Headers         http.Header `json:"headers"`
	Body            string      `json:"body,omitempty"`
	ResponseHeaders http.Header `json:"responseHeaders"`
	ResponseBody    string      `json:"responseBody,omitempty"`
}

type ExceptionContent struct {
	Message string `json:"message"`
	Stack   string `json:"stack"`
}

func newID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
