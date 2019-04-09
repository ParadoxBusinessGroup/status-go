package shhext

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
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

// TimeSource is a function that returns current time.
type TimeSource func() time.Time

// NewHistoryUpdateTracker creates HistoryUpdateTracker instance.
func NewHistoryUpdateTracker(store db.HistoryStore, registry *RequestsRegistry, timeSource TimeSource) *HistoryUpdateTracker {
	return &HistoryUpdateTracker{
		store:      store,
		registry:   registry,
		timeSource: timeSource,
	}
}

// HistoryUpdateTracker responsible for tracking progress for all history requests.
// It listens for 2 events:
//    - when envelope from mail server is received we will update appropriate topic on disk
//    - when confirmation for request completion is received - we will set last envelope timestamp as the last timestamp
//      for all TopicLists in current request.
type HistoryUpdateTracker struct {
	mu         sync.Mutex
	store      db.HistoryStore
	registry   *RequestsRegistry
	timeSource TimeSource
}

// UpdateFinishedRequest removes succesfully finished request and updates every topic
// attached to the request.
func (tracker *HistoryUpdateTracker) UpdateFinishedRequest(id common.Hash, hash common.Hash) error {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	req, err := tracker.store.GetRequest(id)
	if err != nil {
		return err
	}
	// event was received once we processed all envelopes related to this event
	histories := req.Histories()
	for i := range histories {
		th := &histories[i]
		if th.LastEnvelopeHash == hash {
			return req.Delete()
		}
	}
	// if any of the topics already received LastEnvelopeHash we know that request is finished
	// and we can safely remove request. If it is not finished then we need to wait for that LastTimestamp.
	// and we do that in UpdateTopicHistory method
	req.LastEnvelopeHash = hash
	return req.Save()
}

// UpdateTopicHistory updates Current timestamp for the TopicHistory with a given timestamp.
func (tracker *HistoryUpdateTracker) UpdateTopicHistory(topic whisper.TopicType, timestamp time.Time, envelopeHash common.Hash) error {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	histories, err := tracker.store.GetHistoriesByTopic(topic)
	if err != nil {
		return err
	}
	if len(histories) == 0 {
		return fmt.Errorf("no histories for topic 0x%x", topic)
	}
	// TODO support multiple history ranges per topic
	th := histories[0]
	if timestamp.After(th.Current) {
		th.Current = timestamp
		th.LastEnvelopeHash = envelopeHash
	}
	err = th.Save()
	if err != nil {
		return err
	}
	// this case could happen only iff envelopes were delivered out of order
	// last envelope received, request completed, then others envelopes received
	// request complted, last envelope received, and then others envelopes received
	if (th.RequestID == common.Hash{}) {
		return nil
	}
	req, err := tracker.store.GetRequest(th.RequestID)
	if err != nil {
		return err
	}
	// we didn't receive event that request got finished yet
	if (req.LastEnvelopeHash == common.Hash{}) {
		return nil
	}
	// in this case we already processed event that request was finished
	if req.LastEnvelopeHash == envelopeHash {
		return req.Delete()
	}
	return nil
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
	filtered := []db.HistoryRequest{}
	for i := range requests {
		req := requests[i]
		for _, t := range topics {
			if req.Includes(t) {
				delete(topics, t.Topic)
			}
		}
		if !tracker.registry.Has(req.ID) {
			filtered = append(filtered, req)
		}
	}
	filtered = append(filtered, GroupHistoriesByRequestTimespan(tracker.store, mapToList(topics))...)
	return RenewRequests(filtered, tracker.timeSource()), nil
}

// RenewRequests re-sets current, first and end timestamps.
// Changes should not be persisted on disk in this method.
func RenewRequests(requests []db.HistoryRequest, now time.Time) []db.HistoryRequest {
	zero := time.Time{}
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
				Start: uint64(history.Current.Add(-(WhisperTimeAllowance)).Unix()),
				End:   uint64(history.End.Unix()),
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

// GroupHistoriesByRequestTimespan creates requests from provided histories.
// Multiple histories will be included into the same request only if they share timespan.
func GroupHistoriesByRequestTimespan(store db.HistoryStore, histories []db.TopicHistory) []db.HistoryRequest {
	requests := []db.HistoryRequest{}
	for _, th := range histories {
		var added bool
		for i := range requests {
			req := &requests[i]
			histories := req.Histories()
			if histories[0].SameRange(th) {
				req.AddHistory(th)
				added = true
			}
		}
		if !added {
			req := store.NewRequest()
			req.AddHistory(th)
			requests = append(requests, req)
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
		Limit: 10000,
	}
	return rlp.EncodeToBytes(payload)
}

// NewHistoryListener returns instance of the HistoryEventListener.
func NewHistoryListener(tracker *HistoryUpdateTracker, requestsUpdates RequestEventsProducer) *HistoryEventListener {
	return &HistoryEventListener{
		tracker:        tracker,
		requestUpdates: requestsUpdates,
	}
}

// RequestEventsProducer produces events that track flow of history requests.
type RequestEventsProducer interface {
	SubscribeEnvelopeEvents(chan<- whisper.EnvelopeEvent) event.Subscription
}

// HistoryEventListener is an object that keeps open subscriptions for event producers and feeds necessary
// information to HistoryUpdateTracker.
type HistoryEventListener struct {
	wg   sync.WaitGroup
	quit chan struct{}

	tracker *HistoryUpdateTracker

	requestUpdates RequestEventsProducer
}

// Start creates subscriptions and spawns a goroutine to consume events from them.
// Must be called in the same thread as Stop.
func (listener *HistoryEventListener) Start() error {
	if listener.quit != nil {
		return errors.New("already running")
	}
	listener.wg.Add(1)
	listener.quit = make(chan struct{})
	requests := make(chan whisper.EnvelopeEvent, 10)
	requestsSub := listener.requestUpdates.SubscribeEnvelopeEvents(requests)
	go func() {
		for {
			select {
			case <-listener.quit:
				requestsSub.Unsubscribe()
				listener.wg.Done()
				return
			case ev := <-requests:
				if ev.Event != whisper.EventMailServerRequestCompleted {
					continue
				}
				data := ev.Data.(*whisper.MailServerResponse)
				err := listener.tracker.UpdateFinishedRequest(ev.Hash, data.LastEnvelopeHash)
				if err != nil {
					log.Error("failed to update request when it was finished", "id", ev.Hash, "error", err)
				}
			}
		}
	}()
	return nil
}

// Stop consuming goroutine and wait until it exits.
func (listener *HistoryEventListener) Stop() {
	if listener.quit == nil {
		return
	}
	close(listener.quit)
	listener.wg.Wait()
}
