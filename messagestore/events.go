package messagestore

import (
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/event"
	whisper "github.com/status-im/whisper/whisperv6"
)

// EventHistoryPersisted used to notify about newly received timestamp for a particular topic.
type EventHistoryPersisted struct {
	Topic     whisper.TopicType
	Timestamp time.Time
	Hash      common.Hash
}

// StoreWithHistoryEvents notifies when history message got persisted.
type storeWithHistoryEvents struct {
	whisper.MessageStore
	feed *event.Feed
}

// Add notifies subscribers if message got persisted succesfully.
func (store *storeWithHistoryEvents) Add(msg *whisper.ReceivedMessage) error {
	err := store.MessageStore.Add(msg)
	if err == nil && msg.P2P {
		store.feed.Send(EventHistoryPersisted{
			Hash:      msg.EnvelopeHash,
			Topic:     msg.Topic,
			Timestamp: time.Unix(int64(msg.Sent), 0),
		})
	}
	return err
}

// NewStoreWithFeed returns instance of the StoreWithHistoryEvents.
func NewStoreWithFeed(store SQLMessageStore) *StoreWithFeed {
	// initialize feed and pass pointer. this feed will be shared with other instances.
	return &StoreWithFeed{store: store, feed: &event.Feed{}}
}

type StoreWithFeed struct {
	store SQLMessageStore
	feed  *event.Feed
}

// Subscribe allows to subscribe for history events.
func (store *StoreWithFeed) Subscribe(events chan<- EventHistoryPersisted) event.Subscription {
	return store.feed.Subscribe(events)
}

// NewIsolated proxies request for the creation of the new isolated store.
func (store *StoreWithFeed) NewIsolated(enckey string) whisper.MessageStore {
	return &storeWithHistoryEvents{
		MessageStore: store.store.NewIsolated(enckey),
		feed:         store.feed,
	}
}
