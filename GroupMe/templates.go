package wasabeegm

import (
	"bytes"

	wasabee "github.com/wasabee-project/Wasabee-Server"
)

// XXX this is not correct
func gmTemplateExecute(name, lang string, data interface{}) (string, error) {
	var tpBuffer bytes.Buffer
	if err := config.TemplateSet[lang].ExecuteTemplate(&tpBuffer, name, data); err != nil {
		wasabee.Log.Notice(err)
		return "", err
	}
	return tpBuffer.String(), nil
}
