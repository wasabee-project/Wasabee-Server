package model

import (
	"database/sql"
	"time"

	"github.com/wasabee-project/Wasabee-Server/log"
)

// agent is set by the V API
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
	TelegramID  string   `json:"telegramid"` // is this really an int?
	Telegram    string   `json:"telegram"`
	Email       string   `json:"email"`
	StartLat    float64  `json:"lat"`
	StartLon    float64  `json:"lon"`
	Distance    int64    `json:"distance"`
	Roles       []uint8  `json:"roles"`
}

// vToDB updates the database to reflect an agent's current status at V.
// callback
func VToDB(a *VAgent) error {
	if a.Agent == "" {
		return nil
	}

	// telegram, startlat, startlon, distance, fetched are not set on the "trust" API call.
	// do not overwrite telegram if we happen to get it from another source
	_, err := db.Exec("REPLACE INTO v (enlid, gid, vlevel, vpoints, agent, level, quarantine, active, blacklisted, verified, flagged, banned, cellid, startlat, startlon, distance, fetched) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,UTC_TIMESTAMP())",
		a.EnlID, a.Gid, a.Vlevel, a.Vpoints, a.Agent, a.Level, a.Quarantine, a.Active, a.Blacklisted, a.Verified, a.Flagged, a.Banned, a.CellID, a.StartLat, a.StartLon, a.Distance)
	if err != nil {
		log.Error(err)
		return err
	}

	return nil
}

func VFromDB(gid GoogleID) (*VAgent, time.Time, error) {
	a := VAgent{
		Gid: gid,
	}
	var fetched string
	var t time.Time
	var vlevel, vpoints sql.NullInt64
	var telegram, cellid sql.NullString

	err := db.QueryRow("SELECT enlid, vlevel, vpoints, agent, level, quarantine, active, blacklisted, verified, flagged, banned, cellid, telegram, startlat, startlon, distance, fetched FROM v WHERE gid = ?", gid).Scan(&a.EnlID, &vlevel, &vpoints, &a.Agent, &a.Level, &a.Quarantine, &a.Active, &a.Blacklisted, &a.Verified, &a.Flagged, &a.Banned, &cellid, &telegram, &a.StartLat, &a.StartLon, &a.Distance, &fetched)
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

	t, err = time.ParseInLocation("2006-01-02 15:04:05", fetched, time.UTC)
	if err != nil {
		log.Error(err)
		// return &a, t, err
	}
	log.Debugw("VFromDB", "gid", gid, "fetched", fetched, "data", a)
	return &a, t, nil
}

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

func GetTeamsByVID(v int64) ([]TeamID, error) {
	var teams []TeamID

	row, err := db.Query("SELECT teamID FROM team WHERE vteam = ?", v)
	if err != nil {
		log.Error(err)
		return teams, err
	}
	defer row.Close()

	var teamID TeamID
	for row.Next() {
		err = row.Scan(&teamID)
		if err != nil {
			log.Error(err)
			continue
		}
		teams = append(teams, teamID)
	}
	return teams, nil
}

// VConfigure sets V connection for a Wasabee team -- caller should verify ownership
func (teamID TeamID) VConfigure(vteam int64, role uint8) error {
	/* r := v.Roles[role]
	if !ok {
		err := fmt.Errorf("invalid role")
		log.Error(err)
		return err
	} */

	log.Infow("linking team to V", "teamID", teamID, "vteam", vteam, "role", role)

	_, err := db.Exec("UPDATE team SET vteam = ?, vrole = ? WHERE teamID = ?", vteam, role, teamID)
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func (gid GoogleID) VAPIkey() (string, error) {
	var key string
	err := db.QueryRow("SELECT VAPIkey FROM v WHERE gid = ?", gid).Scan(&key)
	if err != nil {
		log.Error(err)
		return "", err
	}
	return key, nil
}

func GetGIDFromEnlID(enlid string) (GoogleID, error) {
	var gid GoogleID

	err := db.QueryRow("SELECT gid FROM v WHERE enlid = ?", enlid).Scan(&gid)
	if err != nil && err != sql.ErrNoRows {
		log.Error(err)
		return gid, err
	}
	return gid, nil
}
