package main

import (
	"crypto/sha1"
	"fmt"
	"math/rand"
	"strconv"
	"time"
)

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

	fmt.Println(len(sha1))
	fmt.Println("expected hash : ", hash)
	fmt.Println("calculated hash from local file : ", string(sha1String))
	return string(sha1String) == hash

}
