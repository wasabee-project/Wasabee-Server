package wasabeetelegram

var tgbotname string
var tgbotid int
var tgrunning bool

// TGSetBot is called from the Telegram bot startup to let other services know it is running
func TGSetBot(botname string, botid int) {
	tgbotname = botname
	tgbotid = botid
	tgrunning = true
}

// TGGetBotName returns the bot's telegram username
// used by templates
func TGGetBotName() (string, error) {
	if !tgrunning {
		return "", nil
	}
	return tgbotname, nil
}

// TGGetBotID returns the bot's telegram ID number
// used by templates
func TGGetBotID() (int, error) {
	if !tgrunning {
		return 0, nil
	}
	return tgbotid, nil
}

// TGRunning is used by templates to determine if they should display telegram info
func TGRunning() (bool, error) {
	return tgrunning, nil
}
