package model

import (
	"database/sql"
	"time"

	"github.com/wasabee-project/Wasabee-Server/log"
)

// VAgent is set by the V API
// most of these fields are empty unless filled in by a team query
type VAgent struct {
	EnlID       string   `json:"enlid"`
	Gid         GoogleID `json:"gid"`
	Vlevel      int64    `json:"vlevel"`
	Vpoints     int64    `json:"vpoints"`
	Agent       string   `json:"agent"`
	Level       int64    `json:"level"`
	Quarantine  bool     `json:"quarantine"`
	Active      bool     `json:"active"`
	Blacklisted bool     `json:"blacklisted"`
	Verified    bool     `json:"verified"`
	Flagged     bool     `json:"flagged"`
	Banned      bool     `json:"banned_by_nia"`
	CellID      string   `json:"cellid"`
	TelegramID  int64    `json:"telegramid"`
	Telegram    string   `json:"telegram"`
	Email       string   `json:"email"`
	StartLat    float64  `json:"lat"`
	StartLon    float64  `json:"lon"`
	Distance    int64    `json:"distance"`
	Roles       []struct {
		ID   uint8  `json:"id"`
		Name string `json:"name"`
	} `json:"roles"`
}

// VToDB updates the database to reflect an agent's current status at V.
func VToDB(a *VAgent) error {
	if a.Agent == "" {
		return nil
	}

	// telegram, startlat, startlon, distance, fetched are not set on the "trust" API call.
	// use ON DUPLICATE so as to not overwrite apikey & telegram from other sources
	// TODO: prune fields we will never use or that V never sends
	_, err := db.Exec("INSERT INTO v (enlid, gid, vlevel, vpoints, agent, level, quarantine, active, blacklisted, verified, flagged, banned, cellid, startlat, startlon, distance, fetched) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,UTC_TIMESTAMP()) ON DUPLICATE KEY UPDATE agent=?, quarantine=?, blacklisted=?, verified=?, flagged=?, banned=?, fetched=UTC_TIMESTAMP()",
		a.EnlID, a.Gid, a.Vlevel, a.Vpoints, a.Agent, a.Level, a.Quarantine, a.Active, a.Blacklisted, a.Verified, a.Flagged, a.Banned, a.CellID, a.StartLat, a.StartLon, a.Distance,
		a.Agent, a.Quarantine, a.Blacklisted, a.Verified, a.Flagged, a.Banned)
	if err != nil {
		log.Error(err)
		return err
	}

	if a.TelegramID != 0 {
		existing, err := a.Gid.TelegramID()
		if err != nil {
			log.Error(err)
			return err
		}
		if existing == 0 {
			err := a.Gid.SetTelegramID(TelegramID(a.TelegramID), a.Telegram)
			if err != nil {
				log.Error(err)
				return err
			}
		}
	}

	return nil
}

// VFromDB pulls a V agent from the database
func VFromDB(gid GoogleID) (*VAgent, time.Time, error) {
	a := VAgent{
		Gid: gid,
	}
	var fetched string
	var t time.Time
	var vlevel, vpoints, distance sql.NullInt64
	var telegram, cellid sql.NullString
	var startlat, startlon sql.NullFloat64

	err := db.QueryRow("SELECT enlid, vlevel, vpoints, agent, level, quarantine, active, blacklisted, verified, flagged, banned, cellid, telegram, startlat, startlon, distance, fetched FROM v WHERE gid = ?", gid).Scan(&a.EnlID, &vlevel, &vpoints, &a.Agent, &a.Level, &a.Quarantine, &a.Active, &a.Blacklisted, &a.Verified, &a.Flagged, &a.Banned, &cellid, &telegram, &startlat, &startlon, &distance, &fetched)
	if err != nil && err != sql.ErrNoRows {
		log.Error(err)
		return &a, t, err
	}

	if err == sql.ErrNoRows {
		return &a, t, nil
	}

	if fetched == "" {
		return &a, t, nil
	}

	if vlevel.Valid {
		a.Vlevel = vlevel.Int64
	}
	if vpoints.Valid {
		a.Vpoints = vpoints.Int64
	}
	if telegram.Valid {
		a.Telegram = telegram.String
	}
	if cellid.Valid {
		a.CellID = cellid.String
	}
	if startlat.Valid {
		a.StartLat = startlat.Float64
	}
	if startlon.Valid {
		a.StartLon = startlon.Float64
	}
	if distance.Valid {
		a.Distance = distance.Int64
	}

	t, err = time.ParseInLocation("2006-01-02 15:04:05", fetched, time.UTC)
	if err != nil {
		log.Error(err)
		// return &a, t, err
	}
	// log.Debugw("VFromDB", "gid", gid, "fetched", fetched, "data", a)
	return &a, t, nil
}

