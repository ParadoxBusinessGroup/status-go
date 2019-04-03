package shhext

import (
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/status-im/status-go/db"
	"github.com/status-im/status-go/mailserver"
	whisper "github.com/status-im/whisper/whisperv6"
)

const (
	// WhisperTimeAllowance is needed to ensure that we won't miss envelopes that were
	// delivered to mail server after we made a request.
	WhisperTimeAllowance = 20 * time.Second
)

// NewHistoryUpdateTracker creates HistoryUpdateTracker instance.
func NewHistoryUpdateTracker(store db.HistoryStore) HistoryUpdateTracker {
	return HistoryUpdateTracker{
		store: store,
	}
}

// HistoryUpdateTracker responsible for tracking progress for all history requests.
// It listens for 2 events:
//    - when envelope from mail server is received we will update appropriate topic on disk
//    - when confirmation for request completion is received - we will set last envelope timestamp as the last timestamp
//      for all TopicLists in current request.
type HistoryUpdateTracker struct {
	mu    sync.Mutex
	store db.HistoryStore
}

// EventRequestFinished is a place holder for actual event.
type EventRequestFinished struct {
	ID common.Hash
}

// once request finished we need to update timestamp for all topics, so that in next request they all will start
// from same timestamp.
func (tracker *HistoryUpdateTracker) handleEventRequestFinished(event EventRequestFinished) error {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	req, err := tracker.store.GetRequest(event.ID)
	if err != nil {
		return err
	}
	histories := req.Histories()
	for i := range histories {
		history := histories[i]
		history.Current = history.End
	}
	err = req.Save()
	if err != nil {
		return err
	}
	return req.Delete()
}

// EventTopicHistoryUpdate placeholder for event with a topic update.
type EventTopicHistoryUpdate struct {
	Topic     whisper.TopicType
	Timestamp time.Time
}

// get all topics that start from given Topic update Timestamp if it is higher than Current
func (tracker *HistoryUpdateTracker) handleEventTopicHistoryUpdate(event EventTopicHistoryUpdate) error {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	histories, err := tracker.store.GetHistoriesByTopic(event.Topic)
	if err != nil {
		return err
	}
	if len(histories) == 0 {
		return fmt.Errorf("no histories for topic 0x%x", event.Topic)
	}
	// TODO support multiple history ranges per topic
	th := histories[0]
	if event.Timestamp.After(th.Current) {
		th.Current = event.Timestamp
	}
	return th.Save()
}

// TopicRequest defines what user has to provide.
type TopicRequest struct {
	Topic    whisper.TopicType
	Duration time.Duration
}

// CreateRequests receives list of topic with desired timestamps and initiates both pending requests and requests
// that cover new topics.
func (tracker *HistoryUpdateTracker) CreateRequests(topicRequests []TopicRequest) ([]db.HistoryRequest, error) {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	topics := map[whisper.TopicType]db.TopicHistory{}
	for i := range topicRequests {
		topic, err := tracker.store.GetHistory(topicRequests[i].Topic, topicRequests[i].Duration)
		if err != nil {
			return nil, err
		}
		topics[topicRequests[i].Topic] = topic
	}
	requests, err := tracker.store.GetAllRequests()
	if err != nil {
		return nil, err
	}
	// removing topcs that are already covered by any request
	for i := range requests {
		for _, t := range topics {
			if requests[i].Includes(t) {
				delete(topics, t.Topic)
			}
		}
	}
	// TODO exclude in-flight requests from 'requests' list
	requests = append(requests, createRequestsFromTopicHistories(tracker.store, mapToList(topics))...)
	// TODO get NTP synced timestamps
	return RenewRequests(requests, time.Now()), nil
}

// RenewRequests re-sets current, first and end timestamps.
// Changes should not be persisted on disk in this method.
func RenewRequests(requests []db.HistoryRequest, now time.Time) []db.HistoryRequest {
	zero := time.Time{}
	// TODO inject our ntp-synced time
	for i := range requests {
		req := requests[i]
		histories := req.Histories()
		for j := range histories {
			history := &histories[j]
			if history.Current == zero {
				history.Current = now.Add(-(history.Duration))
			}
			if history.First == zero {
				history.First = history.Current
			}
			history.End = now
		}
	}
	return requests
}

// CreateTopicOptionsFromRequest transforms histories attached to a single request to a simpler format - TopicOptions.
func CreateTopicOptionsFromRequest(req db.HistoryRequest) TopicOptions {
	histories := req.Histories()
	rst := make(TopicOptions, len(histories))
	for i := range histories {
		history := histories[i]
		rst[i] = TopicOption{
			Topic: history.Topic,
			Range: Range{
				Start: uint64(history.Current.Add(-(WhisperTimeAllowance)).UnixNano()),
				End:   uint64(history.End.UnixNano()),
			},
		}
	}
	return rst
}

func mapToList(topics map[whisper.TopicType]db.TopicHistory) []db.TopicHistory {
	rst := make([]db.TopicHistory, 0, len(topics))
	for key := range topics {
		rst = append(rst, topics[key])
	}
	return rst
}

func createRequestsFromTopicHistories(store db.HistoryStore, histories []db.TopicHistory) []db.HistoryRequest {
	requests := []db.HistoryRequest{}
	for _, th := range histories {
		if len(requests) == 0 {
			requests = append(requests, store.NewRequest())
		}
		for i := range requests {
			req := &requests[i]
			histories := req.Histories()
			if len(histories) == 0 || histories[0].SameRange(th) {
				req.AddHistory(th)
			} else {
				req := store.NewRequest()
				req.AddHistory(th)
				requests = append(requests, req)
			}
		}
	}
	return requests
}

// Range of the request.
type Range struct {
	Start uint64
	End   uint64
}

// TopicOption request for a single topic.
type TopicOption struct {
	Topic whisper.TopicType
	Range Range
}

// TopicOptions is a list of topic-based requsts.
type TopicOptions []TopicOption

// ToBloomFilterOption creates bloom filter request from a list of topics.
func (options TopicOptions) ToBloomFilterOption() BloomFilterOption {
	topics := make([]whisper.TopicType, len(options))
	var start, end uint64
	for i := range options {
		opt := options[i]
		topics[i] = opt.Topic
		if opt.Range.Start > start {
			start = opt.Range.Start
		}
		if opt.Range.End > end {
			end = opt.Range.End
		}
	}

	return BloomFilterOption{
		Range:  Range{Start: start, End: end},
		Filter: topicsToBloom(topics...),
	}
}

// Topics returns list of whisper TopicType attached to each TopicOption.
func (options TopicOptions) Topics() []whisper.TopicType {
	rst := make([]whisper.TopicType, len(options))
	for i := range options {
		rst[i] = options[i].Topic
	}
	return rst
}

// BloomFilterOption is a request based on bloom filter.
type BloomFilterOption struct {
	Range  Range
	Filter []byte
}

// ToMessagesRequestPayload creates mailserver.MessagesRequestPayload and encodes it to bytes using rlp.
func (filter BloomFilterOption) ToMessagesRequestPayload() ([]byte, error) {
	// TODO fix this conversion.
	// we start from time.Duration which is int64, then convert to uint64 for rlp-serilizability
	// why uint32 here? max uint32 is smaller than max int64
	payload := mailserver.MessagesRequestPayload{
		Lower: uint32(filter.Range.Start),
		Upper: uint32(filter.Range.End),
		Bloom: filter.Filter,
		// Client must tell the MailServer if it supports batch responses.
		// This can be removed in the future.
		Batch: true,
	}
	return rlp.EncodeToBytes(payload)
}
