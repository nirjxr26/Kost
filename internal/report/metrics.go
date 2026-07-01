package report

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/nirjar/kost/internal/analyze"
)

// LatestReport is a thread-safe holder for the most recent analysis.
// Atomic pointer avoids locks on the read-heavy /metrics path.
type LatestReport struct {
	ptr atomic.Pointer[analyze.Report]
}

func NewLatest() *LatestReport { return &LatestReport{} }

func (l *LatestReport) Store(r *analyze.Report) { l.ptr.Store(r) }
func (l *LatestReport) Load() *analyze.Report   { return l.ptr.Load() }

// MetricsHandler returns an http.HandlerFunc that exposes Prometheus text format.
func MetricsHandler(cluster string, latest *LatestReport) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		rep := latest.Load()
		if rep == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintln(w, "# no report yet")
			return
		}
		ts := time.Now().UnixMilli()
		fmt.Fprintf(w, "# HELP kost_over_provisioned_count Number of over-provisioned workloads\n")
		fmt.Fprintf(w, "# TYPE kost_over_provisioned_count gauge\n")
		fmt.Fprintf(w, "kost_over_provisioned_count{cluster=%q} %d %d\n", cluster, len(rep.Findings), ts)
		fmt.Fprintf(w, "# HELP kost_waste_dollars Estimated monthly waste in USD\n")
		fmt.Fprintf(w, "# TYPE kost_waste_dollars gauge\n")
		fmt.Fprintf(w, "kost_waste_dollars{cluster=%q} %.2f %d\n", cluster, rep.TotalWaste, ts)
	}
}
