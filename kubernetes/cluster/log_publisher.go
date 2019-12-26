package cluster

import (
	"sync"
)

//日志最大缓冲数
const (
	MAX_LOG_SIZE = 2096
)

//其他使用者(web服务)用来读取安装信息的对象
type LogReader struct {
	publisher *LogPublisher
	c         chan *string
}

func (sr *LogReader) Read() *string {
	v, ok := <-sr.c
	if ok {
		return v
	}
	return nil
}

func (sr *LogReader) Close() {
	sr.publisher.removeReader(sr)
	close(sr.c)
}

type logData struct {
	data string
	next *logData
}

//日志缓冲和共享
type LogPublisher struct {
	readerLock sync.RWMutex // readers对象锁
	readers    map[*LogReader]*LogReader
	logsLock   sync.RWMutex // readers对象锁
	logsCount  int
	firstLog   *logData
	currLog    *logData
}

func NewPublisher() *LogPublisher {
	return &LogPublisher{
		readers:   make(map[*LogReader]*LogReader),
		logsCount: 0,
		firstLog:  nil,
		currLog:   nil,
	}
}

func (sp *LogPublisher) Log(data string) {
	sp.logsLock.Lock()
	if sp.firstLog == nil {
		sp.firstLog = &logData{
			data: data,
			next: nil,
		}
		sp.currLog = sp.firstLog
	} else {
		sp.currLog.next = &logData{
			data: data,
			next: nil,
		}
		sp.currLog = sp.currLog.next
	}
	sp.logsCount++
	if sp.logsCount > MAX_LOG_SIZE {
		sp.firstLog = sp.firstLog.next
	}
	sp.logsLock.Unlock()

	sp.readerLock.RLock()
	defer sp.readerLock.RUnlock()
	for reader, _ := range sp.readers {
		reader.c <- &data
	}
}

func (sp *LogPublisher) CreateReader() *LogReader {
	reader := &LogReader{
		publisher: sp,
		c:         make(chan *string, MAX_LOG_SIZE),
	}

	sp.logsLock.RLock()
	log := sp.firstLog
	for log != nil {
		reader.c <- &log.data
		log = log.next
	}
	sp.logsLock.RUnlock()
	sp.readerLock.Lock()
	defer sp.readerLock.Unlock()
	sp.readers[reader] = reader
	return reader
}

func (sp *LogPublisher) removeReader(reader *LogReader) {
	sp.readerLock.Lock()
	defer sp.readerLock.Unlock()
	delete(sp.readers, reader)
}
