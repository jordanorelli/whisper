// package dox implements utilities for performing field-wise encryption of
// structured documents.
package dox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
)

// a Doc represents an encrypted document.  Document keys are left in
// plaintext, while document fields are encrypted or hashed as specified by
// their struct tags.
type Doc struct {
	Key    []byte                 `json:"key"`
	Fields map[string]interface{} `json:"fields"`
	Blob   []byte                 `json:"blob"`
}

func (d *Doc) Decrypt(key *rsa.PrivateKey, v interface{}) error {
	aesKey, err := rsa.DecryptPKCS1v15(rand.Reader, key, d.Key)
	if err != nil {
		return fmt.Errorf("failed to decrypt aes key: %v", err)
	}

	var blob []byte
	var blobvals map[string]interface{}

	if d.Blob != nil {
		blob, err = aesDecrypt(aesKey, d.Blob)
		if err != nil {
			return fmt.Errorf("failed to decrypt blob: %v", err)
		}
		if err := json.Unmarshal(blob, &blobvals); err != nil {
			return fmt.Errorf("unable to unmarshal blobvals: %v", err)
		}
	}

	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr {
		return fmt.Errorf("cannot Decrypt into non-pointer value of kind %v", rv.Kind())
	}

	rv = rv.Elem() // de-reference our pointer
	rt := rv.Type()

	for i := 0; i < rv.NumField(); i++ {
		fv := rv.Field(i)
		f := rt.Field(i)
		tag := f.Tag.Get("dox")
		switch tag {
		case "":
			val, ok := blobvals[f.Name]
			if !ok {
				return fmt.Errorf("doc blob is missing field %s", f.Name)
			}
			if !fv.CanSet() {
				return fmt.Errorf("cannot set field value %s", f.Name)
			}
			fv.Set(reflect.ValueOf(val))
		case "plaintext":
			val, ok := d.Fields[f.Name]
			if !ok {
				return fmt.Errorf("doc fields is missing field %s", f.Name)
			}
			if !fv.CanSet() {
				return fmt.Errorf("cannot set field value %s", f.Name)
			}
			fv.Set(reflect.ValueOf(val))
		case "aes":
			val, ok := d.Fields[f.Name]
			if !ok {
				return fmt.Errorf("doc fields is missing field %s", f.Name)
			}
			if !fv.CanSet() {
				return fmt.Errorf("cannot set field value %s", f.Name)
			}
			b, ok := val.([]byte)
			if !ok {
				s, ok := val.(string)
				if !ok {
					return fmt.Errorf("doc is corrupt")
				}
				b = make([]byte, len(s))
				n, err := base64.StdEncoding.Decode(b, []byte(s))
				if err != nil {
					return fmt.Errorf("couldn't base64 decode wtf %v", err)
				}
				b = b[:n]
			}
			rawVal, err := aesDecrypt(aesKey, b)
			if err != nil {
				return fmt.Errorf("couldn't decrypt field %s: %v", f.Name, err)
			}
			switch fv.Kind() {
			case reflect.String:
				fv.Set(reflect.ValueOf(string(rawVal)))
			case reflect.Slice:
				fv.Set(reflect.ValueOf(rawVal))
			default:
				panic("wtf")
			}
		default:
			return fmt.Errorf("not there yet stop it")
		}
	}
	return nil
}

func (d *Doc) setField(name string, v interface{}) {
	if d.Fields == nil {
		d.Fields = make(map[string]interface{}, 4)
	}
	d.Fields[name] = v
}

func DecryptJSON(key *rsa.PrivateKey, b []byte, v interface{}) error {
	var doc Doc
	if err := json.Unmarshal(b, &doc); err != nil {
		return err
	}
	return doc.Decrypt(key, v)
}

func EncryptJSON(key *rsa.PublicKey, v interface{}) ([]byte, error) {
	doc, err := Encrypt(key, v)
	if err != nil {
		return nil, err
	}

	b, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("dox.Encrypt unable to marshal doc: %v", err)
	}
	return b, nil
}

func Encrypt(key *rsa.PublicKey, v interface{}) (*Doc, error) {
	aesKey, err := randKey()
	if err != nil {
		return nil, fmt.Errorf("dox.Encrypt unable to generate document key: %v", err)
	}
	ckey, err := rsa.EncryptPKCS1v15(rand.Reader, key, aesKey)
	if err != nil {
		return nil, fmt.Errorf("dox.Encrypt unable to encrypt aes key: %v", err)
	}

	doc := &Doc{Key: ckey}

	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr {
		return nil, fmt.Errorf("dox.Encrypt received non-pointer value")
	}

	rv = rv.Elem() // dereference our pointer
	rt := rv.Type()
	// blobvals stores the values of the struct to be collected into a single
	// opaque blob.
	blobvals := make(map[string]interface{})
	for i := 0; i < rv.NumField(); i++ {
		fv := rv.Field(i)
		f := rt.Field(i)
		tag := f.Tag.Get("dox")
		switch tag {
		case "":
			blobvals[f.Name] = fv.Interface()
		case "plaintext":
			doc.setField(f.Name, fv.Interface())
		case "aes":
			switch value := fv.Interface().(type) {
			case string:
				cval, err := aesEncrypt(aesKey, []byte(value))
				if err != nil {
					return nil, fmt.Errorf("dox.Encrypt couldn't aes encrypt a field: %v", err)
				}
				doc.setField(f.Name, cval)
			case []byte:
				cval, err := aesEncrypt(aesKey, value)
				if err != nil {
					return nil, fmt.Errorf("dox.Encrypt couldn't aes encrypt a field: %v", err)
				}
				doc.setField(f.Name, cval)
			default:
				return nil, fmt.Errorf("dox.Encrypt can only aes encrypt fields of type string or []byte")
			}
		}
	}
	if len(blobvals) > 0 {
		blob, err := json.Marshal(blobvals)
		if err != nil {
			return nil, fmt.Errorf("dox.Encrypt unable to marshal blob fields: %v", err)
		}
		cipherblob, err := aesEncrypt(aesKey, blob)
		if err != nil {
			return nil, fmt.Errorf("dox.Encrypt failed to encrypt blob: %v", err)
		}
		doc.Blob = cipherblob
	}
	return doc, nil
}

func aesEncrypt(key []byte, ptxt []byte) ([]byte, error) {
	ptxt = append(ptxt, '|')
	if len(ptxt)%aes.BlockSize != 0 {
		pad := aes.BlockSize - len(ptxt)%aes.BlockSize
		for i := 0; i < pad; i++ {
			ptxt = append(ptxt, ' ')
		}
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("couldn't aes encrypt: failed to make aes cipher: %v", err)
	}

	ctxt := make([]byte, aes.BlockSize+len(ptxt))
	iv := ctxt[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, fmt.Errorf("couldn't encrypt note: failed to make aes iv: %v", err)
	}

	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ctxt[aes.BlockSize:], ptxt)
	return ctxt, nil
}

func aesDecrypt(key []byte, ctxt []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("unable to create aes cipher: %v", err)
	}
	iv := ctxt[:aes.BlockSize]

	ptxt := make([]byte, len(ctxt)-aes.BlockSize)
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(ptxt, ctxt[aes.BlockSize:])

	for i := len(ptxt) - 1; i >= 0; i-- {
		if ptxt[i] == '|' {
			return ptxt[:i], nil
		}
	}
	return ptxt, fmt.Errorf("unable to strip padding: %v", err)
}

func randslice(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func randKey() ([]byte, error) {
	return randslice(aes.BlockSize)
}
