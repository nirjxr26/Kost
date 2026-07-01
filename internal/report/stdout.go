// Package report formats analysis results for consumption.
package report

import (
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/nirjxr26/Kost/internal/analyze"
)

type reportEnvelope struct {
	Timestamp  string           `json:"timestamp"`
	Cluster    string           `json:"cluster"`
	TotalWaste float64          `json:"total_waste_monthly"`
	Findings   []analyze.Finding `json:"findings,omitempty"`
	Healthy    bool             `json:"healthy"`
}

// Stdout writes the report as JSON to stdout.
func Stdout(r *analyze.Report) error {
	out := reportEnvelope{
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Cluster:    r.ClusterName,
		TotalWaste: r.TotalWaste,
		Findings:   r.Findings,
		Healthy:    len(r.Findings) == 0,
	}
	b, err := json.Marshal(out)
	if err != nil {
		return err
	}
	os.Stdout.Write(b)
	os.Stdout.Write([]byte("\n"))
	return nil
}

// LogReport logs findings one per line.
func LogReport(r *analyze.Report) {
	for _, f := range r.Findings {
		log.Printf("finding kind=%s namespace=%s workload=%s waste=%.2f fix=%s",
			f.Kind, f.Namespace, f.Workload, f.WasteMonthly, f.FixCommand)
	}
	if len(r.Findings) == 0 {
		log.Printf("finding none cluster=%s — no over-provisioned workloads", r.ClusterName)
	}
}
