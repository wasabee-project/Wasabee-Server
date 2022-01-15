package model

// These error values are error strings visible to users, they need to be migrated to the translation system
const (
	ErrAgentNotFound        = "agent not registered with this wasabee server"
	ErrEmptyAgent           = "empty agent request"
	ErrGetLinkUnpopulated   = "attempt to use GetLink on unpopulated *Operation"
	ErrGetMarkerUnpopulated = "attempt to use GetMarker on unpopulated *Operation"
	ErrInvalidOTT           = "invalid OneTimeToken"
	ErrKeyUnableToRemove    = "unable to remove key count for portal"
	ErrKeyUnableToRecord    = "unable to record keys, ensure the op on the server is up-to-date"
	ErrLinkNotFound         = "link not found"
	ErrMarkerNotFound       = "markernot found"
	ErrOpNotFound           = "operation not found"
	ErrMultipleIntelname    = "multiple intelname matches found, not using intelname results"
	ErrMultipleRocks        = "multiple rocks matches found, not using rocks results"
	ErrMultipleV            = "multiple V matches found, not using V results"
	ErrNameGenFailed        = "name generation failed"
	ErrNotOnTeamAddPerm     = "you must be on a team to add it as a permission"
	ErrNotOpOwner           = "not owner of op"
	ErrPortalNotFound       = "portal not found"
	ErrTaskNotFound         = "task not found"
	ErrUnknownGID           = "unknown GoogleID"
	ErrUnknownPermType      = "unknown permission type"
	ErrUnknownUser          = "unknown user"
)
