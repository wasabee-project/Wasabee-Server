package wasabigm

import (
	"bytes"

	wasabi "github.com/cloudkucooland/WASABI"
)

// XXX this is not correct
func gmTemplateExecute(name, lang string, data interface{}) (string, error) {
	lang = "en"

	var tpBuffer bytes.Buffer
	if err := config.TemplateSet[lang].ExecuteTemplate(&tpBuffer, name, data); err != nil {
		wasabi.Log.Notice(err)
		return "", err
	}
	return tpBuffer.String(), nil
}
