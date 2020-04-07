package lraft

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/hashicorp/raft"
)

const raftTimeOut = 10 * time.Second

var (
	ErrNotLear = errors.New("not leader")
)

type Store struct {
	RaftDir  string
	RaftBind string
	UseMem   bool

	mu     sync.Mutex
	mStore map[string]string // The key-value store for the system.

	raft        *raft.Raft // The consensus mechanism
	BloomFilter bool
	Count       int

	logger *log.Logger
}

type Command struct {
	Op    string `json:"op,omitempty"`
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
}

func (s *Store) Open(enableSingle bool, nodeID string) error {
	// Setup Raft configuration.
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(nodeID)

	// Setup Raft communication.
	addr, err := net.ResolveTCPAddr("tcp", s.RaftBind)
	if err != nil {
		return err
	}
	transport, err := raft.NewTCPTransport(s.RaftBind, addr, 3, 10*time.Second, os.Stderr)
	if err != nil {
		return err
	}

	// Create the snapshot store. This allows the Raft to truncate the log.
	snapshots, err := raft.NewFileSnapshotStore(s.RaftDir, 2, os.Stderr)
	if err != nil {
		return fmt.Errorf("file snapshot store: %s", err)
	}
	//Term
	// Create the log store and stable store.
	var logStore raft.LogStore
	var stableStore raft.StableStore
	if s.UseMem {
		logStore = raft.NewInmemStore()
		stableStore = raft.NewInmemStore()
	} else {
		//raftboltdb.NewBoltStore(filepath.Join(s.RaftDir, "raft.db"))
		leveldb, err := NewLevelStore(filepath.Join(s.RaftDir, ""), s.BloomFilter, s.Count)
		if err != nil {
			return fmt.Errorf("new bolt store: %s", err)
		}
		logStore = leveldb
		stableStore = leveldb
	}

	// Instantiate the Raft systems.
	ra, err := raft.NewRaft(config, (*fsm)(s), logStore, stableStore, snapshots, transport)
	if err != nil {
		return fmt.Errorf("new raft: %s", err)
	}
	s.raft = ra

	if enableSingle {
		configuration := raft.Configuration{
			Servers: []raft.Server{
				{
					ID:      config.LocalID,
					Address: transport.LocalAddr(),
				},
			},
		}
		ra.BootstrapCluster(configuration)
	}

	return nil
}

func (s *Store) Get(key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.mStore[key], nil
}

func (s *Store) Set(key, value string) error {
	if s.raft.State() != raft.Leader {
		return ErrNotLear
	}
	c := &Command{
		Op:    "set",
		Key:   key,
		Value: value,
	}
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	f := s.raft.Apply(b, raftTimeOut)
	return f.Error()
}

// Delete deletes the given key.
func (s *Store) Delete(key string) error {
	if s.raft.State() != raft.Leader {
		return ErrNotLear
	}

	c := &Command{
		Op:  "delete",
		Key: key,
	}
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}

	f := s.raft.Apply(b, raftTimeOut)
	return f.Error()
}

func (s *Store) Join(nodeID, addr string) error {
	s.logger.Printf("received join request for remote node %s at %s", nodeID, addr)
	configfuture := s.raft.GetConfiguration()
	if err := configfuture.Error(); err != nil {
		s.logger.Printf("failed to get raft configuration: %v", err)
		return err
	}
	for _, server := range configfuture.Configuration().Servers {
		if server.ID == raft.ServerID(nodeID) || server.Address == raft.ServerAddress(addr) {
			if server.Address == raft.ServerAddress(addr) && server.ID == raft.ServerID(nodeID) {
				s.logger.Printf("node %s at %s already member of cluster, ignoring join request", nodeID, addr)
				return nil
			}
			future := s.raft.RemoveServer(server.ID, 0, 0)
			if err := future.Error(); err != nil {
				return fmt.Errorf("error removing existing node %s at %s: %s", nodeID, addr, err)
			}
		}
	}
	f := s.raft.AddVoter(raft.ServerID(nodeID), raft.ServerAddress(addr), 1, raftTimeOut)
	if err := f.Error(); err != nil {
		return err
	}
	s.logger.Printf("node %s at %s joined successfully", nodeID, addr)
	return nil
}

type fsm Store

// Apply applies a Raft log entry to the key-value store.
func (f *fsm) Apply(l *raft.Log) interface{} {
	var c Command
	if err := json.Unmarshal(l.Data, &c); err != nil {
		panic(fmt.Sprintf("failed to unmarshal command: %s", err.Error()))
	}

	switch c.Op {
	case "set":
		return f.applySet(c.Key, c.Value)
	case "delete":
		return f.applyDelete(c.Key)
	default:
		panic(fmt.Sprintf("unrecognized command op: %s", c.Op))
	}
	return nil
}

// Snapshot returns a snapshot of the key-value store.
func (f *fsm) Snapshot() (raft.FSMSnapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Clone the map.
	o := make(map[string]string)
	for k, v := range f.mStore {
		o[k] = v
	}
	return &fsmSnapshot{store: o}, nil
}

// Restore stores the key-value store to a previous state.
func (f *fsm) Restore(rc io.ReadCloser) error {
	o := make(map[string]string)
	if err := json.NewDecoder(rc).Decode(&o); err != nil {
		return err
	}

	// Set the state from the snapshot, no lock required according to
	// Hashicorp docs.
	f.mStore = o
	return nil
}

func (f *fsm) applySet(key, value string) interface{} {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.mStore[key] = value
	return nil
}

func (f *fsm) applyDelete(key string) interface{} {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.mStore, key)
	return nil
}

type fsmSnapshot struct {
	store map[string]string
}

func (f *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	err := func() error {
		// Encode data.
		b, err := json.Marshal(f.store)
		if err != nil {
			return err
		}

		// Write data to sink.
		if _, err := sink.Write(b); err != nil {
			return err
		}

		// Close the sink.
		return sink.Close()
	}()

	if err != nil {
		sink.Cancel()
	}

	return err
}

func (f *fsmSnapshot) Release() {}
