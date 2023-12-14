package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"encoding/hex"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/revittconsulting/datacryp/api/config"
	"github.com/revittconsulting/datacryp/api/internal/mdbx"
	"github.com/revittconsulting/datacryp/api/internal/types"
	"github.com/revittconsulting/datacryp/api/pkg/utils"
	"github.com/spf13/viper"
	"strings"
)

type IDb interface {
	Close() error
	ListBuckets() ([]string, error)
	CountKeys(bucketName string) (uint64, error)
	CountKeysOfLength(bucketName string, length uint64) (uint64, []string, error)
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

func keysCountLengthHandler(db IDb, withKeys bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bucketName := chi.URLParam(r, "bucketName")
		length, err := strconv.ParseUint(chi.URLParam(r, "length"), 10, 64)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		count, keys, err := db.CountKeysOfLength(bucketName, length)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var response map[string]interface{}
		if withKeys {
			response = map[string]interface{}{"count": count, "keys": keys}
		} else {
			response = map[string]interface{}{"count": count}
		}

		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		enc.Encode(response)
	}
}

func main() {
	// initialise from config
	cfg := &config.Config{}
	err := initializeConfig(cfg)
	if err != nil {
		log.Fatal(err)
	}

	mdbxFilePath := fmt.Sprintf("%s/%s", cfg.DbFile, "chaindata/mdbx.dat")
	fmt.Println("mdbxFilePath:", mdbxFilePath)

	mdbxdb := mdbx.New(mdbxFilePath)

	r := chi.NewRouter()

	cors := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		ExposedHeaders:   []string{"*"},
		AllowCredentials: true,
		MaxAge:           300,
	})

	r.Use(cors.Handler)

	r.Get("/buckets", listBucketsHandler(mdbxdb))
	r.Get("/buckets/{bucketName}/pages/{pageNum}/{pageLen}", getPageHandler(mdbxdb))
	r.Get("/buckets/{bucketName}/count", keysCountHandler(mdbxdb))
	r.Get("/buckets/{bucketName}/count/{length}", keysCountLengthHandler(mdbxdb, false))
	r.Get("/buckets/{bucketName}/count/{length}/keys", keysCountLengthHandler(mdbxdb, true))
	r.Get("/buckets/{bucketName}/keys/{key}", lookupByKeyHandler(mdbxdb))
	r.Get("/buckets/{bucketName}/values/{value}", searchByValueHandler(mdbxdb))

	log.Fatal(http.ListenAndServe(":8080", r))
}

func initializeConfig(cfg *config.Config) error {
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return fmt.Errorf("read config file: %w", err)
		}
	}

	// set config via env vars
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()
	viper.AllowEmptyEnv(true)

	return viper.Unmarshal(cfg)
}
