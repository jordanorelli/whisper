package main

import (
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"github.com/syndtr/goleveldb/leveldb"
	"sync"
)

var (
	openDBs    = make(map[string]userdb, 32)
	dbopenlock sync.Mutex
)

type userdb struct {
	*leveldb.DB
}

func (db *userdb) getPublicKey() (*rsa.PublicKey, error) {
	val, err := db.Get([]byte("public_key"), nil)
	if err != nil {
		return nil, fmt.Errorf("unable to get public key: %v", err)
	}

	var key rsa.PublicKey
	if err := json.Unmarshal(val, &key); err != nil {
		return nil, fmt.Errorf("unable to get public key: %v", err)
	}
	return &key, nil
}

func getUserDB(nick string) (*userdb, error) {
	if db, ok := openDBs[nick]; ok {
		return &db, nil
	}

	path := fmt.Sprintf("./%s.db", nick)
	conn, err := leveldb.OpenFile(path, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to open db file at %s: %v", path, err)
	}
	info_log.Printf("opened database file: %s", path)

	dbopenlock.Lock()
	defer dbopenlock.Unlock()

	db := userdb{conn}
	openDBs[nick] = db
	return &db, nil
}

func getUserKey(nick string) (*rsa.PublicKey, error) {
	db, err := getUserDB(nick)
	if err != nil {
		return nil, err
	}
	return db.getPublicKey()
}
