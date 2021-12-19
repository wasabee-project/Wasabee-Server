package wtg

import (
	"bytes"
	"fmt"

	"github.com/wasabee-project/Wasabee-Server/log"
)

func templateExecute(name, lang string, data interface{}) (string, error) {
	if lang == "" {
		lang = "en"
	}

	_, ok := c.TemplateSet[lang]
	if !ok {
		lang = "en" // default to english if the map doesn't exist
		_, ok = c.TemplateSet[lang]
		if !ok {
			err := fmt.Errorf("invalid template set (no English)")
			return "", err
		}
	}

	var tpBuffer bytes.Buffer
	if err := c.TemplateSet[lang].ExecuteTemplate(&tpBuffer, name, data); err != nil {
		log.Error(err)
		return "", err
	}
	return tpBuffer.String(), nil
}
