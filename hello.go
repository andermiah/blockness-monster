package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
	"slices"
	"bytes"
)

// ammount of leading 0's in the hash
var difficulty int = 2
var peers []string
var blockchain Blockchain

type Blockchain struct {
	Blocks []Block
}

type Block struct {
	Hash         [32]byte
	PreviousHash [32]byte
	Data         string
	Date         time.Time
	Nonce        int
}

func printBlockchain() {
	for i, val := range blockchain.Blocks {
		fmt.Printf("%d - %s\n", i, val.Data)
	}
	fmt.Println()
}

func connectPeers(peers []string) {
	for _, port := range peers {
		if port == os.Args[1] {
			continue
		}

		fmt.Printf("trying to connect to %s\n", port)

		server := "http://localhost:" + port

		_, err := http.Post(server + "/peers", "text/plain", strings.NewReader(os.Args[1]))
		if err != nil {
			fmt.Printf("error %s did not want to connect\n", port)
			continue
		}

		resp, err := http.Get(server + "/chain")
		if err != nil {
			fmt.Printf("error %s did not share blockchain", port)
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		var newBlockchain Blockchain
		err = json.Unmarshal(body, &newBlockchain)
		if err != nil {
			fmt.Println("error")
		}

		if len(newBlockchain.Blocks) > len(blockchain.Blocks) {
			blockchain = newBlockchain
		}

		peerWantsToConnect(port)
	}
	fmt.Println()
}

func peerWantsToConnect(port string) {
	if !slices.Contains(peers, port) {
		peers = append(peers, port)
		fmt.Printf("%s connected\n", port)
	}
}

func updatePeersWithNewBlock(block Block) {
	for _, port := range peers {
		server := "http://localhost:" + port

		j, _ := json.Marshal(block)

		resp, err := http.Post(server + "/recieve", "application/json", bytes.NewReader(j))
		fmt.Printf("block sent to %s\n", port)
		fmt.Println()
		if err != nil {
			fmt.Println("peer unreachable")
		}
		if resp.StatusCode == http.StatusConflict {
			j, _ := json.Marshal(blockchain)
			fmt.Printf("syncing to %s\n", port)
			resp, err := http.Post(server + "/sync", "application/json", bytes.NewReader(j))
			body, _ := io.ReadAll(resp.Body)

			if err != nil {
				fmt.Println("peer cannot sync")
			}

			if resp.StatusCode == http.StatusConflict {
				fmt.Println("other blockchain was longer")
				var newBlockchain Blockchain
				json.Unmarshal(body, &newBlockchain)
				blockchain = newBlockchain
				fmt.Println("new sync'd blockchain is")
				printBlockchain()
			} else {
				fmt.Println("blockchain sync'd")
				fmt.Println()
			}
		}
	}
}

func mineBlock(blockchain *Blockchain, data string) Block {
	fmt.Printf("converting '%s' to a block\n", data)

	previousHash := [32]byte{}
	if len(blockchain.Blocks) > 0 {
		previousHash = blockchain.Blocks[len(blockchain.Blocks)-1].Hash
	}

	block := Block{
		Data:         data,
		PreviousHash: previousHash,
		Date:         time.Now(),
		Nonce:        0,
	}

	j, _ := json.Marshal(block)
	hash := sha256.Sum256(j)

	for !passDifficulty(hash) {
		block.Nonce++
		j, _ := json.Marshal(block)
		hash = sha256.Sum256(j)
	}
	fmt.Printf("mined after %d attempts\n", block.Nonce)

	block.Hash = hash
	blockchain.Blocks = append(blockchain.Blocks, block)

	fmt.Println("new blockchain is")
	printBlockchain()

	return block
}

func checkBlock(block Block) bool {
	if !passDifficulty(block.Hash) {
		fmt.Println("block failed difficulty")
		return false
	}
	lastHash := [32]byte{}
	if len(blockchain.Blocks) > 0 {
		lastHash = blockchain.Blocks[len(blockchain.Blocks)-1].Hash
	}
	if lastHash != block.PreviousHash {
		fmt.Println("block failed to match previous hash")
		return false
	}
	return true
}

func passDifficulty(hash [32]byte) bool {
	i := 1
	for i <= difficulty {
		if hash[i-1] != 0 {
			return false
		}
		i++
	}
	return true
}

func main() {
	possiblePeers := []string{"8080", "8081", "8082"}
	connectPeers(possiblePeers)

	http.HandleFunc("/mine", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		block := mineBlock(&blockchain, string(body))
		fmt.Println("block created, trying to update peers...")
		updatePeersWithNewBlock(block)
	})

	http.HandleFunc("/recieve", func(w http.ResponseWriter, r *http.Request) {
		j, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		var block Block
		err = json.Unmarshal(j, &block)
		if checkBlock(block) {
			fmt.Println("block passes, adding block to chain")
			fmt.Fprintf(w, "ok")
			blockchain.Blocks = append(blockchain.Blocks, block)
			fmt.Println()
		} else {
			fmt.Println("block does not pass, requesting to sync")
			http.Error(w, "block did not check out", http.StatusConflict)
			fmt.Println()
			return
		}
	})

	http.HandleFunc("/sync", func(w http.ResponseWriter, r *http.Request) {
		j, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		var otherBlockchain Blockchain
		json.Unmarshal(j, &otherBlockchain)

		if (len(otherBlockchain.Blocks) >= len(blockchain.Blocks)) {
			blockchain = otherBlockchain
			fmt.Println("new sync'd blockchain is")
			printBlockchain()
			return
		} else {
			fmt.Println("kept current blockchain after sync")
			data, _ := json.Marshal(blockchain)
			http.Error(w, string(data), http.StatusConflict)
		}
	})

	http.HandleFunc("/chain", func(w http.ResponseWriter, r *http.Request) {
		data, err := json.Marshal(blockchain)
		if err != nil {
			http.Error(w, "error", http.StatusInternalServerError)
			return
		}
		w.Write(data)
	})

	http.HandleFunc("/peers", func(w http.ResponseWriter, r *http.Request) {
		port, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "error", http.StatusInternalServerError)
			return
		}
		peerWantsToConnect(string(port))
		data, err := json.Marshal(blockchain)
		if err != nil {
			http.Error(w, "error", http.StatusInternalServerError)
			return
		}
		w.Write(data)
	})

	port := ":" + os.Args[1]
	http.ListenAndServe(port, nil)
}
