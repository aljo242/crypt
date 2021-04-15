package main

import (
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

const (
	difficulty = 10
)

var (
	mutex = &sync.Mutex{}
)

type Block struct {
	Index      uint64
	Timestamp  string
	BPM        uint64
	Hash       string
	PrevHash   string
	Difficulty uint64
	Nonce      string
}

type Message struct {
	BPM uint64
}

type BlockChain []Block

var localChain BlockChain

func getHash(block Block) string {
	input := strconv.Itoa(int(block.Index)) + block.Timestamp + strconv.Itoa(int(block.BPM)) + block.PrevHash + block.Nonce
	hash := sha512.New()
	hash.Write([]byte(input))
	hashed := hash.Sum(nil)
	return hex.EncodeToString(hashed)
}

func generateBlock(prevBlock Block, BPM uint64) Block {
	var newBlock Block

	t := time.Now()

	newBlock.Index = prevBlock.Index + 1
	newBlock.BPM = BPM
	newBlock.Timestamp = t.String()
	newBlock.PrevHash = prevBlock.Hash
	newBlock.Difficulty = difficulty

	// PoW
	start := time.Now()
	fmt.Println("finding hash with difficulty : ", difficulty)
	for i := 0; ; i++ {
		hex := fmt.Sprintf("%x", i)
		newBlock.Nonce = hex
		if !IsHashValid(getHash(newBlock), int(newBlock.Difficulty)) {
			continue
		} else {
			fmt.Println(getHash(newBlock), " work done!")
			elapsed := time.Now().Second() - start.Second()
			fmt.Printf("Elapsed Time: %vs\n", elapsed)

			newBlock.Hash = getHash(newBlock)
			break
		}
	}

	return newBlock
}

func IsHashValid(hash string, difficulty int) bool {
	prefix := strings.Repeat("0", difficulty)
	return strings.HasPrefix(hash, prefix)
}

func IsBlockValid(newBlock, prevBlock Block) bool {
	if prevBlock.Index+1 != newBlock.Index {
		return false
	}

	if prevBlock.Hash != newBlock.PrevHash {
		return false
	}

	if getHash(newBlock) != newBlock.Hash {
		return false
	}

	return true
}

func RespondWithJSON(w http.ResponseWriter, r *http.Request, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	resp, err := json.MarshalIndent(payload, "", " ")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("HTTP 500: Internal Server Error"))
	}

	w.WriteHeader(code)
	w.Write(resp)
}

func handleGetBlockchain() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		bytes, err := json.MarshalIndent(localChain, "", " ")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		io.WriteString(w, string(bytes))
	}
}

func handleWriteBlockchain() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var m Message

		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&m); err != nil {
			RespondWithJSON(w, r, http.StatusBadRequest, r.Body)
			return
		}

		defer r.Body.Close()

		// ensure atomicity when creating new block
		mutex.Lock()
		newBlock := generateBlock(localChain[len(localChain)-1], m.BPM)
		mutex.Unlock()

		if IsBlockValid(newBlock, localChain[len(localChain)-1]) {
			localChain = append(localChain, newBlock)
		}

		RespondWithJSON(w, r, http.StatusCreated, newBlock)
	}
}

func makeMuxRouter() http.Handler {
	mux := mux.NewRouter()

	mux.HandleFunc("/", handleGetBlockchain()).Methods("GET")
	mux.HandleFunc("/", handleWriteBlockchain()).Methods("POST")

	return mux
}

func run() error {
	mux := makeMuxRouter()
	httpAddr := os.Getenv("PORT")
	log.Println("Listening on :", httpAddr)
	s := &http.Server{
		Addr:           ":" + httpAddr,
		Handler:        mux,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	if err := s.ListenAndServe(); err != nil {
		return err
	}

	return nil
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		t := time.Now()
		genesisBlock := Block{}
		genesisBlock = Block{
			Index:      0,
			Timestamp:  t.String(),
			BPM:        0,
			Hash:       getHash(genesisBlock),
			PrevHash:   "",
			Difficulty: difficulty,
		}

		mutex.Lock()
		localChain = append(localChain, genesisBlock)
		mutex.Unlock()
	}()

	log.Fatal(run())
}
