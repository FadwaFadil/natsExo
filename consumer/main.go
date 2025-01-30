package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"nats_exercise/domain"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/fxamacker/cbor/v2"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type NATSClient interface {
	Subscribe(subject string, cb nats.MsgHandler) (*nats.Subscription, error)
}

type KVStore interface {
	Put(ctx context.Context, key string, value []byte) (uint64, error)
}

type messageDep struct {
	ctx   context.Context
	kv    KVStore
	nc    NATSClient
	count int
}

func main() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	nc, err := nats.Connect("nats://localhost:4222")
	if err != nil {
		log.Fatal(err)
	}
	defer nc.Close()

	errChan := make(chan error)

	ctx := context.Background()

	js, err := jetstream.New(nc)
	if err != nil {
		log.Fatal(err)
	}

	kv, err := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: "level-messages"})
	if err != nil {
		log.Fatal("Error opening KV store: ", err)
	}

	entry, err := kv.Get(ctx, "count")
	var count int
	if err != nil {
		log.Println("No previous count found, initializing to 0")
		count = 0
	} else {
		count, err = strconv.Atoi(string(entry.Value()))
		if err != nil {
			log.Println("Error converting count value, initializing to 0.")
			count = 0
		}
	}

	msgDep := &messageDep{
		ctx: ctx,
		kv:  kv,
		nc:  nc,
		count: count,
	}

	go msgDep.startConsumer("level.*", errChan)

	defer func(){
		_, err := msgDep.kv.Put(msgDep.ctx, "count", []byte(fmt.Sprintf("%d", msgDep.count)))
	if err != nil {
		log.Printf("🚨 Error storing the count value in KV store  %v:" ,err)
	} else {
		log.Println("Count value stored in KV store:", msgDep.count)
	}
	}()

	select {
	case err := <-errChan:
		log.Fatal(err)
	case <-quit:
		fmt.Println("Consumer exited successfully")
	}
}

func (msgDep *messageDep) startConsumer(subject string, end chan error) {
	_, err := msgDep.nc.Subscribe(subject, msgDep.processMessage)
	if err != nil {
		end <- err
	}
}

func (msgDep *messageDep) processMessage(m *nats.Msg) {
	decoder := cbor.NewDecoder(bytes.NewReader(m.Data))
	log.Printf("====================================== Received message from %s ======================================", m.Subject)

	if m.Subject == "level.one" {
		var msg domain.Lvl1Msg
		err := decoder.Decode(&msg)
		if err != nil {
			log.Printf("🚨Error unmarshalling message from %s: %v", m.Subject, err)
			return
		}
		log.Printf("++++++++++++ Decoded msg : %v ++++++++++++ ", msg)
		msgDep.ConvertAndStoreLvl1(msg)
	} else {
		for {

			var msg map[string]any
			err := decoder.Decode(&msg)
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				log.Printf("🚨 Error unmarshalling message from %s: %v", m.Subject, err)
				break
			}
			log.Printf("++++++++++++ Decoded msg : %v ++++++++++++ ", msg)
			msgDep.ConvertAndStoreLVL234(msg, m.Subject)
		}
	}
}

func (msgDep *messageDep) storeKV(key string, value []byte) {

	_, err := msgDep.kv.Put(msgDep.ctx, key, value)
	if err != nil {
		log.Printf("🚨 Error storing message in KV store  %s: %v", key, err)
	} else {
		log.Printf("Message stored in KV store: \n\tkey= %s: \n\tvalue= %v", key, value)
	}
}

func (msgDep *messageDep) ConvertAndStoreLvl1(msg domain.Lvl1Msg) {
	msgDep.count++
	msgDep.storeKV("level.one.title."+fmt.Sprintf("%v", msgDep.count), []byte(msg.Title))
	msgDep.storeKV("level.one.value."+fmt.Sprintf("%v", msgDep.count), []byte(fmt.Sprintf("%d", msg.Value)))
	msgDep.storeKV("level.one.hash."+fmt.Sprintf("%v", msgDep.count), msg.Hash)
}

func (msgDep *messageDep) ConvertAndStoreLVL234(msg map[string]any, subject string) {
	msgDep.count++
	for k, v := range msg {
		key := subject + "." + k + "." + fmt.Sprintf("%v", msgDep.count)
		switch v := v.(type) {
		case string:
			msgDep.storeKV(key, []byte(v))
		case int64, uint64:
			msgDep.storeKV(key, []byte(fmt.Sprintf("%d", v)))
		case float64:
			msgDep.storeKV(key, []byte(fmt.Sprintf("%f", v)))
		case []byte:
			msgDep.storeKV(key, v)
		default:
			log.Printf("🚨 Unable to convert and store %T\n", v)
		}
	}
}
