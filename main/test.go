package main

import (
	"cke/log"
	"time"
)

func waitToUpdate() <-chan string {
	updateChan := make(chan string)

	go func() {
		for i := 0; i < 10; i++ {
			time.Sleep(time.Second)
			log.Info("Count is ", i)
		}
		updateChan <- "update task ..."
	}()

	return updateChan
}

func testMain() {
	updateChannel := waitToUpdate()
	msg := <-updateChannel
	log.Info("close msg:", msg)

}
