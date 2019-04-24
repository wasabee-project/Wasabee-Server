package wasabigm

import (
	"bytes"
	// "encoding/json"
	"errors"
	"github.com/cloudkucooland/WASABI"
	"io/ioutil"
	"path/filepath"
	"text/template"
)

func gmTemplates(t map[string]*template.Template) error {
	if config.FrontendPath == "" {
		err := errors.New("FrontendPath not configured")
		wasabi.Log.Critical(err)
		return err
	}

	frontendPath, err := filepath.Abs(config.FrontendPath)
	if err != nil {
		wasabi.Log.Critical("Frontend path couldn't be resolved.")
		panic(err)
	}
	config.FrontendPath = frontendPath

	wasabi.Log.Debugf("Building Template function map")
	funcMap := template.FuncMap{
		"TGGetBotName": wasabi.TGGetBotName,
		"TGGetBotID":   wasabi.TGGetBotID,
		"TGRunning":    wasabi.TGRunning,
		"Webroot":      wasabi.GetWebroot,
		"WebAPIPath":   wasabi.GetWebAPIPath,
		"VEnlOne":      wasabi.GetvEnlOne,
	}
	config.templateSet = make(map[string]*template.Template)

	if err != nil {
		wasabi.Log.Error(err)
	}
	wasabi.Log.Info("Including frontend telegram templates from: ", config.FrontendPath)
	files, err := ioutil.ReadDir(config.FrontendPath)
	if err != nil {
		wasabi.Log.Error(err)
	}

	for _, f := range files {
		lang := f.Name()
		if f.IsDir() && len(lang) == 2 {
			config.templateSet[lang] = template.New("").Funcs(funcMap) // one funcMap for all languages
			// load the masters
			config.templateSet[lang].ParseGlob(config.FrontendPath + "/master/*.gm")
			// overwrite with language specific
			config.templateSet[lang].ParseGlob(config.FrontendPath + "/" + lang + "/*.gm")
			wasabi.Log.Debugf("Templates for lang [%s] %s", lang, config.templateSet[lang].DefinedTemplates())
		}
	}

	return nil
}

func gmTemplateExecute(name, lang string, data interface{}) (string, error) {
	if lang == "" {
		lang = "en"
	}

	_, ok := config.templateSet[lang]
	if !ok {
		lang = "en" // default to english if the map doesn't exist
	}

	// s, _ := json.MarshalIndent(&data, "", "\t")
	// wasabi.Log.Debugf("Calling template %s[%s] with data %s", name, lang, string(s))
	var tpBuffer bytes.Buffer
	if err := config.templateSet[lang].ExecuteTemplate(&tpBuffer, name, data); err != nil {
		wasabi.Log.Notice(err)
		return "", err
	}
	return tpBuffer.String(), nil
}
