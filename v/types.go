package v

import (
	"github.com/wasabee-project/Wasabee-Server/model"
)

type vTeamID int64

// Result is set by the V trust API
type trustResult struct {
	Status  string       `json:"status"`
	Message string       `json:"message,omitempty"`
	Data    model.VAgent `json:"data"`
}

// Version 2.0 of the team query
type teamResult struct {
	Status  string         `json:"status"`
	Message string         `json:"message,omitempty"`
	Agents  []model.VAgent `json:"data"`
}

type bulkResult struct {
	Status string                            `json:"status"`
	Agents map[model.TelegramID]model.VAgent `json:"data"`
}

// myTeams is what V returns when an agent's teams are requested
type myTeams struct {
	Status  string   `json:"status"`
	Teams   []myTeam `json:"data"`
	Message string   `json:"message,omitempty"`
}

type myTeam struct {
	TeamID vTeamID `json:"teamid"`
	Name   string  `json:"team"`
	Roles  []struct {
		ID   uint8  `json:"id"`
		Name string `json:"name"`
	} `json:"roles"`
	Admin bool `json:"admin"`
}

var rolenames = map[uint8]string{
	0:   "All",
	1:   "Planner",
	2:   "Operator",
	3:   "Linker",
	4:   "Keyfarming",
	5:   "Cleaner",
	6:   "Field Agent",
	7:   "Item Sponsor",
	8:   "Key Transport",
	9:   "Recharging",
	10:  "Software Support",
	11:  "Anomaly TL",
	12:  "Team Lead",
	13:  "Other",
	100: "Team-0",
	101: "Team-1",
	102: "Team-2",
	103: "Team-3",
	104: "Team-4",
	105: "Team-5",
	106: "Team-6",
	107: "Team-7",
	108: "Team-8",
	109: "Team-9",
	110: "Team-10",
	111: "Team-11",
	112: "Team-12",
	113: "Team-13",
	114: "Team-14",
	115: "Team-15",
	116: "Team-16",
	117: "Team-17",
	118: "Team-18",
	119: "Team-19",
}
