package report

import (
	_ "embed"
	"encoding/json"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/nirjxr26/Kost/internal/analyze"
)

type reportEntry struct {
	Report   *analyze.Report `json:"report"`
	UnixTime int64           `json:"unix_time"`
}

type ReportHistory struct {
	mu   sync.Mutex
	buf  []reportEntry
	cap  int
	next int
	full bool
}

func NewReportHistory(capacity int) *ReportHistory {
	return &ReportHistory{cap: capacity, buf: make([]reportEntry, capacity)}
}

func (h *ReportHistory) Push(r *analyze.Report) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.buf[h.next] = reportEntry{Report: r, UnixTime: time.Now().Unix()}
	h.next = (h.next + 1) % h.cap
	if h.next == 0 {
		h.full = true
	}
}

func (h *ReportHistory) All() []reportEntry {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := h.next
	if h.full {
		n = h.cap
	}
	out := make([]reportEntry, n)
	for i := 0; i < n; i++ {
		out[i] = h.buf[i]
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UnixTime > out[j].UnixTime })
	return out
}

func (h *ReportHistory) Latest() *analyze.Report {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.full {
		return h.buf[(h.next-1+h.cap)%h.cap].Report
	}
	if h.next == 0 {
		return nil
	}
	return h.buf[h.next-1].Report
}

//go:embed dashboard.html
var dashboardHTML string

func DashboardHandler(history *ReportHistory) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(dashboardHTML))
	}
}

func ReportsHandler(history *ReportHistory) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(history.All())
	}
}
