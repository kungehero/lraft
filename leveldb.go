package lraft

import (
	"bytes"
	"errors"
	"sync"

	"github.com/hashicorp/raft"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

var (
	//corrupt
	dbLogs = []byte("logs")
	dbConf = []byte("conf")

	//error log
	ErrKeyNotFound = errors.New("not found")
	Errclosed      = errors.New("closed")
)

type LevelDBStore struct {
	mu    sync.Mutex
	conn  *leveldb.DB
	batch *leveldb.Batch
	path  string
	bloom bool
	count int
}

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
	for _, log := range logs {
		key := append(dbLogs, uint64ToBytes(log.Index)...)
		val, err := encodeMsgPack(log)
		if err != nil {
			return err
		}
		db.batch.Put(key, val.Bytes())
	}
	return db.bathFlush()
}

func (db *LevelDBStore) bathFlush() error {
	opt := opt.WriteOptions{Sync: true}
	defer db.batch.Reset()
	if err := db.conn.Write(db.batch, &opt); err != nil {
		return err
	}
	return nil
}

func (db *LevelDBStore) DeleteRange(min, max uint64) error {
	db.mu.Lock()
	defer db.mu.Unlock()
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
		db.batch.Delete(key)
	}
	iter.Release()
	err := iter.Error()
	if err != nil {
		return err
	}
	return db.bathFlush()
}

func (db *LevelDBStore) Set(key []byte, val []byte) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	k := append(dbConf, key...)
	v := val
	db.batch.Put(k, v)
	return db.bathFlush()
}

func (db *LevelDBStore) Get(key []byte) ([]byte, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	val, err := db.conn.Get(append(dbConf, key...), nil)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return nil, errors.New("key not found")
		}
		return nil, err
	}
	return val, nil
}
func (db *LevelDBStore) SetUint64(key []byte, val uint64) error {
	return db.Set(key, uint64ToBytes(val))
}
func (db *LevelDBStore) GetUint64(key []byte) (uint64, error) {
	val, err := db.Get(key)
	if err != nil {
		return 0, err
	}
	return bytesToUint64(val), nil
}
