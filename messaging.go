package WASABI

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"text/template"
)

type messagingConfig struct {
	inited       bool
	senders      map[string]func(gid GoogleID, message string) (bool, error)
	templateSet  map[string]*template.Template
	frontendPath string
}

var mc messagingConfig

// SendMessage sends a message to the available message destinations for agent specified by "gid"
// currently only Telegram is supported, but more can be added
func (gid GoogleID) SendMessage(message string) (bool, error) {
	// determine which messaging protocols are enabled for gid
	// pick optimal
	bus := "Telegram"

	// XXX loop through valid, trying until one works
	ok, err := gid.SendMessageVia(message, bus)
	if err != nil {
		Log.Notice("Unable to send message")
		return false, err
	}
	if ok == false {
		err = fmt.Errorf("Unable to send message")
		return false, err
	}
	return true, nil
}

// SendMessageTemplate sends a message using a template to determine the content
func (gid GoogleID) SendMessageTemplate(name string, lang string, in interface{}) (bool, error) {
	_, ok := mc.templateSet[lang]
	if ok == false {
		lang = "en"
	}

	var tpBuffer bytes.Buffer
	if err := mc.templateSet[lang].ExecuteTemplate(&tpBuffer, name, in); err != nil {
		Log.Notice(err)
		return false, err
	}

	return gid.SendMessage(tpBuffer.String())
}

// SendMessageVia sends a message to the destination on the specified bus
func (gid GoogleID) SendMessageVia(message, bus string) (bool, error) {
	_, ok := mc.senders[bus]
	if ok == false {
		err := fmt.Errorf("No such messaging bus: [%s]", bus)
		return false, err
	}

	ok, err := mc.senders[bus](gid, message)
	if err != nil {
		Log.Notice(err)
		return false, err
	}
	return ok, nil
}

// SendAnnounce sends a message to everyone on the team, determining what is the best route per agent
func (teamID TeamID) SendAnnounce(message string) error {
	// for each agent on the team
	// determine which messaging protocols are enabled for gid
	// pick optimal

	// ok, err := SendMessage(gid, message)
	return nil
}

// RegisterMessageBus registers a function used to send messages by various protocols
func RegisterMessageBus(name string, f func(GoogleID, string) (bool, error)) error {
	mc.senders[name] = f
	return nil
}

// called at server start to init the configuration
func init() {
	mc.senders = make(map[string]func(GoogleID, string) (bool, error))
	mc.inited = true
	mc.frontendPath = "frontend" // XXX don't hardcode this
	messagingTemplates()
}

// set up the templates
func messagingTemplates() error {
	if mc.frontendPath == "" {
		err := errors.New("frontendPath not configured")
		Log.Critical(err)
		return err
	}

	frontendPath, err := filepath.Abs(mc.frontendPath)
	if err != nil {
		Log.Critical("Frontend path couldn't be resolved.")
		panic(err)
	}
	mc.frontendPath = frontendPath

	funcMap := template.FuncMap{
		"TGGetBotName": TGGetBotName,
		"TGGetBotID":   TGGetBotID,
		"TGRunning":    TGRunning,
		"Webroot":      GetWebroot,
		"WebAPIPath":   GetWebAPIPath,
		"VEnlOne":      GetvEnlOne,
	}
	mc.templateSet = make(map[string]*template.Template)

	if err != nil {
		Log.Error(err)
	}
	Log.Info("Including messaging templates from: ", mc.frontendPath)
	files, err := ioutil.ReadDir(mc.frontendPath)
	if err != nil {
		Log.Error(err)
	}

	for _, f := range files {
		lang := f.Name()
		if f.IsDir() && len(lang) == 2 {
			mc.templateSet[lang] = template.New("").Funcs(funcMap) // one funcMap for all languages
			// load the masters
			mc.templateSet[lang].ParseGlob(mc.frontendPath + "/master/*.msg")
			// overwrite with language specific
			mc.templateSet[lang].ParseGlob(mc.frontendPath + "/" + lang + "/*.msg")
			Log.Debugf("Templates for lang [%s] %s", lang, mc.templateSet[lang].DefinedTemplates())
		}
	}

	return nil
}
