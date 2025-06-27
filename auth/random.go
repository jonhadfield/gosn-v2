package auth

import (
	crand "crypto/rand"
	"encoding/binary"
	"math/rand"

	"github.com/jonhadfield/gosn-v2/log"
)

// cryptoSource allows math/rand to use crypto/rand for randomness.
type cryptoSource struct{}

func (s cryptoSource) Seed(seed int64) {}

func (s cryptoSource) Int63() int64 {
	return int64(s.Uint64() & ^uint64(1<<63))
}

func (s cryptoSource) Uint64() (v uint64) {
	if err := binary.Read(crand.Reader, binary.BigEndian, &v); err != nil {
		log.Fatal(err.Error())
	}
	return v
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

// randomString returns a crypto-random string of the requested length.
func randomString(n int) string {
	var src cryptoSource
	rnd := rand.New(src)
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rnd.Intn(len(letterRunes))]
	}
	return string(b)
}
