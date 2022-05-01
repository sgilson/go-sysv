package main

import (
	. "github.com/sgilson/go-sysv/example"
	"log"
)

func main() {
	queueID := NewExampleQueue()
	Must(queueID.Remove())
	log.Println("deleted message queue with ID:", queueID)
}
