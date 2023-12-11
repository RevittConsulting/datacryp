package bolt

import (
	"log"
	"github.com/boltdb/bolt"
	"github.com/revittconsulting/datacryp/api/internal/types"
	"fmt"
)

type Bolt struct {
	db *bolt.DB
}

func New(filePath string) *Bolt {
	return &Bolt{
		db: openDB(filePath),
	}
}

func openDB(filePath string) *bolt.DB {
	db, err := bolt.Open(filePath, 0600, nil)
	if err != nil {
		log.Fatal(err)
	}
	return db
}

func (b *Bolt) Close() error {
	return b.db.Close()
}

func (b *Bolt) ListBuckets() ([]string, error) {
	var buckets []string
	err := b.db.View(func(tx *bolt.Tx) error {
		return tx.ForEach(func(name []byte, _ *bolt.Bucket) error {
			buckets = append(buckets, string(name))
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return buckets, nil
}

func (b *Bolt) FindByKey(bucketName string, key []byte) ([]byte, error) {
	var foundVal []byte
	err := b.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return nil
		}
		v := b.Get(key)
		if v == nil {
			return nil
		}
		foundVal = v
		return nil
	})
	if err != nil {
		return nil, err
	}
	return foundVal, nil
}

func (b *Bolt) FindByValue(bucketName string, value []byte) ([][]byte, error) {
	foundKeys := make([][]byte, 0)
	err := b.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return nil
		}
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			if string(v) == string(value) {
				foundKeys = append(foundKeys, k)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return foundKeys, nil
}

func (b *Bolt) Read(bucketName string, take, offset uint64) ([]types.KeyValuePair, error) {
	var data []types.KeyValuePair

	err := b.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return fmt.Errorf("bucket %q not found", bucketName)
		}

		c := b.Cursor()

		var count uint64
		for k, v := c.First(); k != nil; k, v = c.Next() {
			if count >= offset && count < (offset+take) {
				data = append(data, types.KeyValuePair{Key: k, Value: v})
			}

			count++
			if count >= (offset + take) {
				break
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return data, nil
}
