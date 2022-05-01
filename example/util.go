package example

import (
	"github.com/sgilson/go-sysv"
	"log"
	"path"
	"runtime"
)

func Must(err error) {
	if err != nil {
		log.Panic(err)
	}
}

func NewExampleQueue() sysv.QueueID {
	// Want to always get path to queueFile adjacent to this source file
	_, sourceFilePath, _, ok := runtime.Caller(0)
	if !ok {
		log.Panic("failed to lookup path of source file")
	}

	queueFilePath := path.Join(path.Dir(sourceFilePath), "queueFile")

	queueID, err := sysv.NewQueue(queueFilePath, 1001, 0666|sysv.IPC_CREAT)
	Must(err)
	println("using queue with ID:", queueID)
	return queueID
}
