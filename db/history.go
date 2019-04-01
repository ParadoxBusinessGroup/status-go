package db

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"time"

	"github.com/ethereum/go-ethereum/common"
	whisper "github.com/status-im/whisper/whisperv6"
	"github.com/syndtr/goleveldb/leveldb/util"
)

var (
	// ErrEmptyKey returned if key is not expected to be empty.
	ErrEmptyKey = errors.New("TopicHistoryKey is empty")
)

// Interface is a common interface for DB operations.
type Interface interface {
	Get([]byte) ([]byte, error)
	Put([]byte, []byte) error
	Delete([]byte) error
	Range([]byte, []byte) *util.Range
	NewIterator(*util.Range) PrefixedIterator
}

// TopicHistoryKey defines bytes that are used as unique key for TopicHistory.
// first 4 bytes are whisper.TopicType bytes
// next 8 bytes are time.Duration encoded in big endian notation.
type TopicHistoryKey [8]byte

func (key TopicHistoryKey) ToEmptyTopicHistory() (th TopicHistory, err error) {
	if (key == TopicHistoryKey{}) {
		return th, ErrEmptyKey
	}
	topic := whisper.TopicType{}
	copy(topic[:], key[:4])
	duration := binary.BigEndian.Uint64(key[4:])
	return TopicHistory{Topic: topic, Duration: time.Duration(duration)}, nil
}

// TopicHistory stores necessary information.
type TopicHistory struct {
	db Interface

	// whisper topic
	Topic whisper.TopicType

	Duration time.Duration
	// Timestamp that was used for the first request with this topic.
	// Used to identify overlapping ranges.
	First time.Time
	// Timestamp of the last synced envelope.
	Current time.Time
}

// Key returns unique identifier for this TopicHistory.
func (t TopicHistory) Key() TopicHistoryKey {
	key := TopicHistoryKey{}
	copy(key[:], t.Topic[:])
	binary.BigEndian.PutUint64(key[4:], uint64(t.Duration))
	return key
}

func (t TopicHistory) Value() ([]byte, error) {
	return json.Marshal(t)
}

func (t *TopicHistory) Load() error {
	key := t.Key()
	if (key == TopicHistoryKey{}) {
		return errors.New("key is empty")
	}
	value, err := t.db.Get(key[:])
	if err != nil {
		return err
	}
	return json.Unmarshal(value, t)
}

func (t TopicHistory) Save() error {
	key := t.Key()
	val, err := t.Value()
	if err != nil {
		return err
	}
	return t.db.Put(key[:], val)
}

// SameRange returns true if topic has same range, which means:
// true if Current is zero and Duration is the same
// and true if Current is the same
func (t TopicHistory) SameRange(other TopicHistory) bool {
	zero := time.Time{}
	if t.Current == zero && other.Current == zero {
		return t.Duration == other.Duration
	}
	return t.Current == other.Current
}

// HistoryRequest is kept in the database while request is in the progress.
// Stores necessary information to identify topics with associated ranges included in the request.
type HistoryRequest struct {
	db Interface

	histories []TopicHistory

	// Generated ID
	ID common.Hash
	// List of the topics
	TopicHistoryKeys []TopicHistoryKey
}

func (req *HistoryRequest) AddHistory(history TopicHistory) {
	req.histories = append(req.histories, history)
	req.TopicHistoryKeys = append(req.TopicHistoryKeys, history.Key())
}

func (req *HistoryRequest) Histories() []TopicHistory {
	// TODO Lazy load from database on first access
	return req.histories
}

func (req HistoryRequest) Value() ([]byte, error) {
	return json.Marshal(req)
}

func (req HistoryRequest) Save() error {
	for _, h := range req.histories {
		if err := h.Save(); err != nil {
			return err
		}
	}
	val, err := req.Value()
	if err != nil {
		return err
	}
	return req.db.Put(req.ID.Bytes(), val)
}

func (req *HistoryRequest) Load() error {
	val, err := req.db.Get(req.ID.Bytes())
	if err != nil {
		return err
	}
	err = req.RawUnmarshall(val)
	if err != nil {
		return err
	}
	for _, hk := range req.TopicHistoryKeys {
		th, err := hk.ToEmptyTopicHistory()
		if err != nil {
			return err
		}
		err = th.Load()
		if err != nil {
			return err
		}
		req.histories = append(req.histories, th)
	}
	return nil
}

func (req *HistoryRequest) RawUnmarshall(val []byte) error {
	return json.Unmarshal(val, req)
}

func (req *HistoryRequest) Includes(history TopicHistory) bool {
	key := history.Key()
	for i := range req.TopicHistoryKeys {
		if key == req.TopicHistoryKeys[i] {
			return true
		}
	}
	return false
}
