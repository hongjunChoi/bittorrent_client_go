package main

import (
	"time"
	"strconv"
	"math/rand"
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