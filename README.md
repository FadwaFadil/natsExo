# An exercise in NATS and CBOR
The goal of this exercise is to write a simple go program to read CBOR-encoded messages on different NATS subjects.
What to do with those messages, display them in the console, store them in a db table or send them back to my email address as you receive them (but if you do that one I'll be pretty upset) is left to your judgement.
A good idea to get better acquainted with NATS' features would be to store the messages content in a NATS key/value store (see NATS documentation).

## dependencies
for ease of use, we've included a `docker-compose.yaml` file containing a NATS cluster, alongside it's configuration (with jetstream already enabled).
All you need to do is to run `docker compose up -d` from the root directory of this archive to get it started.

## The exercise

Each message type will be sent on a different subject, the 4 subjects are as follow:
### `level.one`
Each message contains one single CBOR data block, encoding the `Lvl1Msg` struct found at `./domain/lvl1Msg.go`:

### `level.two`
Each message contains one single CBOR data block, encoding the same structure of data than in `level.one` but as a `map[string]any` instead.

### `level.three`
Each message contains a random number (> 0 and <= 3) of CBOR data blocks, each encoding the same data as in `level.two`

### `level.four`
Each message contains a random number of CBOR data blocks (> 0 and <= 16). Each message is a `map[string]any` with a uuid as a key and a random data type as a value.
The possible data types are:
  * positive integer (>= 0 and < 2048)
  * negative integer (<= 0 and > -2048)
  * positive float64 (>= 0.0 and < 2048.0)
  * random string
  * array of random bytes (of length > 0 and <= 32)

## Goals of this exercise
This exercise has got three main goals:

### 1. Learn your way around NATS 
In this exercise, you will learn how to connect to a NATS cluster and subscribe to different subjects, and potentially start learning about jetstream (if you choose to store the messages content in a k/v store)

### 2. Learn about CBOR
CBOR is going to be used a lot in our stack; with this exercise you'll start learning about it: how to unmarshal CBOR data blocks, how the different possible go types get encoded and how to handle
messages containing the `any` type.

### 3. Write testeable code
Although we do not think the point of this exercise should be to have the entire program covered by unit tests, at least keeping in mind it's testability and writing a few unit tests as a proof-of-concept will be a good experience for what's to come.
