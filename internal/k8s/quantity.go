package k8s

import (
	"fmt"
	"strings"
)

// parseQuantity parses a Kubernetes quantity string into a millibase value.
// For CPU: returns millicores ("500m" → 500, "1" → 1000).
// For memory: returns bytes ("256Mi" → 268435456, "1Gi" → 1073741824).
func parseQuantity(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return 0, fmt.Errorf("empty quantity")
	}

	type suffix struct {
		text string
		mult int64
	}

	suffixes := []suffix{
		{"Gi", 1024 * 1024 * 1024},
		{"Mi", 1024 * 1024},
		{"Ki", 1024},
		{"G", 1000 * 1000 * 1000},
		{"M", 1000 * 1000},
		{"k", 1000},
		{"K", 1000},
		{"m", 1},
	}

	for _, sf := range suffixes {
		if strings.HasSuffix(s, sf.text) && len(s) > len(sf.text) {
			val, err := parseInt(s[:len(s)-len(sf.text)])
			if err != nil {
				return 0, err
			}
			return val * sf.mult, nil
		}
	}

	// No suffix — plain integer
	return parseInt(s)
}

func parseInt(s string) (int64, error) {
	if len(s) == 0 {
		return 0, fmt.Errorf("empty number")
	}
	var val int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid character %q in %q", c, s)
		}
		val = val*10 + int64(c-'0')
	}
	return val, nil
}
