package rocks

import (
	"github.com/wasabee-project/Wasabee-Server/model"
)

// communityNotice is sent from a community when an agent is added or removed
// consumed by RocksCommunitySync function below
type communityNotice struct {
	Community string           `json:"community"`
	Action    string           `json:"action"`
	User      agent            `json:"user"`
	TGId      model.TelegramID `json:"tg_id"`
	TGName    string           `json:"tg_user"`
}

// communityResponse is returned from a query request
type communityResponse struct {
	Community  string           `json:"community"`
	Title      string           `json:"title"`
	Members    []model.GoogleID `json:"members"`    // googleID
	Moderators []string         `json:"moderators"` // googleID
	User       agent            `json:"user"`       // (Members,Moderators || User) present, not both
	Error      string           `json:"error"`
}

// Agent is the data sent by enl.rocks -- the version sent in the communityResponse is different, but close enough for our purposes
type agent struct {
	Gid      model.GoogleID `json:"gid"`
	TGId     int64          `json:"tgid"`
	Agent    string         `json:"agentid"`
	Verified bool           `json:"verified"`
	Smurf    bool           `json:"smurf"`
}

// sent by rocks on community pushes
type rocksPushResponse struct {
	Error   string `json:"error"`
	Success bool   `json:"success"`
}
