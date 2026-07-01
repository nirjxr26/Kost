// Package analyze detects over-provisioned workloads from pod usage data.
package analyze

import (
	"fmt"

	"github.com/nirjxr26/Kost/internal/k8s"
)

type Finding struct {
	Kind         string  `json:"kind"`
	Namespace    string  `json:"namespace"`
	Workload     string  `json:"workload"`
	OwnerKind    string  `json:"owner_kind"`
	OwnerName    string  `json:"owner_name"`
	RequestCPU   float64 `json:"request_cpu"`
	RequestMem   float64 `json:"request_mem"`
	ActualCPU    float64 `json:"actual_cpu"`
	ActualMem    float64 `json:"actual_mem"`
	WasteCPU     float64 `json:"waste_cpu"`
	WasteMem     float64 `json:"waste_mem"`
	WasteMonthly float64 `json:"waste_monthly"`
	FixCommand   string  `json:"fix_command"`
}

type Report struct {
	ClusterName string    `json:"cluster_name"`
	Findings    []Finding `json:"findings"`
	TotalWaste  float64   `json:"total_waste_monthly"`
}

type Analyzer struct {
	cluster  string
	ratio    float64
	minWaste float64
	cpuPrice float64
	memPrice float64
}

func New(cluster string, ratio, minWaste, cpuPrice, memPrice float64) *Analyzer {
	return &Analyzer{
		cluster:  cluster,
		ratio:    ratio,
		minWaste: minWaste,
		cpuPrice: cpuPrice,
		memPrice: memPrice,
	}
}

// Run flags over-provisioned pods where request > actual * ratio.
func (a *Analyzer) Run(usages []k8s.PodUsage) *Report {
	r := &Report{ClusterName: a.cluster}
	for _, u := range usages {
		wasteCPU := u.RequestCPU - u.ActualCPU
		wasteMem := u.RequestMem - u.ActualMem
		if wasteCPU <= 0 || wasteMem <= 0 {
			continue
		}
		if u.RequestCPU < u.ActualCPU*a.ratio && u.RequestMem < u.ActualMem*a.ratio {
			continue
		}
		monthly := wasteCPU*a.cpuPrice*730 + wasteMem*a.memPrice*730
		if monthly < a.minWaste {
			continue
		}
		suggestCPU := u.ActualCPU * 1.3
		if suggestCPU < 0.1 {
			suggestCPU = 0.1
		}
		suggestMem := u.ActualMem * 1.3
		if suggestMem < 0.1 {
			suggestMem = 0.1
		}
		r.Findings = append(r.Findings, Finding{
			Kind:         "over-provisioned",
			Namespace:    u.Namespace,
			Workload:     u.Name,
			OwnerKind:    u.OwnerKind,
			OwnerName:    u.OwnerName,
			RequestCPU:   u.RequestCPU,
			RequestMem:   u.RequestMem,
			ActualCPU:    u.ActualCPU,
			ActualMem:    u.ActualMem,
			WasteCPU:     wasteCPU,
			WasteMem:     wasteMem,
			WasteMonthly: monthly,
			FixCommand:   fixCommand(u),
		})
		r.TotalWaste += monthly
	}
	return r
}

// fixCommand generates the kubectl command targeting the owning resource.
// Falls back to the pod name if no owner is detected.
func fixCommand(u k8s.PodUsage) string {
	target := u.OwnerName
	kind := u.OwnerKind
	if target == "" {
		target = u.Name
		kind = "pod"
	}
	suggestCPU := u.ActualCPU * 1.3
	if suggestCPU < 0.1 {
		suggestCPU = 0.1
	}
	suggestMem := u.ActualMem * 1.3
	if suggestMem < 0.1 {
		suggestMem = 0.1
	}
	return fmt.Sprintf(
		"kubectl set resources %s/%s -n %s --requests=cpu=%.1f,memory=%.0fMi",
		kind, target, u.Namespace, suggestCPU, suggestMem*1024)
}
