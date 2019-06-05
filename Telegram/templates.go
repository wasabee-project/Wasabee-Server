package wasabitelegram

import (
	"bytes"

	"github.com/cloudkucooland/WASABI"
)

func templateExecute(name, lang string, data interface{}) (string, error) {
	if lang == "" {
		lang = "en"
	}

	_, ok := config.TemplateSet[lang]
	if !ok {
		lang = "en" // default to english if the map doesn't exist
	}

	var tpBuffer bytes.Buffer
	if err := config.TemplateSet[lang].ExecuteTemplate(&tpBuffer, name, data); err != nil {
		wasabi.Log.Notice(err)
		return "", err
	}
	return tpBuffer.String(), nil
}
