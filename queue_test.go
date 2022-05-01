package sysv

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"math"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"testing"
)

const testMsgType = MessageType(1)

func TestReceiveAny(t *testing.T) {
	q := newTestQueue(t)
	buf, err := q.NewBuffer(100)
	require.NoError(t, err)
	defer buf.Close()

	msgType := testMsgType
	msg := []byte("heyo")
	require.NoError(t, buf.MsgSnd(msgType, msg))

	gotType, gotMsg, err := buf.MsgRcv(ReceiveAny, RNoWaitFlag)
	require.NoError(t, err)
	assert.Equal(t, msgType, gotType)
	assert.Equal(t, msg, gotMsg)
}

func TestReceiveNoErrorFlag(t *testing.T) {
	q := newTestQueue(t)

	snd, err := q.NewBuffer(15)
	require.NoError(t, err)
	rcv, err := q.NewBuffer(10)
	require.NoError(t, err)

	msgType := testMsgType
	require.NoError(t, snd.MsgSnd(msgType, []byte("aaaaaaaaaaaaaaa")))

	gotType, msg, err := rcv.MsgRcv(testMsgType, MsgNoErrorFlag)
	require.NoError(t, err)
	assert.Equal(t, gotType, msgType)
	assert.Equal(t, []byte("aaaaaaaaaa"), msg)
}

func TestUseAfterFree(t *testing.T) {
	buf, err := newTestQueue(t).NewBuffer(0)
	require.NoError(t, err)

	require.NoError(t, buf.Close())

	err = buf.MsgSnd(testMsgType, []byte{})
	assert.ErrorIs(t, UseAfterFreeError, err, "send")

	_, _, err = buf.MsgRcv(testMsgType)
	assert.ErrorIs(t, UseAfterFreeError, err, "receive")

	err = buf.Close()
	assert.ErrorIs(t, UseAfterFreeError, err, "close")
}

func TestSendReceiveMany(t *testing.T) {
	q := newTestQueue(t)
	msgType := MessageType(rand.Int())
	maxSize := uint64(100)
	iter := 10_000

	wg := new(sync.WaitGroup)
	wg.Add(2)

	go func() {
		rcv, err := q.NewBuffer(maxSize)
		require.NoError(t, err)
		defer rcv.Close()

		for want := 0; want < iter; want++ {
			gotType, bytes, err := rcv.MsgRcv(msgType)
			assert.NoError(t, err, "receive message")
			assert.Equal(t, msgType, gotType)
			got, err := strconv.Atoi(string(bytes))
			assert.NoError(t, err, "parse int")
			assert.Equal(t, want, got, "message number")
			if want != got {
				want = int(math.Max(float64(want), float64(got)))
			}
		}
		wg.Done()
	}()

	go func() {
		snd, err := q.NewBuffer(maxSize)
		require.NoError(t, err)
		defer snd.Close()

		for i := 0; i < iter; i++ {
			msg := []byte(strconv.Itoa(i))
			err := snd.MsgSnd(msgType, msg)
			assert.NoError(t, err, "send, iteration %d", i)
		}
		wg.Done()
	}()

	wg.Wait()
}

func newTestQueue(t *testing.T) QueueId {
	t.Helper()
	file, err := os.CreateTemp("", "ipc")
	require.NoError(t, err)
	require.NoError(t, file.Close())
	t.Cleanup(func() {
		os.Remove(file.Name())
	})

	q, err := NewQueue(file.Name(), -1, 0666|IPC_CREAT)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, q.Remove())
	})
	return q
}
