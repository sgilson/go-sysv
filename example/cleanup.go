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

	if err := queueID.Remove(); err != nil {
		log.Panic(err)
	}
	log.Println("deleted message queue with ID:", queueID)
}
