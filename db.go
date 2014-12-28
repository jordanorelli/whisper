package main

import (
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"
	"strings"
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

func (db *userdb) nextKey(prefix string) (string, error) {
	r := util.BytesPrefix([]byte(prefix))
	it := db.NewIterator(r, nil)
	defer it.Release()

	id := 0
	if it.Last() {
		key := it.Key()
		id_s := strings.TrimPrefix(string(key), prefix)
		lastId, err := decodeInt(id_s)
		if err != nil {
			return "0", fmt.Errorf("error getting note id: %v", err)
		}
		id = lastId + 1
	}
	return fmt.Sprintf("%s%s", prefix, encodeInt(id)), nil
}

func getUserDB(nick string, create bool) (*userdb, error) {
	if db, ok := openDBs[nick]; ok {
		return &db, nil
	}

	opts := &opt.Options{
		ErrorIfMissing: create,
	}
	path := fmt.Sprintf("./%s.db", nick)
	conn, err := leveldb.OpenFile(path, opts)
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
	db, err := getUserDB(nick, false)
	if err != nil {
		return nil, err
	}
	return db.getPublicKey()
}
