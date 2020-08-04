package wasabeetelegram

import (
	"bytes"
	"fmt"

	"github.com/wasabee-project/Wasabee-Server"
)

func templateExecute(name, lang string, data interface{}) (string, error) {
	if lang == "" {
		lang = "en"
	}

	_, ok := config.TemplateSet[lang]
	if !ok {
		lang = "en" // default to english if the map doesn't exist
		_, ok = config.TemplateSet[lang]
		if !ok {
			err := fmt.Errorf("invalid template set (no English)")
			return "", err
		}
	}

	var tpBuffer bytes.Buffer
	if err := config.TemplateSet[lang].ExecuteTemplate(&tpBuffer, name, data); err != nil {
		wasabee.Log.Info(err)
		return "", err
	}
	return tpBuffer.String(), nil
}
