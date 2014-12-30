package main

import (
	"bytes"
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

// iterates through a range of values, starting with a prefix, parsing the
// lexnum part on each key, and calling the callback for each value with the
// value's associated number in its lexical series
func (db *userdb) collect(prefix []byte, n int, fn func(n int, v []byte) error) error {
	r := util.BytesPrefix(prefix)
	it := db.NewIterator(r, nil)
	defer it.Release()

	var step func() bool
	if n < 0 {
		if !it.Last() {
			return fmt.Errorf("collect unable to advance iterator to last")
		}
		step = it.Prev
		n = -n
	} else {
		step = it.Next
	}

	for i := 0; it.Valid() && i < n; i++ {
		id_s := string(bytes.TrimPrefix(it.Key(), prefix))
		id, err := decodeInt(id_s)
		if err != nil {
			return fmt.Errorf("unable to collect on prefix %s: %v", prefix, err)
		}
		if err := fn(id, it.Value()); err != nil {
			return fmt.Errorf("callback error in collect: %v", err)
		}
		step()
	}
	return nil
}

func getUserDB(nick string, create bool) (*userdb, error) {
	if db, ok := openDBs[nick]; ok {
		return &db, nil
	}

	opts := &opt.Options{
		ErrorIfMissing: !create,
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
