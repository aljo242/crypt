package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/joho/godotenv"
)

const ()

var ()

// Block is "item" in our blockchain
type Block struct {
	Index     uint64
	Timestamp string
	BPM       uint64 // instead of this, we can have the block contain the current ledge which is a map of IDs to balances
	Hash      string
	PrevHash  string
}

type BlockChain []Block

var localChain BlockChain
var serverChain chan BlockChain
var mutex = &sync.Mutex{}

// perform SHA256 hash of block
func hashBlock(block Block) (string, error) {
	input := string(block.Index) + block.Timestamp + string(block.BPM) + block.PrevHash
	hash := sha256.New()
	_, err := hash.Write([]byte(input))
	if err != nil {
		return "", err
	}

	hashed := hash.Sum(nil) // TODO ????
	return hex.EncodeToString(hashed), nil
}

func generateNewBlock(oldBlock Block, BPM uint64) (Block, error) {
	var newBlock Block

	t := time.Now()

	newBlock.Index = oldBlock.Index + 1
	newBlock.Timestamp = t.String()
	newBlock.BPM = BPM
	newBlock.PrevHash = oldBlock.Hash

	h, err := hashBlock(newBlock)
	if err != nil {
		return newBlock, err
	}

	newBlock.Hash = h

	return newBlock, nil
}

// IsBlockValid ensures the old and new blocks have proper indexing, and hashes
func IsBlockValid(newBlock, oldBlock Block) bool {
	if newBlock.Index != oldBlock.Index+1 {
		fmt.Println(1)
		return false
	}

	if newBlock.PrevHash != oldBlock.Hash {
		return false
	}

	h, err := hashBlock(newBlock)
	if err != nil {
		fmt.Println(2)
		return false
	}

	if h != newBlock.Hash {
		fmt.Println(3)
		fmt.Println(h)
		fmt.Println(newBlock.Hash)
		return false
	}

	return true
}

// if another chain is longer than our local chain, use that one
func replaceChain(newChain BlockChain) {
	if len(newChain) > len(localChain) {
		localChain = newChain
	}
}

func handleConn(conn net.Conn) {
	defer conn.Close()

	io.WriteString(conn, "Enter a new BPM: ")

	scanner := bufio.NewScanner(conn)

	// take in  BPM from stdin and add it to the blockchain after validating
	go func() {
		for scanner.Scan() {
			bpm, err := strconv.Atoi(scanner.Text())
			if err != nil {
				log.Printf("%v not a number: %v", scanner.Text(), err)
				continue
			}

			newBlock, err := generateNewBlock(localChain[len(localChain)-1], uint64(bpm))
			if err != nil {
				log.Println(err)
				continue
			}

			if IsBlockValid(newBlock, localChain[len(localChain)-1]) {
				newBlockChain := append(localChain, newBlock)
				replaceChain(newBlockChain)
			}

			serverChain <- localChain
			io.WriteString(conn, "\nEnter a new BPM: ")
		}
	}()

	go func() {
		for {
			time.Sleep(30 * time.Second) // simulate transmission time
			mutex.Lock()
			output, err := json.Marshal(localChain)
			if err != nil {
				log.Fatal(err)
			}
			mutex.Unlock()
			io.WriteString(conn, string(output))
		}
	}()

	for _ = range serverChain {
		spew.Dump(localChain)
	}
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal(err) // if we fail to load env, just kill app, since nothing will work anyways
	}

	serverChain = make(chan BlockChain)

	// create genesis block
	t := time.Now()
	genesisBlock := Block{}
	h, err := hashBlock(genesisBlock)
	if err != nil {
		log.Fatal(err) // unable to build a genesis block
	}

	genesisBlock = Block{
		Index:     0,
		Timestamp: t.String(),
		BPM:       0,
		Hash:      h,
		PrevHash:  "",
	}

	spew.Dump(genesisBlock)
	localChain = append(localChain, genesisBlock)

	// start TCP and serve TCP server
	server, err := net.Listen("tcp", ":"+os.Getenv("ADDR"))
	if err != nil {
		log.Fatal(err)
	}

	defer server.Close()

	// loop pump accepts a conn, then dispatches a goroutine to handle
	for {
		conn, err := server.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go handleConn(conn)
	}
}
