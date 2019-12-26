package cluster

import (
	"strings"
	"sync"
	"testing"
)

func TestNewLogger(t *testing.T) {
	publisher := NewPublisher()
	reader := publisher.CreateReader()
	reader.Close()
}

func TestAddLog(t *testing.T) {
	var wg sync.WaitGroup
	readFunc := func(reader *LogReader) {
		for {
			log := reader.Read()
			if log == nil {
				break
			} else if !strings.Contains(*log, "test") {
				t.Errorf("error: string [%s] does not contain test", *log)
			}
		}
		wg.Done()
	}

	publisher := NewPublisher()
	reader1 := publisher.CreateReader()
	reader2 := publisher.CreateReader()

	wg.Add(1)
	go readFunc(reader1)

	wg.Add(1)
	go readFunc(reader2)

	publisher.Log("test1")
	publisher.Log("test2")
	publisher.Log("test3")

	reader1.Close()
	reader2.Close()
	wg.Wait()
}

func TestMaxLog(t *testing.T) {
	publisher := NewPublisher()

	for i := 0; i < MAX_LOG_SIZE+1; i++ {
		publisher.Log("test")
	}

	k := 0
	log := publisher.firstLog
	for log != nil {
		k++
		log = log.next
	}
	if k != MAX_LOG_SIZE {
		t.Errorf("TestMaxLog error log count:%d, MAX_LOG_SIZE:%d ", k, MAX_LOG_SIZE)
	}
}

func TestHistoryLog(t *testing.T) {
	const (
		TEST_COUNT = 10
	)
	var wg sync.WaitGroup
	readFunc := func(reader *LogReader) {
		count := 0
		for {
			log := reader.Read()
			if log == nil {
				break
			} else if !strings.Contains(*log, "test") {
				t.Errorf("error: string [%s] does not contain test", *log)
			}
			count++
		}
		if count != TEST_COUNT*2 {
			t.Errorf("Test history log error: hope count:%d and actual count:%d", TEST_COUNT, count)
		}
		wg.Done()
	}

	publisher := NewPublisher()

	for i := 0; i < TEST_COUNT; i++ {
		publisher.Log("test1")
	}

	reader := publisher.CreateReader()

	wg.Add(1)
	go readFunc(reader)

	for i := 0; i < TEST_COUNT; i++ {
		publisher.Log("test2")
	}

	reader.Close()
	wg.Wait()
}
