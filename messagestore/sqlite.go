package messagestore

import (
	"database/sql"
	"encoding/json"

	"github.com/status-im/migrate"
	"github.com/status-im/migrate/database/sqlcipher"
	bindata "github.com/status-im/migrate/source/go_bindata"
	"github.com/status-im/status-go/messagestore/migrations"
	whisper "github.com/status-im/whisper/whisperv6"
)

// InitializeSQLMessageStore runs migrations on opened database and creates SQLMessageStore instance.
func InitializeSQLMessageStore(db *sql.DB) (SQLMessageStore, error) {
	store := SQLMessageStore{db: db}
	return store, store.migrate()
}

// SQLMessageStore uses SQL database to store messages.
type SQLMessageStore struct {
	db *sql.DB
}

func (store SQLMessageStore) migrate() error {
	resources := bindata.Resource(
		migrations.AssetNames(),
		func(name string) ([]byte, error) {
			return migrations.Asset(name)
		},
	)

	source, err := bindata.WithInstance(resources)
	if err != nil {
		return err
	}

	driver, err := sqlcipher.WithInstance(store.db, &sqlcipher.Config{})
	if err != nil {
		return err
	}

	m, err := migrate.NewWithInstance(
		"go-bindata",
		source,
		"sqlcipher",
		driver)
	if err != nil {
		return err
	}

	if err = m.Up(); err != migrate.ErrNoChange {
		return err
	}
	return nil
}

// Add upserts received message into table with received messages.
func (store SQLMessageStore) Add(enckey string, msg *whisper.ReceivedMessage) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	stmt, err := store.db.Prepare("INSERT OR REPLACE INTO whisper_received_messages(hash, enckey, body) VALUES (?, ?, ?)")
	if err != nil {
		return err
	}
	_, err = stmt.Exec(msg.EnvelopeHash[:], enckey, body)
	return err
}

// Pop reads every row from table with received messages and clears it afterwards.
func (store SQLMessageStore) Pop(enckey string) ([]*whisper.ReceivedMessage, error) {
	tx, err := store.db.Begin()
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query("SELECT body FROM whisper_received_messages WHERE enckey = ?", enckey)
	if err != nil {
		return nil, err
	}
	rst := []*whisper.ReceivedMessage{}
	for rows.Next() {
		body := []byte{}
		err := rows.Scan(&body)
		if err != nil {
			return nil, err
		}
		msg := whisper.ReceivedMessage{}
		err = json.Unmarshal(body, &msg)
		if err != nil {
			return nil, err
		}
		rst = append(rst, &msg)
	}
	_, err = tx.Exec("DELETE FROM whisper_received_messages WHERE enckey = ?", enckey)
	if err != nil {
		return nil, err
	}
	err = tx.Commit()
	if err != nil {
		return nil, err
	}
	return rst, nil
}

// NewIsolated returns sql store where operations are isolated using provided key.
func (store SQLMessageStore) NewIsolated(enckey string) whisper.MessageStore {
	return limitedWithAKey{
		store:  store,
		enckey: enckey,
	}
}

type limitedWithAKey struct {
	store  SQLMessageStore
	enckey string
}

func (store limitedWithAKey) Add(msg *whisper.ReceivedMessage) error {
	return store.store.Add(store.enckey, msg)
}

func (store limitedWithAKey) Pop() ([]*whisper.ReceivedMessage, error) {
	return store.store.Pop(store.enckey)
}
