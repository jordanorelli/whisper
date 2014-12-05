package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

func generate() {
	priv, err := rsa.GenerateKey(rand.Reader, keyLength)
	if err != nil {
		exit(1, "couldn't generate private key: %v", err)
	}
	json.NewEncoder(os.Stdout).Encode(priv)
}

func encrypt() {
	key, err := publicKey()
	if err != nil {
		exit(1, "couldn't setup key: %v", err)
	}
	msg, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		exit(1, "error reading input message: %v", err)
	}
	ctxt, err := rsa.EncryptPKCS1v15(rand.Reader, key, msg)
	if err != nil {
		exit(1, "error encrypting message: %v", err)
	}

	var buf bytes.Buffer
	enc := base64.NewEncoder(base64.StdEncoding, &buf)
	if _, err := enc.Write(ctxt); err != nil {
		exit(1, "error b64 encoding ciphertext: %v", err)
	}
	enc.Close()

	b64 := buf.Bytes()
	os.Stdout.Write(b64)
	os.Stdout.Close()
}

func decrypt() {
	key, err := privateKey()
	if err != nil {
		exit(1, "couldn't setup key: %v", err)
	}

	raw, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		exit(1, "error reading input message: %v", err)
	}

	buf := bytes.NewBuffer(raw)
	decoder := base64.NewDecoder(base64.StdEncoding, buf)
	ctxt, err := ioutil.ReadAll(decoder)
	if err != nil {
		exit(1, "error reading b64 buffer %v", err)
	}

	msg, err := rsa.DecryptPKCS1v15(rand.Reader, key, ctxt)
	if err != nil {
		exit(1, "error decrypting message: %v", err)
	}
	fmt.Printf("%s", msg)
}

func publicKey() (*rsa.PublicKey, error) {
	if options.publicKey == "" {
		priv, err := privateKey()
		if err != nil {
			return nil, err
		}
		return &priv.PublicKey, nil
	}
	f, err := os.Open(options.publicKey)
	if err != nil {
		return nil, fmt.Errorf("unable to read public key from file %s: %v", options.publicKey, err)
	}
	defer f.Close()
	d1 := json.NewDecoder(f)
	var key rsa.PublicKey
	if err := d1.Decode(&key); err != nil {
		return nil, fmt.Errorf("unable to decode key from file %s: %v", options.key, err)
	}
	return &key, nil
}

func privateKey() (*rsa.PrivateKey, error) {
	f, err := os.Open(options.key)
	if err != nil {
		return nil, fmt.Errorf("unable to open private key file at %s: %v", options.key, err)
	}
	defer f.Close()

	d1 := json.NewDecoder(f)
	var key rsa.PrivateKey
	if err := d1.Decode(&key); err != nil {
		return nil, fmt.Errorf("unable to decode key from file %s: %v", options.key, err)
	}
	return &key, nil
}

func getPublic() {
	priv, err := privateKey()
	if err != nil {
		exit(1, "unable to read private key file: %v", err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(priv.PublicKey); err != nil {
		exit(1, "unable to marshal key: %v", err)
	}
}
