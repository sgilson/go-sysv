package sysv

/*
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <errno.h>
#include <sys/ipc.h>
#include <sys/msg.h>
*/
import "C"
import (
	"errors"
	"fmt"
	"os"
	"sync"
	"syscall"
	"unsafe"
)

const (
	IPC_PRIVATE = C.IPC_PRIVATE
	IPC_CREAT   = C.IPC_CREAT
	IPC_EXCL    = C.IPC_EXCL
	errNum      = -1
	longSize    = 8 // Both unsafe.Sizeof(C.long) and C.sizeof fail
)

var (
	InvalidMsgTypeError = errors.New("sent msgType must be greater than 0")
	MsgTooLongError     = errors.New("message cannot exceed buffer length")
	UseAfterFreeError   = errors.New("buffer has already been freed")
)

type key = int
type QueueID C.int
type MessageType int64

type SendFlag int
type ReceiveFlag int

const (

	/*SNoWaitFlag
	Return immediately if queue is full when attempting
	to send a message.
	*/
	SNoWaitFlag = SendFlag(C.IPC_NOWAIT)

	/*RNoWaitFlag
	Return immediately if no message with the requested
	type is in the queue.
	*/
	RNoWaitFlag = ReceiveFlag(C.IPC_NOWAIT)

	/*MsgCopyFlag Nondestructively copy the message at the position in the
	queue specified by the message type.
	*/
	//MsgCopyFlag = ReceiveFlag(C.MSG_COPY)

	/*MsgExceptFlag Used with msgtype greater than 0 to read the first message
	in the queue with a type that differs from msgtype.
	*/
	//MsgExceptFlag = ReceiveFlag(C.MSG_EXCEPT)

	//MsgNoErrorFlag Truncate message if longer than the buffer size.
	MsgNoErrorFlag = ReceiveFlag(C.MSG_NOERROR)

	ReceiveAny = MessageType(0)
)

/*NewQueue
Create or get the id of a queue uniquely identified by path and projectId.
This implementation ivokes ftok(path, projectId) to create the queue identifier
and uses the resulting id in a call to msgget.

Reference:
   https://man7.org/linux/man-pages/man3/ftok.3.html
   https://man7.org/linux/man-pages/man2/msgget.2.html
*/
func NewQueue(path string, projectID int, msgFlag os.FileMode) (QueueID, error) {
	key, err := ftok(path, projectID)
	if err != nil {
		return 0, err
	}
	queueId, err := msgget(key, msgFlag)
	if err != nil {
		return 0, err
	}
	return queueId, nil
}

/*Remove
Delete this queue using msgctl(queueID, IPC_RMID, NULL).

Reference:
    https://man7.org/linux/man-pages/man2/msgctl.2.html
*/
func (q QueueID) Remove() error {
	res, err := C.msgctl(C.int(q), C.IPC_RMID, (*C.struct_msqid_ds)(C.NULL))
	if res == errNum {
		return err
	}
	return nil
}

func ftok(path string, id int) (key, error) {
	res, err := C.ftok(C.CString(path), C.int(id))
	if res == errNum {
		return 0, fmt.Errorf("ftok: %w", err)
	}
	return key(res), nil
}

func msgget(key key, msgFlag os.FileMode) (QueueID, error) {
	res, err := C.msgget(C.int(key), C.int(msgFlag))
	if res == errNum {
		return 0, fmt.Errorf("msgget: %w", err)
	}
	return QueueID(res), nil
}

/*MsgBuffer
Manage a buffer used for either sending or receiving from a queue.
MsgSnd and MsgRcv can safely be invoked concurrently,
but note that only one operation will be performed at a time.
For this reason, it is recommended to use separate buffers for sending and receiving.
*/
type MsgBuffer struct {
	queueID QueueID
	size    uint64
	buffer  unsafe.Pointer

	lock   *sync.Mutex
	closed bool

	resumeOnInterrupt    bool
	numInterruptsIgnored int
}

/*NewMsgBuffer
Initialize a new buffer ready to send and receive messages
from the queue with the given QueueID.
A single buffer with the given size will be allocated and used during
msgsnd and msgrcv operations.
*/
func NewMsgBuffer(queueID QueueID, size uint64, opts ...MsgBufferOpt) (*MsgBuffer, error) {
	buffer := unsafe.Pointer(C.malloc(C.ulong(longSize + size + 1)))
	if buffer == C.NULL {
		return nil, fmt.Errorf("malloc failed")
	}

	msgBuf := &MsgBuffer{
		queueID: queueID,
		size:    size,
		buffer:  buffer,
		lock:    new(sync.Mutex),
	}
	for _, opt := range opts {
		opt(msgBuf)
	}
	return msgBuf, nil
}

