package sysv

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"math"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"syscall"
	"testing"
	"time"
)

const testMsgType = MessageType(1)

func TestReceiveAny(t *testing.T) {
	buf, err := NewMsgBuffer(newTestQueue(t), 100)
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
	queueID := newTestQueue(t)

	snd, err := NewMsgBuffer(queueID, 15)
	require.NoError(t, err)
	rcv, err := NewMsgBuffer(queueID, 10)
	require.NoError(t, err)

	msgType := testMsgType
	require.NoError(t, snd.MsgSnd(msgType, []byte("aaaaaaaaaaaaaaa")))

	_, _, err = rcv.MsgRcv(testMsgType)
	require.Error(t, err)

	gotType, msg, err := rcv.MsgRcv(testMsgType, MsgNoErrorFlag)
	require.NoError(t, err)
	assert.Equal(t, gotType, msgType)
	assert.Equal(t, []byte("aaaaaaaaaa"), msg)
}

func TestNoResumeOnInterrupt(t *testing.T) {
	/*
		Originally there was also a test for ResumeOnInterrupt,
		but it could not be run in the same process as this test.

		The tests succeeded when run in isolation, but once either
		test sent and was interrupted by a signal, other blocking
		calls did not get an EINTR error.

		Definitely deserves more attention because this leaves the test suite
		somewhat incomplete.
	*/
	rcvFail, err := NewMsgBuffer(newTestQueue(t), 0, NoResumeOnInterrupt)
	require.NoError(t, err)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		_, _, err := rcvFail.MsgRcv(testMsgType)
		require.ErrorIs(t, err, syscall.EINTR)
		wg.Done()
	}()
	awaitMsgBufLocked(rcvFail)
	require.NoError(t, syscall.Kill(syscall.Getpid(), syscall.SIGUSR1))
	wg.Wait()
}

func TestUseAfterFree(t *testing.T) {
	buf, err := NewMsgBuffer(newTestQueue(t), 0)
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
	queueID := newTestQueue(t)
	msgType := MessageType(rand.Int())
	maxSize := uint64(100)
	iter := 10_000

	wg := new(sync.WaitGroup)
	wg.Add(2)

	go func() {
		rcv, err := NewMsgBuffer(queueID, maxSize, ResumeOnInterrupt)
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
		snd, err := NewMsgBuffer(queueID, maxSize, ResumeOnInterrupt)
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

func newTestQueue(t *testing.T) QueueID {
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

func awaitMsgBufLocked(b *MsgBuffer) {
	// Want to wait for the another goroutine to actually
	// be in the blocking system call.
	// This isn't perfect, but we can at least wait for another goroutine to
	// have acquired the lock on the queue before proceeding
	for {
		time.Sleep(10 * time.Millisecond)
		if b.lock.TryLock() {
			b.lock.Unlock()
		} else {
			break
		}
	}
	// Additional time to allow other goroutine to start
	// system call such as msgsnd or msgrcv
	time.Sleep(10 * time.Millisecond)
}
