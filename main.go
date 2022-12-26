package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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

type Message struct {
	BPM int
}

type Block struct {
	Index     int    // position of the data in blockchain
	Timestamp string // the time the data is written
	BPM       int    // beats per minute (pulse rate)
	Hash      string // SHA256 hashing
	PrevHash  string // SHA256 hashing of previous record
}

var Blockchain []Block
var bcServer chan []Block
var mutex = &sync.Mutex{}

// main
func main() {
	err := godotenv.Load() // read in variables from .env
	if err != nil {
		log.Fatal(err)
	}

	bcServer = make(chan []Block)
	// create initial block
	t := time.Now()
	genesisBlock := Block{0, t.String(), 0, "", ""}
	spew.Dump(genesisBlock)

	Blockchain = append(Blockchain, genesisBlock)
	tcpPort := os.Getenv("PORT")
	// start TCP and TCP server
	server, err := net.Listen("tcp", ":"+tcpPort)
	if err != nil {
		log.Fatal(err)
	}
	defer server.Close()

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

	io.WriteString(conn, "Enter a new BPM:")

	scanner := bufio.NewScanner(conn)
	// take in BPM from stdin and add it to blockchain
	go func() {
		for scanner.Scan() {
			bpm, err := strconv.Atoi(scanner.Text())
			if err != nil {
				log.Printf("%v not a number : %v", scanner.Text(), err)
				continue
			}
			newBlock, err := generateBlock(Blockchain[len(Blockchain)-1], bpm)
			if err != nil {
				log.Println(err)
				continue
			}
			if isBlockValid(newBlock, Blockchain[len(Blockchain)-1]) {
				newBlockchain := append(Blockchain, newBlock)
				replaceChain(newBlockchain)
			}

			bcServer <- Blockchain
			io.WriteString(conn, "\nEnter a new BPM: ")
		}
	}()

	// simulate receiving broadcast
	go func() {
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
	}()

	for _ = range bcServer {
		spew.Dump(Blockchain)
	}
}

// concatenates Index, Timestamp, BPM and PrevHash and return its SHA256 hashing
func calculateHash(block Block) string {
	record := string(block.Index) + block.Timestamp + string(block.BPM) + block.PrevHash
	h := sha256.New()
	h.Write([]byte(record))
	hashed := h.Sum(nil)
	return hex.EncodeToString(hashed)
}

func generateBlock(oldBlock Block, BPM int) (Block, error) {
	var newBlock Block
	t := time.Now()
	newBlock.Index = oldBlock.Index + 1
	newBlock.Timestamp = t.String()
	newBlock.BPM = BPM
	newBlock.PrevHash = oldBlock.Hash
	newBlock.Hash = calculateHash(newBlock)

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

	if calculateHash(newBlock) != newBlock.Hash {
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
// 	response, err := json.MarshalIndent(payload, "", " ")
// 	if err != nil {
// 		w.WriteHeader(http.StatusInternalServerError)
// 		w.Write([]byte("HTTP 500: Internal Server Error"))
// 		return
// 	}
// 	w.WriteHeader(code)
// 	w.Write(response)
// }
