package db

import (
	"time"

	"github.com/ethereum/go-ethereum/common"
	whisper "github.com/status-im/whisper/whisperv6"
)

// HistoryStore provides utility methods for quering history and requests store.
type HistoryStore struct {
	topicDB   Interface
	requestDB Interface
}

func (h HistoryStore) GetHistory(topic whisper.TopicType, duration time.Duration) (TopicHistory, error) {
	thist := TopicHistory{db: h.topicDB, Duration: duration, Topic: topic}
	return thist, thist.Load()
}

func (h HistoryStore) NewRequest() HistoryRequest {
	return HistoryRequest{db: h.requestDB}
}

func (h HistoryStore) GetRequest(id common.Hash) (HistoryRequest, error) {
	req := HistoryRequest{db: h.requestDB, ID: id}
	return req, req.Load()
}

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
