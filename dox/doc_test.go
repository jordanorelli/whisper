package dox

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"testing"
)

// things that we shouldn't be allowed to encrypt
var doNotEncrypt = []interface{}{
	0,
	1,
	1.1,
	true,
	false,
	"a string",
	[]byte("a byte slice"),
	struct{ x, y int }{5, 10},
}

func TestAes(t *testing.T) {
	key, err := randKey()
	if err != nil {
		t.Errorf("unable to create aes key for testing: %v", err)
	}
	t.Logf("aes key for testing: %x\n", key)

	plaintext, err := randslice(512)
	if err != nil {
		t.Errorf("unable to create random slice for testing: %v", err)
	}
	t.Logf("plaintext: %x\n", plaintext)

	ciphertext, err := aesEncrypt(key, plaintext)
	if err != nil {
		t.Error(err)
	}
	t.Logf("ciphertext: %x\n", ciphertext)

	if bytes.Equal(plaintext, ciphertext) {
		t.Error("plaintext and ciphertext bytes are the same! nothing changed!")
	}

	plaintext2, err := aesDecrypt(key, ciphertext)
	if err != nil {
		t.Errorf("unable to aes decrypt: %v", err)
	}
	if !bytes.Equal(plaintext, plaintext2) {
		t.Errorf("aes decryption output bytes do not match input bytes!\ninput: %x\noutput: %x\n", plaintext, plaintext2)
	}
	t.Logf("plaintext2: %x\n", plaintext2)
}

func TestEncrypt(t *testing.T) {
	keysize := 1024
	t.Logf("generating %d-bit rsa key", keysize)
	rsaKeyAlice, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatal("unable to generate key to run tests: %v", err)
	}
	// rsaKeyBob, err := rsa.GenerateKey(rand.Reader, 1024)
	// if err != nil {
	// 	t.Fatal("unable to generate key to run tests: %v", err)
	// }

	for _, v := range doNotEncrypt {
		_, err := Encrypt(&rsaKeyAlice.PublicKey, v)
		if err == nil {
			t.Errorf("encrypting non-pointers should result in an error")
		}
	}

	type Person1 struct {
		Name string
	}
	p1 := &Person1{Name: "jordan"}
	doc1, err := Encrypt(&rsaKeyAlice.PublicKey, p1)
	if err != nil {
		t.Errorf("failed to encrypt person: %v", err)
	} else {
		t.Logf("doc1: %v", doc1)
	}

	for _, v := range doNotEncrypt {
		err := doc1.Decrypt(rsaKeyAlice, v)
		if err == nil {
			t.Errorf("decrypting into non-pointers should result in an error")
		}
	}

	var p1_2 Person1
	if err := doc1.Decrypt(rsaKeyAlice, &p1_2); err != nil {
		t.Error(err)
	} else {
		t.Logf("doc1 decrypted: %v", p1_2)
	}

	b1, err := EncryptJSON(&rsaKeyAlice.PublicKey, p1)
	if err != nil {
		t.Errorf("failed to encrypt person: %v", err)
	} else {
		t.Logf("doc1 json: %v", string(b1))
	}

	type Person2 struct {
		Name string `dox:"plaintext"`
	}
	p2 := &Person2{Name: "jordan"}
	doc2, err := Encrypt(&rsaKeyAlice.PublicKey, p2)
	if err != nil {
		t.Errorf("failed to encrypt person: %v", err)
	} else {
		t.Logf("%v", doc2)
	}

	var p2_2 Person2
	if err := doc2.Decrypt(rsaKeyAlice, &p2_2); err != nil {
		t.Error(err)
	} else {
		t.Logf("doc2 decrypted: %v", p2_2)
	}

	b2, err := EncryptJSON(&rsaKeyAlice.PublicKey, p2)
	if err != nil {
		t.Errorf("failed to encrypt person: %v", err)
	} else {
		t.Logf("%v", string(b2))
	}

	type Person3 struct {
		Name string `dox:"aes"`
	}
	p3 := &Person3{Name: "jordan"}
	doc3, err := Encrypt(&rsaKeyAlice.PublicKey, p3)
	if err != nil {
		t.Errorf("failed to encrypt person: %v", err)
	} else {
		t.Logf("doc3: %v", doc3)
	}

	var p3_2 Person3
	if err := doc3.Decrypt(rsaKeyAlice, &p3_2); err != nil {
		t.Error(err)
	} else {
		t.Logf("doc3 decrypted: %v", p3_2)
	}

	b3, err := EncryptJSON(&rsaKeyAlice.PublicKey, p3)
	if err != nil {
		t.Errorf("failed to encrypt person: %v", err)
	} else {
		t.Logf("%v", string(b3))
	}

	var p3_3 Person3
	if err := DecryptJSON(rsaKeyAlice, b3, &p3_3); err != nil {
		t.Error(err)
	} else {
		t.Logf("doc3 json decrypted: %v", p3_3)
	}
}
