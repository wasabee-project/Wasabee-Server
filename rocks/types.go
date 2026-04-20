package rocks

import (
	"github.com/wasabee-project/Wasabee-Server/model"
)

// communityNotice is sent from an enl.rocks community when an agent joins or leaves.
// Consumed by the CommunitySync function.
type communityNotice struct {
	Community string           `json:"community"`
	Action    string           `json:"action"` // "onJoin", etc.
	User      agent            `json:"user"`
	TGId      model.TelegramID `json:"tg_id"`
	TGName    string           `json:"tg_user"`
}

// communityResponse is returned from a manual query to an enl.rocks community.
type communityResponse struct {
	Community  string           `json:"community"`
	Title      string           `json:"title"`
	Members    []model.GoogleID `json:"members"`    // List of GoogleIDs
	Moderators []model.GoogleID `json:"moderators"` // List of GoogleIDs
	User       agent            `json:"user"`       // Present if querying a specific user
	Error      string           `json:"error"`
}

// agent represents the simplified agent data sent by enl.rocks.
// Note: The field tags match the Enl.Rocks API response keys.
type agent struct {
	Gid      model.GoogleID `json:"gid"`
	TGId     int64          `json:"tgid"`
	Agent    string         `json:"agentid"` // This is the Ingress Name (EnlightenedID)
	Verified bool           `json:"verified"`
	Smurf    bool           `json:"smurf"`
}

// rocksPushResponse is the standard response body for POST/DELETE actions.
type rocksPushResponse struct {
	Error   string `json:"error"`
	Success bool   `json:"success"`
}
