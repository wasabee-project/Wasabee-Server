package wasabi

import (
	"html/template"
	"io/ioutil"
	"path/filepath"
)

func TemplateConfig(frontendPath string) (map[string]*template.Template, error) {
	// Transform frontendPath to an absolute path
	fp, err := filepath.Abs(frontendPath)
	if err != nil {
		Log.Critical("frontend path could not be resolved.")
		panic(err)
	}

	templateSet := make(map[string]*template.Template)

	Log.Debugf("Loading Template function map")
	funcMap := template.FuncMap{
		"TGGetBotName": TGGetBotName,
		"TGGetBotID":   TGGetBotID,
		"TGRunning":    TGRunning,
		"Webroot":      GetWebroot,
		"WebAPIPath":   GetWebAPIPath,
		"VEnlOne":      GetvEnlOne,
		"EnlRocks":     GetEnlRocks,
		"TeamMenu":     TeamMenu,
	}

	Log.Info("Including frontend templates from: ", fp)
	files, err := ioutil.ReadDir(fp)
	if err != nil {
		Log.Error(err)
		return nil, err
	}

	for _, f := range files {
		lang := f.Name()
		if f.IsDir() && len(lang) == 2 {
			templateSet[lang] = template.New("").Funcs(funcMap) // one funcMap for all languages
			// load the masters
			_, err = templateSet[lang].ParseGlob(fp + "/master/*")
			if err != nil {
				Log.Error(err)
			}
			// overwrite with language specific
			_, err = templateSet[lang].ParseGlob(fp + "/" + lang + "/*")
			if err != nil {
				Log.Error(err)
			}
			Log.Debugf("Templates for lang [%s] %s", lang, templateSet[lang].DefinedTemplates())
		}
	}
	return templateSet, nil
}
