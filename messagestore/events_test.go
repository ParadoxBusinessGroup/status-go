package messagestore

import (
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/status-im/status-go/sqlite"
	whisper "github.com/status-im/whisper/whisperv6"
	"github.com/stretchr/testify/require"
)

func TestEvents(t *testing.T) {
	tmpdb, err := ioutil.TempFile("", "messagestoredb")
	defer os.Remove(tmpdb.Name())
	require.NoError(t, err)
	db, err := sqlite.OpenDB(tmpdb.Name(), "testkey")
	require.NoError(t, err)
	store, err := InitializeSQLMessageStore(db)

	feed := NewStoreWithFeed(store)
	eventer := feed.NewIsolated("test")
	events := make(chan EventHistoryPersisted, 1)
	sub := feed.Subscribe(events)
	defer sub.Unsubscribe()

	now := time.Now().Unix()
	msg := whisper.ReceivedMessage{
		EnvelopeHash: common.Hash{},
		Sent:         uint32(now),
		Topic:        whisper.TopicType{1},
		P2P:          true,
	}
	require.NoError(t, eventer.Add(&msg))
	select {
	case <-time.After(time.Second):
		require.FailNow(t, "timed out waiting for event")
	case ev := <-events:
		require.Equal(t, now, ev.Timestamp.Unix())
		require.Equal(t, msg.Topic, ev.Topic)
	}
}
