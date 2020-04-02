package lraft

import (
	"github.com/hashicorp/raft"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/filter"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

type LevelStore struct {
	conn *leveldb.DB
	path string
	//bloom filter

}

type Options struct {
	path  string
	bloom bool
	count int
}

var (
	db  *leveldb.DB
	err error
)

//golevel
func NewLevelStore(path string, bloom bool, count int) (*LevelStore, error) {
	if count > 0 {
		return New(Options{path: path, bloom: bloom, count: count})
	}

	return New(Options{path: path, bloom: bloom, count: 10})
}

func New(options Options) (*LevelStore, error) {
	if options.bloom {
		o := opt.Options{Filter: filter.NewBloomFilter(options.count)}
		db, err = leveldb.OpenFile(options.path, &o)
		return &LevelStore{conn: db}, err
	}
	db, err = leveldb.OpenFile(options.path, nil)
	return &LevelStore{conn: db}, err
}

func (l *LevelStore) FirstIndex() (uint64, error) {
	b, err := l.conn.Get([]byte("logs"), nil)
	if err != nil {
		return 0, err
	}
	return bytesToUint64(b), nil
}

func (l *LevelStore) LastIndex() (uint64, error) {
	b, err := l.conn.Get([]byte("logs"), nil)
	if err != nil {
		return 0, err
	}
	return bytesToUint64(b), nil
}

func (l *LevelStore) GetLog(index uint64, log *raft.Log) error {
	data, err := l.conn.Get(uint64ToBytes(index), nil)
	if data == nil {

		return err
	}
	return decodeMsgPack(data, log)
}

func (l *LevelStore) StoreLog(log *raft.Log) error {
	return l.StoreLogs([]*raft.Log{log})
}

func (l *LevelStore) StoreLogs(logs []*raft.Log) error {
	for _, log := range logs {
		key := uint64ToBytes(log.Index)
		val, err := encodeMsgPack(log)
		if err != nil {
			return err
		}
		if err := l.conn.Put(key, val.Bytes(), nil); err != nil {
			return err
		}
	}
	return nil
}

func (l *LevelStore) DeleteRange(min, max uint64) error {
	//minkey := uint64ToBytes(min)
	iter := l.conn.NewIterator(nil, nil)
	for iter.Next() {
		key := iter.Key()
		//value := iter.Value()
		if bytesToUint64(key) > max {
			break
		}
		if err := l.conn.Delete(key, nil); err != nil {
			return err
		}

	}
	iter.Release()
	err := iter.Error()
	return err
}

func (l *LevelStore) Set(key []byte, val []byte) error {
	return l.conn.Put(key, val, nil)
}

func (l *LevelStore) Get(key []byte) ([]byte, error) {
	return l.conn.Get(key, nil)
}
func (l *LevelStore) SetUint64(key []byte, val uint64) error {
	return l.Set(key, uint64ToBytes(val))
}
func (l *LevelStore) GetUint64(key []byte) (uint64, error) {
	val, err := l.Get(key)
	if err != nil {
		return 0, err
	}
	return bytesToUint64(val), nil
}
