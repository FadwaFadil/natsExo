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
	"syscall"

	"github.com/fxamacker/cbor/v2"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

var (
	ErrUnmarshallingCBOR = errors.New("error unmarshalling message")
	ErrStoringKV = errors.New("error storing message in KV store") 

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
	count uint64
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

	// Initialize JetStream 
	js, err := jetstream.New(nc)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	// Create or open a key-value store
	kv, err := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: "level-messages"})
	if err != nil {
		log.Fatal("Error opening KV store: ", err)
	}

	// Get the status of the key-value store 
	kvStatus, err := kv.Status(ctx)
	if err != nil {
		errChan <- err
	}
 
	msgDep := &messageDep{
		ctx:   ctx,
		kv:    kv,
		nc:    nc,
		count: kvStatus.Values() + 1, // current sequence
	}
 
	go msgDep.startConsumer("level.*", errChan)
 
	select {
	case err := <-errChan:
		log.Fatal(err)
	case <-quit:
		log.Println("Consumer exited successfully")
	}
}
 
func (msgDep *messageDep) startConsumer(subject string, end chan error) {
	_, err := msgDep.nc.Subscribe(subject, func(msg *nats.Msg) {
		err := msgDep.processMessage(msg)
		if err != nil {
			log.Printf("Error processing message: %v", err)
		}
	})
	if err != nil {
		end <- err
	}
}

// Process incoming NATS messages
func (msgDep *messageDep) processMessage(m *nats.Msg) error {
	decoder := cbor.NewDecoder(bytes.NewReader(m.Data))
	log.Printf("====================================== Received message from %s ======================================", m.Subject)

	if m.Subject == "level.one" {
		var msg domain.Lvl1Msg
		err := decoder.Decode(&msg)
		if err != nil { 
			return fmt.Errorf("%w from %s: %v", ErrUnmarshallingCBOR, m.Subject, err)
			
		}
		log.Printf("++++++++++++ Decoded msg : %v ++++++++++++ ", msg)
		err = msgDep.ConvertAndStoreLvl1(msg)
		if err != nil {
			return err
		}
	} else {
		for {
			var msg map[string]any
			err := decoder.Decode(&msg)
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				} 
				return fmt.Errorf("%w from %s: %v", ErrUnmarshallingCBOR, m.Subject, err)
				
			}
			log.Printf("++++++++++++ Decoded msg : %v ++++++++++++ ", msg)
			err = msgDep.ConvertAndStoreLVL234(msg, m.Subject)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// Store a key-value pair in the key-value store
func (msgDep *messageDep) storeKV(key string, value []byte) error{
	_, err := msgDep.kv.Put(msgDep.ctx, key, value)
	if err != nil {
		return fmt.Errorf("%w %s: %v", ErrStoringKV, key, err)
	} else {
		log.Printf("Message stored in KV store: \n\tkey= %s: \n\tvalue= %v", key, value)
		msgDep.count++
		return nil
	}
}

//  Convert and store level one messages
func (msgDep *messageDep) ConvertAndStoreLvl1(msg domain.Lvl1Msg) error {
	err := msgDep.storeKV("level.one.title."+fmt.Sprintf("%v", msgDep.count), []byte(msg.Title))
	if err != nil {
		return err
	}
	err = msgDep.storeKV("level.one.value."+fmt.Sprintf("%v", msgDep.count), []byte(fmt.Sprintf("%d", msg.Value)))
	if err != nil {
		return err
	}
	err = msgDep.storeKV("level.one.hash."+fmt.Sprintf("%v", msgDep.count), msg.Hash)
	if err != nil {
		return err
	}
	return nil
}

// Convert and store level two, three, and four messages

func (msgDep *messageDep) ConvertAndStoreLVL234(msg map[string]any, subject string) error {
	for k, v := range msg {
		key := subject + "." + k + "." + fmt.Sprintf("%v", msgDep.count)
		var value []byte
		switch v := v.(type) {
		case string:
			value = []byte(v)
		case int64, uint64:
			value = []byte(fmt.Sprintf("%d", v))
		case float64:
			value = []byte(fmt.Sprintf("%f", v))
		case []byte:
			value = v
		default:
			return fmt.Errorf("unable to convert and store %T", v)
		}
		if err := msgDep.storeKV(key, value); err != nil {
			return err
		}
	}
	return nil
}