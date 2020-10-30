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

func randomWord(array []string, try int) string {
	if try > 10 {
		Log.Fatalw("crypto/rand failed 10 times, giving up")
		return ""
	}

	i, err := rand.Int(rand.Reader, big.NewInt(int64(len(array))))
	if err != nil {
		Log.Error(err)
		return randomWord(array, try+1)
	}
	return array[i.Int64()]
}

// GenerateID generates a random ASCII-safe string of specified length
func GenerateID(size int) string {
	var buf = make([]byte, size)

	for i := 0; i < size; i++ {
		r, err := rand.Int(rand.Reader, big.NewInt(int64(len(characters))))
		if err != nil {
			Log.Error(err)
		}
		b := []byte(characters[r.Int64()])
		buf[i] = b[0]
	}
	return string(buf)
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
	return nil
}

// GenerateName generates a slug in the format "cornflake-peddling-bp0q".
func GenerateName() string {
	first := randomWord(words, 0)
	second := randomWord(words, 0)
	third := GenerateID(5)

	return fmt.Sprintf("%s-%s-%s", first, second, third)
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
