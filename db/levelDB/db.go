package levelDB

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/syndtr/goleveldb/leveldb"
)

var gDBList map[string]*DB
var gMut sync.Mutex

type DB struct {
	key string
	db  *leveldb.DB
}

func GetDB(path string) *DB {
	if gDBList == nil {
		gDBList = make(map[string]*DB)
		gMut = sync.Mutex{}
	}

	gMut.Lock()
	defer gMut.Unlock()
	path = strings.ReplaceAll(path, "/", "_")

	if ldb, ok := gDBList[path]; !ok {
		db, err := leveldb.OpenFile(path, nil)
		if err != nil {
			fmt.Println(err)
			return nil
		} else {
			ldb = &DB{key: path, db: db}
			gDBList[path] = ldb
			return ldb
		}
	} else {
		return ldb
	}
}

func (d *DB) Put(key string, value interface{}) error {
	if jsonData, err := json.Marshal(value); err == nil {
		return d.db.Put([]byte(key), jsonData, nil)
	} else {
		return err
	}
}

func (d *DB) Get(key string) ([]byte, error) {
	data, err := d.db.Get([]byte(key), nil)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (d *DB) GetAll() ([][]byte, error) {
	result := make([][]byte, 0)
	iter := d.db.NewIterator(nil, nil)
	defer iter.Release()

	for ok := iter.First(); ok; ok = iter.Next() {
		// 현재 요소의 값 복사 및 결과에 추가
		value := make([]byte, len(iter.Value()))
		copy(value, iter.Value())
		result = append(result, value)
	}

	return result, nil
}

func (d *DB) Update(key string, value interface{}) error {
	return d.Put(key, value)
}

func (d *DB) Delete(key string) error {
	return d.db.Delete([]byte(key), nil)
}

func (d *DB) Close() error {
	gDBList[d.key] = nil
	return d.db.Close()
}

func CloseAll() {
	if len(gDBList) > 0 {
		for _, ldb := range gDBList {
			if ldb != nil {
				ldb.Close()
			}
		}
	}
}
