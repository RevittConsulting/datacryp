package level

import (
	"log"

	"github.com/revittconsulting/datacryp/api/internal/types"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/util"
)

type Level struct {
	db *leveldb.DB
}

func New(filePath string) *Level {
	db, err := leveldb.OpenFile(filePath, nil)
	if err != nil {
		log.Fatal(err)
	}
	return &Level{db}
}

func (l *Level) Close() error {
	return l.db.Close()
}

func (l *Level) FindByKey(bucketName string, key []byte) ([]byte, error) {
	fullKey := bucketName + string(key)
	val, err := l.db.Get([]byte(fullKey), nil)
	if err != nil {
		return nil, err
	}
	return val, nil
}

func (l *Level) FindByValue(bucketName string, value []byte) ([]string, error) {
	var foundKeys []string
	iter := l.db.NewIterator(util.BytesPrefix([]byte(bucketName)), nil)
	for iter.Next() {
		if string(iter.Value()) == string(value) {
			foundKeys = append(foundKeys, string(iter.Key()))
		}
	}
	iter.Release()
	err := iter.Error()
	return foundKeys, err
}

func (l *Level) Read(bucketName string, take, offset int) ([]types.KeyValuePair, error) {
	var data []types.KeyValuePair
	iter := l.db.NewIterator(util.BytesPrefix([]byte(bucketName)), nil)
	count := 0
	for iter.Next() {
		if count >= offset && count < (offset+take) {
			data = append(data, types.KeyValuePair{
				Key:   iter.Key(),
				Value: iter.Value(),
			})
		}
		count++
		if count >= (offset + take) {
			break
		}
	}
	iter.Release()
	err := iter.Error()
	return data, err
}
