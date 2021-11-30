package config

import (
	"github.com/gorilla/mux"
)

type WasabeeConf struct {
	V        bool
	Rocks    bool
	PubSub   bool
	Telegram bool
	HTTP     struct {
		Webroot string
		APIpath string
		Router  *mux.Router
	}
}

var c WasabeeConf

func Get() *WasabeeConf {
	return &c
}
