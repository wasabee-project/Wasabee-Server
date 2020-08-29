package wasabee

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
	return [...]string{"All", "Alpha", "Beta", "Gamma", "Delta", "Epison", "Zeta", "Eta"}[z]
}
