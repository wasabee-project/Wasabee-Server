package wasabee

import (
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"strings"
)

var words = []string{}
var characters = strings.Split("abcdefghijklmnopqrstuvwxyz0123456789", "")

func randomWord(array []string, try int, err error) string {
	if try > 10 {
		// We somehow tried 10 times to get a random value and it failed every time. Something is extremely wrong here.
		Log.Fatalw("crypto/rand issue - something is probably extremely wrong!", "error", err)
		// panic(err)
	}

	i, err := rand.Int(rand.Reader, big.NewInt(int64(len(array))))
	if err != nil {
		return randomWord(array, try+1, err)
	}
	return array[i.Int64()]
}

// LoadWordsFile imports the word definition file used for names.
func LoadWordsFile(filename string) error {
	// #nosec
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		Log.Error(err)
		return err
	}

	err = loadWords(content)
	if err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

// LoadWordsStream reads the words file from a stream such as Google Cloud Storage
func LoadWordsStream(r io.Reader) error {
	content, err := ioutil.ReadAll(r)
	if err != nil {
		Log.Error(err)
		return err
	}

	err = loadWords(content)
	if err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

func loadWords(content []byte) error {
	source := strings.Split(string(content), "\n")
	words = make([]string, 0)
	for _, word := range source {
		word = strings.ToLower(strings.TrimSpace(word))
		if len(word) > 0 && !strings.HasPrefix(word, "#") {
			words = append(words, word)
		}
	}
	if len(words) == 0 {
		err := fmt.Errorf("empty word list")
		return err
	}
	// Log.Debugw("startup", "words loaded", len(words))
	return nil
}

// GenerateName generates a slug in the format "cornflake-peddling-bp0q". Note that this function will return an empty string if an error occurs along the way!
func GenerateName() string {
	text := ""
	var word string
	for i := 0; i < 6; i++ {
		if i < 2 && len(words) > 0 {
			word = randomWord(words, 0, nil)
		} else {
			word = randomWord(characters, 0, nil)
		}

		if i < 3 && len(words) > 0 {
			text += "-" + word
		} else {
			text += word
		}
	}

	return strings.TrimPrefix(text, "-")
}

// GenerateSafeName generates a slug (like GenerateName()) that doesn't exist in the database yet.
func GenerateSafeName() (string, error) {
	name := ""
	rows := 1

	for rows > 0 {
		var i, total int
		name = GenerateName()
		if name == "" {
			err := fmt.Errorf("name generation failed")
			return "", err
		}
		err := db.QueryRow("SELECT COUNT(lockey) FROM agent WHERE lockey = ?", name).Scan(&i)
		if err != nil {
			return "", err
		}
		total = i
		err = db.QueryRow("SELECT COUNT(teamID) FROM team WHERE teamID = ?", name).Scan(&i)
		if err != nil {
			return "", err
		}
		total += i
		err = db.QueryRow("SELECT COUNT(joinLinkToken) FROM team WHERE joinLinkToken = ?", name).Scan(&i)
		if err != nil {
			return "", err
		}
		total += i
		rows = total
	}

	return name, nil
}
