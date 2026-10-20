package firmware

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
)

// Relation is how a device firmware version compares with the bundled copy.
type Relation int

const (
	Unknown Relation = iota
	Older
	Equal
	Newer
)

func parseVersion(s string) ([3]uint64, bool) {
	var out [3]uint64
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return out, false
	}
	for i, part := range parts {
		if part == "" || len(part) > 1 && part[0] == '0' {
			return out, false
		}
		n, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			return out, false
		}
		out[i] = n
	}
	return out, true
}

// Relate compares dotted numeric versions such as 0.1.0. Unknown if either
// value is not three decimal components without leading zeros.
func Relate(device, bundled string) Relation {
	a, okA := parseVersion(device)
	b, okB := parseVersion(bundled)
	if !okA || !okB {
		return Unknown
	}
	for i := range a {
		if a[i] < b[i] {
			return Older
		}
		if a[i] > b[i] {
			return Newer
		}
	}
	return Equal
}

func NeedsUpdate(device, bundled string) bool {
	return Relate(device, bundled) == Older
}

// ReadVersion returns the packaged firmwareVersion, or empty if the manifest
// is missing or the value is not a dotted numeric version.
func ReadVersion(path string) string {
	b, err := os.ReadFile(path)
	if err != nil || len(b) == 0 || len(b) > 16*1024 {
		return ""
	}
	var m struct {
		FirmwareVersion string `json:"firmwareVersion"`
	}
	if json.Unmarshal(b, &m) != nil {
		return ""
	}
	if _, ok := parseVersion(m.FirmwareVersion); !ok {
		return ""
	}
	return m.FirmwareVersion
}
