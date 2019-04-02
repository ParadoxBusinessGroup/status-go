package db

import (
	"time"

	"github.com/ethereum/go-ethereum/common"
	whisper "github.com/status-im/whisper/whisperv6"
	"github.com/syndtr/goleveldb/leveldb/errors"
)

// HistoryStore provides utility methods for quering history and requests store.
type HistoryStore struct {
	topicDB   Interface
	requestDB Interface
}

// GetHistory creates history instance and loads history from database.
// Returns instance populated with topic and duration if history is not found in database.
func (h HistoryStore) GetHistory(topic whisper.TopicType, duration time.Duration) (TopicHistory, error) {
	thist := TopicHistory{db: h.topicDB, Duration: duration, Topic: topic}
	err := thist.Load()
	if err != nil && err != errors.ErrNotFound {
		return TopicHistory{}, err
	}
	return thist, nil
}

// NewRequest returns instance of the HistoryRequest.
func (h HistoryStore) NewRequest() HistoryRequest {
	return HistoryRequest{db: h.requestDB, topicDB: h.topicDB}
}

// GetRequest loads HistoryRequest from database.
func (h HistoryStore) GetRequest(id common.Hash) (HistoryRequest, error) {
	req := HistoryRequest{db: h.requestDB, topicDB: h.topicDB, ID: id}
	err := req.Load()
	if err != nil {
		return HistoryRequest{}, err
	}
	return req, nil
}

// GetAllRequests loads all not-finished history requests from database.
func (h HistoryStore) GetAllRequests() ([]HistoryRequest, error) {
	rst := []HistoryRequest{}
	iter := h.requestDB.NewIterator(h.requestDB.Range(nil, nil))
	for iter.Next() {
		req := HistoryRequest{db: h.requestDB}
		err := req.RawUnmarshall(iter.Value())
		if err != nil {
			return nil, err
		}
		rst = append(rst, req)
	}
	return rst, nil
}
