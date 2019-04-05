package shhext

import (
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/status-im/status-go/db"
	"github.com/status-im/status-go/mailserver"
	whisper "github.com/status-im/whisper/whisperv6"
	"github.com/stretchr/testify/require"
)

func createInMemStore(t *testing.T) db.HistoryStore {
	mdb, err := db.NewMemoryDB()
	require.NoError(t, err)
	return db.NewHistoryStore(mdb)
}

func TestRenewRequest(t *testing.T) {
	req := db.HistoryRequest{}
	duration := time.Hour
	req.AddHistory(db.TopicHistory{Duration: duration})

	firstNow := time.Now()
	RenewRequests([]db.HistoryRequest{req}, firstNow)

	initial := firstNow.Add(-duration).Unix()

	th := req.Histories()[0]
	require.Equal(t, initial, th.Current.Unix())
	require.Equal(t, initial, th.First.Unix())
	require.Equal(t, firstNow.Unix(), th.End.Unix())

	secondNow := time.Now()
	RenewRequests([]db.HistoryRequest{req}, secondNow)

	require.Equal(t, initial, th.Current.Unix())
	require.Equal(t, initial, th.First.Unix())
	require.Equal(t, secondNow.Unix(), th.End.Unix())
}

func TestCreateTopicOptionsFromRequest(t *testing.T) {
	req := db.HistoryRequest{}
	topic := whisper.TopicType{1}
	now := time.Now()
	req.AddHistory(db.TopicHistory{Topic: topic, Current: now, End: now})
	options := CreateTopicOptionsFromRequest(req)
	require.Len(t, options, len(req.Histories()),
		"length must be equal to the number of topic histories attached to request")
	require.Equal(t, topic, options[0].Topic)
	require.Equal(t, uint64(now.Add(-WhisperTimeAllowance).UnixNano()), options[0].Range.Start,
		"start of the range must be adjusted by the whisper time allowance")
	require.Equal(t, uint64(now.UnixNano()), options[0].Range.End)
}

func TestTopicOptionsToBloom(t *testing.T) {
	options := TopicOptions{
		{Topic: whisper.TopicType{1}, Range: Range{Start: 1, End: 10}},
		{Topic: whisper.TopicType{2}, Range: Range{Start: 3, End: 12}},
	}
	bloom := options.ToBloomFilterOption()
	require.Equal(t, uint64(3), bloom.Range.Start, "Start must be the latest Start across all options")
	require.Equal(t, uint64(12), bloom.Range.End, "End must be the latest End across all options")
	require.Equal(t, topicsToBloom(options[0].Topic, options[1].Topic), bloom.Filter)
}

func TestBloomFilterToMessageRequestPayload(t *testing.T) {
	var (
		start   uint32 = 10
		end     uint32 = 20
		filter         = []byte{1, 1, 1, 1}
		message        = mailserver.MessagesRequestPayload{
			Lower: start,
			Upper: end,
			Bloom: filter,
			Batch: true,
		}
		bloomOption = BloomFilterOption{
			Filter: filter,
			Range: Range{
				Start: uint64(start),
				End:   uint64(end),
			},
		}
	)
	expected, err := rlp.EncodeToBytes(message)
	require.NoError(t, err)
	payload, err := bloomOption.ToMessagesRequestPayload()
	require.NoError(t, err)
	require.Equal(t, expected, payload)
}

func TestCreateRequestsEmptyState(t *testing.T) {
	tracker := NewHistoryUpdateTracker(createInMemStore(t))
	requests, err := tracker.CreateRequests([]TopicRequest{
		{Topic: whisper.TopicType{1}, Duration: time.Hour},
		{Topic: whisper.TopicType{2}, Duration: time.Hour},
		{Topic: whisper.TopicType{3}, Duration: 10 * time.Hour},
	})
	require.NoError(t, err)
	require.Len(t, requests, 2)
	// FIXME verify requests content
}

func TestCreateRequestsWithExistingRequest(t *testing.T) {
	store := createInMemStore(t)
	req := store.NewRequest()
	req.ID = common.Hash{1}
	th := store.NewHistory(whisper.TopicType{1}, time.Hour)
	req.AddHistory(th)
	require.NoError(t, req.Save())
	tracker := NewHistoryUpdateTracker(store)
	requests, err := tracker.CreateRequests([]TopicRequest{
		{Topic: whisper.TopicType{1}, Duration: time.Hour},
		{Topic: whisper.TopicType{2}, Duration: time.Hour},
		{Topic: whisper.TopicType{3}, Duration: time.Hour},
	})
	require.NoError(t, err)
	require.Len(t, requests, 2)
}

func TestRequestFinishedUpdate(t *testing.T) {
	store := createInMemStore(t)
	req := store.NewRequest()
	req.ID = common.Hash{1}
	now := time.Now()
	thOne := store.NewHistory(whisper.TopicType{1}, time.Hour)
	thOne.End = now
	thTwo := store.NewHistory(whisper.TopicType{2}, time.Hour)
	thTwo.End = now
	req.AddHistory(thOne)
	req.AddHistory(thTwo)
	require.NoError(t, req.Save())

	tracker := NewHistoryUpdateTracker(store)
	require.NoError(t, tracker.UpdateFinishedRequest(req.ID))
	_, err := store.GetRequest(req.ID)
	require.EqualError(t, err, "leveldb: not found")

	require.NoError(t, thOne.Load())
	require.NoError(t, thTwo.Load())
	require.Equal(t, thOne.End, thOne.Current)
	require.Equal(t, thTwo.End, thTwo.Current)
}

func TestTopicHistoryUpdate(t *testing.T) {
	store := createInMemStore(t)
	th := store.NewHistory(whisper.TopicType{1}, time.Hour)
	require.NoError(t, th.Save())
	tracker := NewHistoryUpdateTracker(store)
	now := time.Now()
	hour := now.Add(time.Hour)

	require.NoError(t, tracker.UpdateTopicHistory(th.Topic, hour))
	require.NoError(t, th.Load())
	require.Equal(t, hour.Unix(), th.Current.Unix())

	require.NoError(t, tracker.UpdateTopicHistory(th.Topic, now))
	require.NoError(t, th.Load())
	require.Equal(t, hour.Unix(), th.Current.Unix())
}
