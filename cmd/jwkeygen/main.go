package main

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"

	"github.com/lestrrat-go/jwx/jwk"

	"github.com/wasabee-project/Wasabee-Server/generatename"
)

func main() {
	raw, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		fmt.Printf("failed to generate new RSA private key: %s\n", err)
		return
	}

	key, err := jwk.New(raw)
	if err != nil {
		fmt.Printf("failed to create symmetric key: %s\n", err)
		return
	}
	if _, ok := key.(jwk.RSAPrivateKey); !ok {
		fmt.Printf("expected jwk.SymmetricKey, got %T\n", key)
		return
	}

	key.Set(jwk.KeyIDKey, generatename.GenerateID(16))

	buf, err := json.MarshalIndent(key, "", "  ")
	if err != nil {
		fmt.Printf("failed to marshal key into JSON: %s\n", err)
		return
	}
	fmt.Printf("%s\n", buf)

	pk, _ := jwk.PublicKeyOf(key)
	buf, err = json.MarshalIndent(pk, "", "  ")
	if err != nil {
		fmt.Printf("failed to marshal public key into JSON: %s\n", err)
		return
	}
	fmt.Printf("%s\n", buf)
}
