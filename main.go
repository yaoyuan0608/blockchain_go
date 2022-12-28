package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/joho/godotenv"
)

type Message struct {
	BPM int
}

const difficulty = 1

type Block struct {
	Index     int    // position of the data in blockchain
	Timestamp string // the time the data is written
	BPM       int    // beats per minute (pulse rate)
	Hash      string // SHA256 hashing
	PrevHash  string // SHA256 hashing of previous record
	Validator string
}

// blockchain is a series of validated blocks
var Blockchain []Block
var tempBlocks []Block

// candidateBlocks handles incoming blocks for validation
var candidateBlocks = make(chan Block)

// announcements broadcast winning validator to all nodes
var announncements = make(chan string)

// validator keeps track of open validator and balances
var validators = make(map[string]int)

var mutex = &sync.Mutex{}

// main
func main() {
	err := godotenv.Load() // read in variables from .env
	if err != nil {
		log.Fatal(err)
	}

	t := time.Now()
	genesisBlock := Block{}
	genesisBlock = Block{0, t.String(), 0, calculateBlockHash(genesisBlock), "", ""}
	spew.Dump(genesisBlock)

	mutex.Lock()
	Blockchain = append(Blockchain, genesisBlock)
	mutex.Unlock()

	tcpPort := os.Getenv("PORT")
	// start TCP and TCP server
	server, err := net.Listen("tcp", ":"+tcpPort)
	if err != nil {
		log.Fatal(err)
	}
	defer server.Close()

	go func() {
		for candidate := range candidateBlocks {
			mutex.Lock()
			tempBlocks = append(tempBlocks, candidate)
			mutex.Unlock()
		}
	}()

	go func() {
		for {
			pickWinner()
		}
	}()

	for {
		conn, err := server.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go handleConn(conn)
	}
}

func handleConn(conn net.Conn) {
	defer conn.Close()
	go func() {
		for {
			msg := <-announncements
			io.WriteString(conn, msg)
		}
	}()

	// validator address
	var address string

	// allow user to allocate number of tokens to stake
	io.WriteString(conn, "Enter your token amount:")
	scanBalance := bufio.NewScanner(conn)
	for scanBalance.Scan() {
		balance, err := strconv.Atoi(scanBalance.Text())
		if err != nil {
			log.Printf("%v not a number: %v", scanBalance.Text(), err)
			return
		}
		t := time.Now()
		address = calculateHash(t.String())
		validators[address] = balance
		fmt.Println(validators)
		break
	}

	io.WriteString(conn, "\nEnter a new BPM: ")
	scanBPM := bufio.NewScanner(conn)

	// take in BPM from stdin and add it to blockchain
	go func() {
		for {
			for scanBPM.Scan() {
				bpm, err := strconv.Atoi(scanBPM.Text())
				// if someone tries to give a bad input, delete the validator and lose all tokens
				if err != nil {
					log.Printf("%v not a number : %v", scanBPM.Text(), err)
					delete(validators, address)
					conn.Close()
				}
				mutex.Lock()
				oldLastIndex := Blockchain[len(Blockchain)-1]
				mutex.Unlock()

				newBlock, err := generateBlock(oldLastIndex, bpm, address)
				if err != nil {
					log.Println(err)
					continue
				}
				if isBlockValid(newBlock, oldLastIndex) {
					candidateBlocks <- newBlock
				}
				io.WriteString(conn, "\nEnter a new BPM: ")
			}
		}

	}()

	// simulate receiving broadcast
	for {
		time.Sleep(30 * time.Second)
		mutex.Lock()
		output, err := json.Marshal(Blockchain)
		if err != nil {
			log.Fatal(err)
		}
		mutex.Unlock()
		io.WriteString(conn, string(output))
	}

}

// create a lottery pool of validators and chooses the validator who gets to forge ablock
// randomly select by weighted amount of tokens staked
func pickWinner() {
	time.Sleep(30 * time.Second)
	mutex.Lock()
	temp := tempBlocks
	mutex.Unlock()

	lotteryPool := []string{}
	if len(temp) > 0 {
	OUTER:
		for _, block := range temp {
			for _, node := range lotteryPool {
				if block.Validator == node {
					continue OUTER
				}
			}
			mutex.Lock()
			setValidators := validators
			mutex.Unlock()

			k, ok := setValidators[block.Validator]
			if ok {
				for i := 0; i < k; i++ {
					lotteryPool = append(lotteryPool, block.Validator)
				}
			}
		}
		// randomly pick winner
		s := rand.NewSource(time.Now().Unix())
		r := rand.New(s)
		lotteryWinner := lotteryPool[r.Intn(len(lotteryPool))]

		for _, block := range temp {
			if block.Validator == lotteryWinner {
				mutex.Lock()
				Blockchain = append(Blockchain, block)
				mutex.Unlock()
				for _ = range validators {
					announncements <- "\nWinning Validator: " + lotteryWinner + "\n"
				}
				break
			}
		}
	}
	mutex.Lock()
	tempBlocks = []Block{}
	mutex.Unlock()
}

