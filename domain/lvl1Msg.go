package domain

type Lvl1Msg struct {
	Title string // a random(ish) string
	Value int    // a random int >=0 and < 100
	Hash  []byte // an array of random bytes value of length > 0 and <= 32
}
