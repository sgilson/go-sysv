package main

import (
	"github.com/sgilson/go-sysv"
	"log"
)

func main() {
	queueID, err := sysv.NewQueue("queueFile", 1001, 0666|sysv.IPC_CREAT)
	if err != nil {
		log.Panic(err)
	}
	println("using queue with ID:", queueID)

	buf, err := queueID.NewBuffer(1024)
	if err != nil {
		log.Panic(err)
	}
	defer buf.Close()

	msgType, msg, err := buf.MsgRcv(sysv.MessageType(1))
	if err != nil {
		log.Panic(err)
	}

	println("got message type:", msgType)
	println("got message:", string(msg))
}
