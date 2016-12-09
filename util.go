package main

import (
	"crypto/sha1"
	"math/rand"
	"strconv"
	"time"
)

// type BlockQueue struct {
// 	Queue []*Block
// }

// func NewBlockQueue() *BlockQueue {
// 	queue := new(BlockQueue)
// 	queue.Queue = make([]*Block, 0)
// 	return queue
// }

// func (bq *BlockQueue) push(b *Block) {
// 	bq.Queue = append(bq.Queue, b)
// }

// func (bq *BlockQueue) pop() *Block {

// }

func generatePeerId() string {
	t := time.Now().Unix()
	return strconv.FormatInt(t, 10) + RandStringBytes(10)

}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func RandStringBytes(n int) string {
	rand.Seed(time.Now().UTC().UnixNano())
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func getPeerIndex(torrent *Torrent, peer *Peer) int {
	for i, p := range torrent.PeerList {
		if p == peer {
			return i
		}
	}
	return 0
}

func checkHash(data []byte, hash string) bool {
	sha1 := (sha1.Sum(data))
	sha1String := make([]byte, len(sha1))
	for i := 0; i < len(sha1); i++ {
		sha1String[i] = sha1[i]
	}

	return string(sha1String) == hash

}
