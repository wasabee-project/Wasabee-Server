package templates

import (
	"bytes"
	"html/template"
	"io/ioutil"
	"path"
	"path/filepath"

	"github.com/wasabee-project/Wasabee-Server/log"
	// "github.com/wasabee-project/Wasabee-Server/http"
	"github.com/wasabee-project/Wasabee-Server/Telegram"
	"github.com/wasabee-project/Wasabee-Server/config"
)

// XXX this is a kludge
var ts map[string]*template.Template
var funcMap = template.FuncMap{
	"TGGetBotName": wasabeetelegram.TGGetBotName,
	"TGGetBotID":   wasabeetelegram.TGGetBotID,
	"Webroot":      config.GetWebroot,
	"WebAPIPath":   config.GetWebAPIPath,
}

// Startup should be called once from main to establish the templates.
// Individual subsystems should provide their own execution function since requirements will vary
// XXX TODO: establish a way of refreshing/reloading that doesn't leak
//
func Startup(frontendPath string) (map[string]*template.Template, error) {
	// Transform frontendPath to an absolute path
	fp, err := filepath.Abs(frontendPath)
	if err != nil {
		log.Fatalw("startup", "error", "frontend path could not be resolved.")
		// panic(err)
	}

	templateSet := make(map[string]*template.Template)

	log.Debugw("startup", "frontend template directory", fp)
	files, err := ioutil.ReadDir(fp)
	if err != nil {
		log.Error(err)
		return nil, err
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
			log.Debugw("startup", "language", lang, "templates", templateSet[lang].DefinedTemplates())
		}
	}
	ts = templateSet
	return templateSet, nil
}

func Execute(name string, data interface{}) (string, error) {
	lang := "en"

	var tpBuffer bytes.Buffer
	if err := ts[lang].ExecuteTemplate(&tpBuffer, name, data); err != nil {
		log.Info(err)
		return "", err
	}
	return tpBuffer.String(), nil
}