// VTeam returns the V team/role pair for a Wasabee team
func (teamID TeamID) VTeam() (int64, uint8, error) {
	var team int64
	var role uint8
	err := db.QueryRow("SELECT vteam, vrole FROM team WHERE teamID = ?", teamID).Scan(&team, &role)
	if err != nil {
		log.Error(err)
		return 0, 0, err
	}
	return team, role, nil
}

// GetTeamsByVID returns all wasabee teams which match the V team ID
func GetTeamsByVID(v int64) ([]TeamID, error) {
	var teams []TeamID

	row, err := db.Query("SELECT teamID FROM team WHERE vteam = ?", v)
	if err != nil {
		log.Error(err)
		return teams, err
	}
	defer row.Close()

	for row.Next() {
		var teamID TeamID
		err = row.Scan(&teamID)
		if err != nil {
			log.Error(err)
			continue
		}
		teams = append(teams, teamID)
	}
	return teams, nil
}

// VTeamExists checks if a v-team/role pair exists for a specific googleID
func VTeamExists(vteam int64, vrole uint8, gid GoogleID) (bool, error) {
	i := false
	err := db.QueryRow("SELECT COUNT(*) FROM team WHERE vteam = ? AND vrole = ? AND owner = ?", vteam, vrole, gid).Scan(&i)
	if err != nil {
		log.Error(err)
		return i, err
	}
	return i, nil
}

// VConfigure sets V connection for a Wasabee team -- caller should verify ownership
func (teamID TeamID) VConfigure(vteam int64, role uint8) error {
	log.Infow("linking team to V", "teamID", teamID, "vteam", vteam, "role", role)

	if _, err := db.Exec("UPDATE team SET vteam = ?, vrole = ? WHERE teamID = ?", vteam, role, teamID); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// GetGIDFromEnlID looks up an agent's GoogleID by V EnlID
func GetGIDFromEnlID(enlid string) (GoogleID, error) {
	var gid GoogleID

	err := db.QueryRow("SELECT gid FROM v WHERE enlid = ?", enlid).Scan(&gid)
	if err != nil && err != sql.ErrNoRows {
		log.Error(err)
		return gid, err
	}
	return gid, nil
}

// GetVAPIkey (gid GoogleID) loads an agents's V API key (this should be unusual); "" is "not set"
func (gid GoogleID) GetVAPIkey() (string, error) {
	var v sql.NullString

	err := db.QueryRow("SELECT VAPIkey FROM v WHERE GID = ?", gid).Scan(&v)
	if err != nil {
		log.Error(err)
		return "", nil
	}
	if !v.Valid {
		return "", nil
	}
	return v.String, nil
}

// SetVAPIkey stores an agent's V API key (this should be unusual)
func (gid GoogleID) SetVAPIkey(key string) error {
	if _, err := db.Exec("UPDATE v SET VAPIkey = ? WHERE gid  = ? ", key, gid); err != nil {
		log.Error(err)
		return err
	}
	return nil
}
