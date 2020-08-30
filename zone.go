package wasabee

import (
	"bytes"
	"encoding/json"
)

// This is a reasonable pattern for enums. We should convert others to use it

// Zone is the sub-operation zone identifer
type Zone int

// ZoneAlpha ... is the friendly name for the zones
const (
	ZoneAll Zone = iota
	ZoneAlpha
	ZoneBeta
	ZoneGamma
	ZoneDelta
	ZoneEpsilon
	ZoneZeta
	ZoneEta
)

// String is the string represenation for the zone
func (z Zone) String() string {
	return zoneToString[z]
}

var zoneToString = map[Zone]string{
	ZoneAll:     "All",
	ZoneAlpha:   "Alpha",
	ZoneBeta:    "Beta",
	ZoneGamma:   "Gamma",
	ZoneDelta:   "Delta",
	ZoneEpsilon: "Epison",
	ZoneZeta:    "Zeta",
	ZoneEta:     "Eta",
}

var zoneToID = map[string]Zone{
	"All":     ZoneAll,
	"Alpha":   ZoneAlpha,
	"Beta":    ZoneBeta,
	"Gamma":   ZoneGamma,
	"Delta":   ZoneDelta,
	"Epsilon": ZoneEpsilon,
	"Zeta":    ZoneZeta,
	"Eta":     ZoneEta,
}

// Valid returns a boolean if the zone is in the valid range
func (z Zone) Valid() bool {
	if z >= ZoneAll && z <= ZoneEta {
		return true
	}
	return false
}

// ZoneFromString takes a string and returns a zone
func ZoneFromString(in string) Zone {
	// unmatched == ZoneAll
	return zoneToID[in]
}

// MarshalJSON marshals the enum as a quoted json string
func (z Zone) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString(`"`)
	buffer.WriteString(zoneToString[z])
	buffer.WriteString(`"`)
	return buffer.Bytes(), nil
}

// UnmarshalJSON unmashals a quoted json string to the enum value
func (z *Zone) UnmarshalJSON(b []byte) error {
	var j string
	err := json.Unmarshal(b, &j)
	if err != nil {
		return err
	}
	// unmatched == ZoneAll
	*z = zoneToID[j]
	return nil
}
