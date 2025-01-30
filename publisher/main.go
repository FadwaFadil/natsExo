package main

import (
	"fmt"
	"log"
	"math/rand/v2"
	"nats_exercise/domain"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

func randomByteArray(len int) []byte {
	hashLen := rand.IntN(len)
	hash := make([]byte, hashLen)
	for i := 0; i < hashLen; i++ {
		hash[i] = byte(rand.IntN(0xff))
	}

	return hash
}

var titles = []string{"Hello", "Bonjour", "Buenos dias"}

func level1Msg() ([]byte, error) {
	return cbor.Marshal(domain.Lvl1Msg{
		Title: titles[rand.Int32N(int32(len(titles)))],
		Value: rand.IntN(100),
		Hash:  randomByteArray(rand.IntN(32) + 1),
	})
}

func level2Msg() ([]byte, error) {
	return cbor.Marshal(

		map[string]any{
			"Title": titles[rand.Int32N(int32(len(titles)))],
			"Value": rand.IntN(100),
			"Hash":  randomByteArray(rand.IntN(32) + 1),
		},
	)
}

func level4Msg() ([]byte, error) {
	data := map[string]any{}
	mapLen := rand.IntN(16) + 1
	dataGenerators := []func() any{
		func() any { return rand.IntN(2048) },
		func() any { return rand.IntN(2048) * -1 },
		func() any { return float64(rand.IntN(2048)) * rand.Float64() },
		func() any { return titles[rand.Int32N(int32(len(titles)))] },
		func() any { return randomByteArray(rand.IntN(32) + 1) },
	}

	for i := 0; i < mapLen; i++ {
		data[uuid.NewString()] = dataGenerators[rand.IntN(4)]()
	}

	return cbor.Marshal(data)
}

func sendLvl1Messages(nc *nats.Conn, end chan error) {
	for {
		cborMsg, err := level1Msg()
		if err != nil {
			end <- err
		}
		nc.Publish("level.one", cborMsg)

		time.Sleep(500 * time.Millisecond)
	}
}

func sendLvl2Messages(nc *nats.Conn, end chan error) {
	for {
		cborMsg, err := level2Msg()
		if err != nil {
			end <- err
		}
		nc.Publish("level.two", cborMsg)

		time.Sleep(500 * time.Millisecond)
	}
}

func sendLvl3Messages(nc *nats.Conn, end chan error) {
	for {
		dataBlocksCount := rand.IntN(3) + 1
		cborMsg := []byte{}
		for i := 0; i < dataBlocksCount; i++ {
			dataBlock, err := level2Msg()
			if err != nil {
				end <- err
			}
			cborMsg = append(cborMsg, dataBlock...)
		}
		nc.Publish("level.three", cborMsg)

		time.Sleep(500 * time.Millisecond)
	}
}

func sendLvl4Messages(nc *nats.Conn, end chan error) {
	for {
		dataBlocksCount := rand.IntN(3) + 1
		cborMsg := []byte{}
		for i := 0; i < dataBlocksCount; i++ {
			dataBlock, err := level4Msg()
			if err != nil {
				end <- err
			}
			cborMsg = append(cborMsg, dataBlock...)
		}
		nc.Publish("level.four", cborMsg)

		time.Sleep(500 * time.Millisecond)
	}
}

func main() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	nc, err := nats.Connect("nats://localhost:4222")
	if err != nil {
		log.Fatal(err)
	}
	errChan := make(chan error)

	go sendLvl1Messages(nc, errChan)
	go sendLvl2Messages(nc, errChan)
	go sendLvl3Messages(nc, errChan)
	go sendLvl4Messages(nc, errChan)

	select {
	case err := <-errChan:
		log.Fatal(err)
	case <-quit:
		fmt.Println("publisher exited successfully")
		os.Exit(0)
	}
}
