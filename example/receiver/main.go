package main

import (
	"github.com/sgilson/go-sysv"
	. "github.com/sgilson/go-sysv/example"
)

func main() {
	queueID := NewExampleQueue()
	buf, err := sysv.NewMsgBuffer(queueID, 1024, sysv.ResumeOnInterrupt)
	Must(err)
	defer buf.Close()

	msgType, msg, err := buf.MsgRcv(sysv.MessageType(1))
	Must(err)

	println("got message type:", msgType)
	println("got message:", string(msg))
}