/*MsgBufferOpt
Additional options that can be applied to a message buffer.
*/
type MsgBufferOpt func(buffer *MsgBuffer)

var (
	ResumeOnInterrupt MsgBufferOpt = func(b *MsgBuffer) {
		b.resumeOnInterrupt = true
	}
	NoResumeOnInterrupt MsgBufferOpt = func(b *MsgBuffer) {
		b.resumeOnInterrupt = false
	}
)

/*MsgSnd
Send a message with the given type.

The message type must be greater than 0.
The length of msg must be less than or equal to the buffer length.

By default, this operation will block if the queue is full.
Refer to the msgsnd documentation for more details.

Reference:
   https://man7.org/linux/man-pages/man2/msgsnd.2.html
*/
func (b *MsgBuffer) MsgSnd(msgType MessageType, msg []byte, flags ...SendFlag) error {
	if msgType <= 0 {
		return InvalidMsgTypeError
	}
	if uint64(len(msg)) > b.size {
		return MsgTooLongError
	}

	b.lock.Lock()
	defer b.lock.Unlock()
	if b.closed {
		return UseAfterFreeError
	}

	flag := mergeFlags(flags...)
	end := C.int(longSize + len(msg) + 1)
	*(*C.long)(b.buffer) = C.long(msgType)
	if len(msg) > 0 {
		C.strcpy((*C.char)(unsafe.Add(b.buffer, longSize)), (*C.char)(unsafe.Pointer(&msg[0])))
	}
	*(*C.char)(unsafe.Add(b.buffer, int(end))) = 0 // append null byte

	for {
		ret, err := C.msgsnd(C.int(b.queueID), b.buffer, C.ulong(b.size), flag)
		if ret == errNum {
			if errors.Is(err, syscall.EINTR) && b.resumeOnInterrupt {
				b.numInterruptsIgnored++
				continue
			}
			return err
		}
		return nil
	}
}

/*MsgRcv
Receive a message with the given type.

The ReceiveAny type can be used to consume messages of any type from the queue,
but care should be taken to ensure the current buffer length is large enough to
hold any possible message produced by your project.

The byte slice returned by this function excludes all bytes including and after
the first null byte in the message buffer.

By default, this call will block until a matching message arrives,
the queue is deleted, or this process is interrupted.
Refer to the msgrcv documentation for more details.

Reference:
   https://man7.org/linux/man-pages/man2/msgrcv.2.html.
*/
func (b *MsgBuffer) MsgRcv(msgType MessageType, flags ...ReceiveFlag) (MessageType, []byte, error) {
	b.lock.Lock()
	defer b.lock.Unlock()
	if b.closed {
		return 0, nil, UseAfterFreeError
	}

	for {
		ret, err := C.msgrcv(C.int(b.queueID), b.buffer, C.ulong(b.size), C.long(msgType), mergeFlags(flags...))
		if ret == errNum {
			if errors.Is(err, syscall.EINTR) && b.resumeOnInterrupt {
				b.numInterruptsIgnored++
				continue
			}
			return 0, nil, err
		}
		break
	}

	result := make([]byte, b.size)

	gotMsgType := MessageType(*(*C.long)(b.buffer))
	if b.size > 0 {
		C.memcpy(
			unsafe.Pointer(&result[0]),
			unsafe.Add(b.buffer, longSize),
			C.ulong(b.size))
	}
	return gotMsgType, result[:firstNullByteIdx(result)], nil
}

// Close Release underlying buffer.
// All subsequent calls on this buffer will fail with a UseAfterFreeError.
func (b *MsgBuffer) Close() error {
	b.lock.Lock()
	defer b.lock.Unlock()
	if b.closed {
		return UseAfterFreeError
	}
	C.free(b.buffer)
	b.closed = true
	return nil
}

func (b *MsgBuffer) safeNumInterruptsIgnored() int {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.numInterruptsIgnored
}

func firstNullByteIdx(buf []byte) int {
	for i, b := range buf {
		if b == 0 {
			return i
		}
	}
	return len(buf)
}

func mergeFlags[T SendFlag | ReceiveFlag](flags ...T) C.int {
	flag := 0
	for _, f := range flags {
		flag = flag | int(f)
	}
	return C.int(flag)
}
