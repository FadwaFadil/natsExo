package main

import (
	"bytes"
	"context"
	"errors"
	"log"
	"testing"

	"nats_exercise/domain"

	"github.com/fxamacker/cbor/v2"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type KVStoreMock struct {
	mock.Mock
}

func (c *KVStoreMock) Put(ctx context.Context, key string, value []byte) (uint64, error) {
	args := c.Called(ctx, key, value)
	return args.Get(0).(uint64), args.Error(1)
}

type NATSClientMock struct {
	mock.Mock
}

func (n *NATSClientMock) Subscribe(subject string, cb nats.MsgHandler) (*nats.Subscription, error) {
	args := n.Called(subject, cb)
	return args.Get(0).(*nats.Subscription), args.Error(1)
}

func TestProcessMessage(t *testing.T) {
	testCases := []struct {
		name         string
		msg          *nats.Msg
		setup        func(*KVStoreMock)
		expectedLogs string
		assertion    assert.ErrorAssertionFunc
	}{
		{
			name: "Valid Level 1 message",
			msg: &nats.Msg{
				Subject: "level.one",
				Data: func() []byte {
					data, err := cbor.Marshal(domain.Lvl1Msg{
						Title: "Test Title",
						Value: 123,
						Hash:  []byte("testhash"),
					})
					if err != nil {
						t.Fatalf("failed to marshal message: %v", err)
					}
					return data
				}(),
			},
			setup: func(kv *KVStoreMock) {
				kv.On("Put", mock.Anything, "level.one.title.1", []byte("Test Title")).Return(uint64(1), nil)
				kv.On("Put", mock.Anything, "level.one.value.1", []byte("123")).Return(uint64(1), nil)
				kv.On("Put", mock.Anything, "level.one.hash.1", []byte("testhash")).Return(uint64(1), nil)
			},
			expectedLogs: "Message stored in KV store",
			assertion: func(tt assert.TestingT, err error, i ...interface{}) bool {
				return assert.Nil(tt, err)
			},
		},
		{
			name: "Invalid CBOR data",
			msg: &nats.Msg{
				Subject: "level.one",
				Data:    []byte("invalid data"),
			},
			setup: func(kv *KVStoreMock) {
				kv.AssertNotCalled(t, "Put")
			},
			expectedLogs: "Error unmarshalling message",
			assertion: func(tt assert.TestingT, err error, i ...interface{}) bool {
				return assert.NotNil(tt, err)
			},
		},
		{
			name: "KV Store Put Failure",
			msg: &nats.Msg{
				Subject: "level.one",
				Data: func() []byte {
					data, err := cbor.Marshal(domain.Lvl1Msg{
						Title: "Test Title",
						Value: 123,
						Hash:  []byte("testhash"),
					})
					if err != nil {
						t.Fatalf("failed to marshal message: %v", err)
					}
					return data
				}(),
			},
			setup: func(kv *KVStoreMock) {
				kv.On("Put", mock.Anything, mock.Anything, mock.Anything).Return(uint64(0), errors.New("failed to store"))
			},
			expectedLogs: "Error storing message in KV store",
			assertion: func(tt assert.TestingT, err error, i ...interface{}) bool {
				return assert.NotNil(tt, err)
			},
		},
		{
			name: "Valid Level 3 message",
			msg: &nats.Msg{
				Subject: "level.three",
				Data: func() []byte {
					cborMsg := []byte{}
					data1, err := cbor.Marshal(map[string]any{
						"Title": "Test Title 1",
						"Value": 123,
						"Hash":  []byte("testhash1"),
					})
					if err != nil {
						t.Fatalf("failed to marshal message: %v", err)
					}
					cborMsg = append(cborMsg, data1...)
					data2, err := cbor.Marshal(map[string]any{
						"Title": "Test Title 2",
						"Value": 456,
						"Hash":  []byte("testhash2"),
					})
					if err != nil {
						t.Fatalf("failed to marshal message: %v", err)
					}
					cborMsg = append(cborMsg, data2...)

					return cborMsg
				}(),
			},
			setup: func(kv *KVStoreMock) {
				kv.On("Put", mock.Anything, "level.three.Title.1", []byte("Test Title 1")).Return(uint64(1), nil)
				kv.On("Put", mock.Anything, "level.three.Value.1", []byte("123")).Return(uint64(1), nil)
				kv.On("Put", mock.Anything, "level.three.Hash.1", []byte("testhash1")).Return(uint64(1), nil)
				kv.On("Put", mock.Anything, "level.three.Title.2", []byte("Test Title 2")).Return(uint64(1), nil)
				kv.On("Put", mock.Anything, "level.three.Value.2", []byte("456")).Return(uint64(1), nil)
				kv.On("Put", mock.Anything, "level.three.Hash.2", []byte("testhash2")).Return(uint64(1), nil)
			},
			expectedLogs: "Message stored in KV store",
			assertion: func(tt assert.TestingT, err error, i ...interface{}) bool {
				return assert.Nil(tt, err)
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			kVStoreMock := new(KVStoreMock)
			tt.setup(kVStoreMock)
			msgDep := &messageDep{
				ctx: context.Background(),
				kv:  kVStoreMock,
			}

			var buf bytes.Buffer
			log.SetOutput(&buf)
			defer log.SetOutput(nil)

			msgDep.processMessage(tt.msg)

			kVStoreMock.AssertExpectations(t)
			require.Contains(t, buf.String(), tt.expectedLogs)
		})
	}
}
