package db

import (
	"encoding/binary"
	"fmt"
	"os"
	"time"

	"github.com/buck54321/eco/encode"
	"github.com/decred/slog"
	"go.etcd.io/bbolt"
	"golang.org/x/crypto/blake2s"
)

const (
	DBVersion = 0
)

var (
	appBucket      = []byte("app")
	versionKey     = []byte("version")
	servicesBucket = []byte("services")
	settingsKey    = []byte("settings")
	stateKey       = []byte("state")
	mapBucket      = []byte("map")
)

// the db.DB interface defined at decred.org/dcrdex/client/db.
type DB struct {
	*bbolt.DB
	log slog.Logger
}

// NewDB is a constructor for a *BoltDB.
func NewDB(dbPath string, logger slog.Logger) (*DB, error) {
	_, err := os.Stat(dbPath)
	isNew := os.IsNotExist(err)

	db, err := bbolt.Open(dbPath, 0600, &bbolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, err
	}

	// Release the file lock on exit.
	bdb := &DB{
		DB:  db,
		log: logger,
	}

	err = bdb.makeTopLevelBuckets([][]byte{appBucket, servicesBucket, mapBucket})
	if err != nil {
		return nil, err
	}

	// If the db is a new one, initialize it with the current DB version.
	if isNew {
		err := bdb.DB.Update(func(dbTx *bbolt.Tx) error {
			bkt := dbTx.Bucket(appBucket)
			if bkt == nil {
				return fmt.Errorf("app bucket not found")
			}

			ver := make([]byte, 4)
			binary.BigEndian.PutUint32(ver, DBVersion)
			err := bkt.Put(versionKey, ver)
			if err != nil {
				return fmt.Errorf("Error initializing the database version: %v", err)
			}

			bdb.log.Infof("creating new version %d database", DBVersion)

			return nil
		})
		if err != nil {
			return nil, err
		}

		return bdb, nil
	}

	// Run upgrades here.

	return bdb, nil
}

// makeTopLevelBuckets creates a top-level bucket for each of the provided keys,
// if the bucket doesn't already exist.
func (db *DB) makeTopLevelBuckets(buckets [][]byte) error {
	return db.Update(func(tx *bbolt.Tx) error {
		for _, bucket := range buckets {
			_, err := tx.CreateBucketIfNotExists(bucket)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

// Store saves the bytes at the specified key. The key is converted to a hash
// before internal use.
func (db *DB) Store(k string, b []byte) error {
	return db.Update(func(tx *bbolt.Tx) error {
		bkt := tx.Bucket(mapBucket)
		if bkt == nil {
			return fmt.Errorf("map bucket not found")
		}
		return bkt.Put(hashKey([]byte(k)), b)
	})
}

// Fetch retrieves the bytes stored with Store.
func (db *DB) Fetch(k string) ([]byte, error) {
	var b []byte
	return b, db.View(func(tx *bbolt.Tx) error {
		bkt := tx.Bucket(mapBucket)
		if bkt == nil {
			return fmt.Errorf("map bucket not found")
		}
		b = encode.CopySlice(bkt.Get(hashKey([]byte(k))))
		return nil
	})
}

func (db *DB) EncodeStore(k string, thing interface{}) error {
	b, err := encode.GobEncode(thing)
	if err != nil {
		return fmt.Errorf("Error encoding %q", k)
	}
	return db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(mapBucket).Put(hashKey([]byte(k)), b)
	})
}

func (db *DB) FetchDecode(k string, thing interface{}) (loaded bool, err error) {
	return loaded, db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(mapBucket).Get(hashKey([]byte(k)))
		if b == nil {
			return nil
		}
		loaded = true
		return encode.GobDecode(b, thing)
	})
}

// hashKey creates a unique key from the hash of the supplied bytes.
func hashKey(b []byte) []byte {
	h := blake2s.Sum256(b)
	return h[:]
}
