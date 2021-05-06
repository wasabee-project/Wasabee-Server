package wasabee

// IntelFaction is stored as an in in the database
type IntelFaction int8

const (
	factionUnset = -1
	factionRes   = 0
	factionEnl   = 1
)

// FactionFromString takes a string and returns the int
func FactionFromString(in string) IntelFaction {
	switch in {
	case "RESISTANCE", "RES", "res", "0":
		return factionRes
	case "ENLIGHTENED", "ENL", "enl", "1":
		return factionEnl
	default:
		return factionUnset
	}
}

// String returns the string representation of an IntelFaction
func (f IntelFaction) String() string {
	switch f {
	case factionRes:
		return "RESISTANCE"
	case factionEnl:
		return "ENLIGHTENED"
	default:
		return "unset"
	}
}
