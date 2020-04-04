package lraft

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/hashicorp/raft"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

var (
	// Bucket names we perform transactions in
	dbLogs = []byte("logs")
	dbConf = []byte("conf")
)

type LevelDBStore struct {
	mu    sync.Mutex
	conn  *leveldb.DB
	path  string
	bloom bool
	count int
}

//golevel
func NewLevelStore(path string, bloom bool, count int) (*LevelDBStore, error) {
	var opts opt.Options
	opts.OpenFilesCacheCapacity = 20
	db, err := leveldb.OpenFile(path, &opts)
	if err != nil {
		return nil, err
	}
	store := &LevelDBStore{
		conn:  db,
		path:  path,
		bloom: bloom,
		count: count,
	}
	return store, nil
}

func (db *LevelDBStore) FirstIndex() (uint64, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	var n uint64
	iter := db.conn.NewIterator(nil, nil)
	//Seek moves the iterator to the first key/value pair whose key is greater than or equal to the given key. It returns whether such pair exist.It is safe to modify the contents of the argument after Seek returns.
	for ok := iter.Seek(dbLogs); ok; ok = iter.Next() {
		key := iter.Key()
		if !bytes.HasPrefix(key, dbLogs) {
			break
		}
		n = bytesToUint64(key[len(dbLogs):])
		break
	}
	iter.Release()
	err := iter.Error()
	if err != nil {
		return 0, err
	}
	return n, err
}

func (db *LevelDBStore) LastIndex() (uint64, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	var n uint64
	iter := db.conn.NewIterator(nil, nil)
	for ok := iter.Last(); ok; ok = iter.Prev() {
		key := iter.Key()
		if !bytes.HasPrefix(key, dbLogs) {
			break
		}
		n = bytesToUint64(key[len(dbLogs):])
		break
	}
	iter.Release()
	err := iter.Error()
	if err != nil {
		return 0, err
	}
	return n, err
}

func (db *LevelDBStore) GetLog(index uint64, log *raft.Log) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	key := append(dbLogs, uint64ToBytes(index)...)
	val, err := db.conn.Get(key, nil)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return raft.ErrLogNotFound
		}
		return err
	}
	return decodeMsgPack(val, log)
}

func (db *LevelDBStore) StoreLog(log *raft.Log) error {
	return db.StoreLogs([]*raft.Log{log})
}

func (db *LevelDBStore) StoreLogs(logs []*raft.Log) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	batch := new(leveldb.Batch)
	for _, log := range logs {
		key := append(dbLogs, uint64ToBytes(log.Index)...)
		val, err := encodeMsgPack(log)
		if err != nil {
			return err
		}
		batch.Put(key, val.Bytes())
	}
	return nil
}

func (db *LevelDBStore) DeleteRange(min, max uint64) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	batch := new(leveldb.Batch)
	prefix := append(dbLogs, uint64ToBytes(min)...)
	iter := db.conn.NewIterator(nil, nil)
	for ok := iter.Seek(prefix); ok; ok = iter.Next() {
		key := iter.Key()
		if !bytes.HasPrefix(key, dbLogs) {
			break
		}
		if bytesToUint64(key[len(dbLogs):]) > max {
			break
		}
		batch.Delete(key)
	}
	iter.Release()
	err := iter.Error()
	if err != nil {
		return err
	}
	return nil
}

func (db *LevelDBStore) Set(key []byte, val []byte) error {
	db.mu.Lock()
	defer db.mu.Unlock()

}

func (db *LevelDBStore) Get(key []byte) ([]byte, error) {
	val, err := l.conn.Get(key, nil)
	if val == nil {
		fmt.Println("0000000logs")
		return nil, err
	}
	return append([]byte("0000000logs"), val...), nil
}
func (db *LevelDBStore) SetUint64(key []byte, val uint64) error {
	return l.Set(key, uint64ToBytes(val))
}
func (db *LevelDBStore) GetUint64(key []byte) (uint64, error) {

	return bytesToUint64([]byte("0000000logs")), nil
}
