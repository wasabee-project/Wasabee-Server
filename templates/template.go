package templates

import (
	"bytes"
	"html/template"
	"os"
	"path"
	"path/filepath"

	"github.com/wasabee-project/Wasabee-Server/config"
	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
)

var ts map[string]*template.Template
var funcMap = template.FuncMap{
	"TelegramBotName": config.TelegramBotName,
	"TelegramBotID":   config.TelegramBotID,
	"Webroot":         config.GetWebroot,
	"WebAPIPath":      config.GetWebAPIPath,
	"WebUI":           config.GetWebUI,
	"IngressName":     model.IngressName,
}

// Start should be called once from main to establish the templates.
func Start(frontendPath string) error {
	fp, err := filepath.Abs(frontendPath)
	if err != nil {
		log.Fatalw("startup", "error", "template path could not be resolved.")
		panic(err)
	}

	templateSet := make(map[string]*template.Template)

	log.Debugw("startup", "frontend template directory", fp)
	files, err := os.ReadDir(fp)
	if err != nil {
		log.Error(err)
		return err
	}

	for _, f := range files {
		lang := f.Name()
		if f.IsDir() && len(lang) == 2 {
			templateSet[lang] = template.New("").Funcs(funcMap) // one funcMap for all languages
			// load the masters
			masterpath := path.Join(fp, "master", "*")
			_, err = templateSet[lang].ParseGlob(masterpath)
			if err != nil {
				log.Error(err)
			}
			// overwrite with language specific
			langpath := path.Join(fp, lang, "*")
			_, err = templateSet[lang].ParseGlob(langpath)
			if err != nil {
				log.Error(err)
			}
			// log.Debugw("startup", "language", lang, "templates", templateSet[lang].DefinedTemplates())
		}
	}

	ts = templateSet
	return nil
}

// Execute runs a template with the given data -- defaulting to English
func Execute(name string, data interface{}) (string, error) {
	return ExecuteLang(name, "en", data)
}

// ExecuteLang runs a given template in a specified language
func ExecuteLang(name, lang string, data interface{}) (string, error) {
	var tpBuffer bytes.Buffer

	if _, ok := ts[lang]; !ok {
		lang = "en"
	}

	if err := ts[lang].ExecuteTemplate(&tpBuffer, name, data); err != nil {
		log.Info(err)
		return "", err
	}
	return tpBuffer.String(), nil
}
