package main

import (
	"github.com/sgilson/go-sysv"
	. "github.com/sgilson/go-sysv/example"
	"log"
)

func main() {
	queueID := NewExampleQueue()
	buf, err := queueID.NewBuffer(1024)
	Must(err)
	defer buf.Close()

	err = buf.MsgSnd(sysv.MessageType(1), []byte("hello!"))
	Must(err)
	log.Println("sent message")
}
