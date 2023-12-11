package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/revittconsulting/datacryp/api/pkg/utils"
	"github.com/revittconsulting/datacryp/api/internal/types"
	"encoding/hex"
	"github.com/revittconsulting/datacryp/api/internal/mdbx"
	"strings"
)

type IDb interface {
	Close() error
	ListBuckets() ([]string, error)
	CountKeys(bucketName string) (uint64, error)
	FindByKey(bucketName string, key []byte) ([]byte, error)
	FindByValue(bucketName string, value []byte) ([][]byte, error)
	Read(bucketName string, take, offset uint64) ([]types.KeyValuePair, error)
}

func getPageHandler(db IDb) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bucketName := chi.URLParam(r, "bucketName")
		pageNum, err := strconv.Atoi(chi.URLParam(r, "pageNum"))
		if err != nil {
			http.Error(w, "Invalid page number", http.StatusBadRequest)
			return
		}
		pageLen, err := strconv.Atoi(chi.URLParam(r, "pageLen"))
		if err != nil {
			http.Error(w, "Invalid page length", http.StatusBadRequest)
			return
		}

		foundData, err := db.Read(bucketName, uint64(pageLen), uint64(pageNum-1)*uint64(pageLen))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		data := make([]types.KeyValuePairString, 0)

		for _, kv := range foundData {
			data = append(data, kv.HexKeyHexValue())
		}

		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		err = enc.Encode(data)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func listBucketsHandler(db IDb) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		buckets, err := db.ListBuckets()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		err = enc.Encode(buckets)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func lookupByKeyHandler(db IDb) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bucketName := chi.URLParam(r, "bucketName")
		searchKey := chi.URLParam(r, "key")

		var foundValue []byte
		if strings.HasPrefix(searchKey, "0x") {
			searchKey = searchKey[2:]
			str, err := hex.DecodeString(searchKey)
			if err != nil {
				fmt.Println("Error:", err)
				return
			}
			foundValue, _ = db.FindByKey(bucketName, str)
		} else {
			num, err := strconv.ParseUint(searchKey, 10, 64)
			if err != nil {
				fmt.Println("Error:", err)
				return
			}
			skuint := utils.Uint64ToBytes(num)
			foundValue, _ = db.FindByKey(bucketName, skuint)
		}

		response := map[string]string{"value": fmt.Sprintf("%x", foundValue)}
		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		enc.Encode(response)
	}
}

func searchByValueHandler(db IDb) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bucketName := chi.URLParam(r, "bucketName")
		searchValue := chi.URLParam(r, "value")

		num, err := strconv.ParseUint(searchValue, 16, 64)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}

		foundKeys, _ := db.FindByValue(bucketName, utils.Uint64ToBytes(num))
		hexKeys := make([]string, 0)

		for _, key := range foundKeys {
			hexKeys = append(hexKeys, hex.EncodeToString(key))
		}

		response := map[string][]string{"keys": hexKeys}
		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		enc.Encode(response)
	}
}

func keysCountHandler(db IDb) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bucketName := chi.URLParam(r, "bucketName")
		count, err := db.CountKeys(bucketName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		response := map[string]uint64{"count": count}
		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		enc.Encode(response)
	}
}

func main() {

	//db := boltdb.New(dbFile)

	mdbxdb := mdbx.New("/Volumes/2TB/datadirs/hermez-testnet/chaindata/mdbx.dat")

	r := chi.NewRouter()

	r.Get("/buckets", listBucketsHandler(mdbxdb))
	r.Get("/buckets/{bucketName}/pages/{pageNum}/{pageLen}", getPageHandler(mdbxdb))
	r.Get("/buckets/{bucketName}/count", keysCountHandler(mdbxdb))
	r.Get("/buckets/{bucketName}/keys/{key}", lookupByKeyHandler(mdbxdb))
	r.Get("/buckets/{bucketName}/values/{value}", searchByValueHandler(mdbxdb))

	log.Fatal(http.ListenAndServe(":8080", r))
}