// concatenates Index, Timestamp, BPM and PrevHash and return its SHA256 hashing
func calculateHash(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	hashed := h.Sum(nil)
	return hex.EncodeToString(hashed)
}

func calculateBlockHash(block Block) string {
	record := strconv.Itoa(block.Index) + block.Timestamp + strconv.Itoa(block.BPM) + block.PrevHash
	return calculateHash(record)
}

func generateBlock(oldBlock Block, BPM int, address string) (Block, error) {
	var newBlock Block
	t := time.Now()
	newBlock.Index = oldBlock.Index + 1
	newBlock.Timestamp = t.String()
	newBlock.BPM = BPM
	newBlock.PrevHash = oldBlock.Hash
	newBlock.Hash = calculateBlockHash(newBlock)
	newBlock.Validator = address

	return newBlock, nil
}

// Block validation
func isBlockValid(newBlock, oldBlock Block) bool {
	if oldBlock.Index+1 != newBlock.Index {
		return false
	}

	if oldBlock.Hash != newBlock.PrevHash {
		return false
	}

	if calculateBlockHash(newBlock) != newBlock.Hash {
		return false
	}
	return true
}

// replace the Blockchain with the longest one
func replaceChain(newBlocks []Block) {
	mutex.Lock()
	if len(newBlocks) > len(Blockchain) {
		Blockchain = newBlocks
	}
	mutex.Unlock()
}

// Hash validation
func isHashValid(hash string, difficulty int) bool {
	prefix := strings.Repeat("0", difficulty)
	return strings.HasPrefix(hash, prefix)
}

// // web server
// func run() error {
// 	mux := makeMuxRouter()
// 	httpAddr := os.Getenv("PORT")
// 	log.Println("Listening on ", os.Getenv("PORT"))
// 	s := &http.Server{
// 		Addr:           ":" + httpAddr,
// 		Handler:        mux,
// 		ReadTimeout:    10 * time.Second,
// 		WriteTimeout:   10 * time.Second,
// 		MaxHeaderBytes: 1 << 20,
// 	}

// 	if err := s.ListenAndServe(); err != nil {
// 		return err
// 	}
// 	return nil
// }

// // define all handlers
// func makeMuxRouter() http.Handler {
// 	muxRouter := mux.NewRouter()
// 	muxRouter.HandleFunc("/", handleGetBlockchain).Methods("GET")
// 	muxRouter.HandleFunc("/", handleWriteBlock).Methods("POST")
// 	return muxRouter
// }

// // GET handler
// func handleGetBlockchain(w http.ResponseWriter, r *http.Request) {
// 	bytes, err := json.MarshalIndent(Blockchain, "", " ")
// 	if err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}
// 	io.WriteString(w, string(bytes))
// }

// // POST handler
// func handleWriteBlock(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	var m Message

// 	decoder := json.NewDecoder(r.Body)
// 	if err := decoder.Decode(&m); err != nil {
// 		respondWithJSON(w, r, http.StatusBadRequest, r.Body)
// 		return
// 	}
// 	defer r.Body.Close()

// 	mutex.Lock()
// 	prevBlock := Blockchain[len(Blockchain)-1]
// 	newBlock, err := generateBlock(prevBlock, m.BPM)
// 	if err != nil {
// 		respondWithJSON(w, r, http.StatusInternalServerError, m)
// 		return
// 	}
// 	if isBlockValid(newBlock, prevBlock) {
// 		Blockchain = append(Blockchain, newBlock)
// 		spew.Dump(Blockchain) // help to print structs into consle
// 	}
// 	mutex.Unlock()
// 	respondWithJSON(w, r, http.StatusCreated, newBlock)
// }

// func respondWithJSON(w http.ResponseWriter, r *http.Request, code int, payload interface{}) {
// 	w.Header().Set("Content-Type", "application/json")
// 	response, err := json.MarshalIndent(payload, "", " ")
// 	if err != nil {
// 		w.WriteHeader(http.StatusInternalServerError)
// 		w.Write([]byte("HTTP 500: Internal Server Error"))
// 		return
// 	}
// 	w.WriteHeader(code)
// 	w.Write(response)
// }
