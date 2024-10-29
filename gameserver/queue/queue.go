// job queue for handling database operations
// keeps things smooth when we're getting hammered

package queue

import (
	"log"
	"sync"
	"time"
)

type Job struct {
	Type     string
	PlayerID string
	Data     interface{}
}

type JobQueue struct {
	queue    chan Job
	workers  int
	handler  func(Job) error
	shutdown chan struct{}
	wg       sync.WaitGroup
}

func NewJobQueue(bufferSize int, workers int, handler func(Job) error) *JobQueue {
	jq := &JobQueue{
		queue:    make(chan Job, bufferSize),
		workers:  workers,
		handler:  handler,
		shutdown: make(chan struct{}),
	}
	jq.Start()
	return jq
}

func (jq *JobQueue) Start() {
	for i := 0; i < jq.workers; i++ {
		jq.wg.Add(1)
		go jq.worker()
	}
}

func (jq *JobQueue) worker() {
	defer jq.wg.Done()
	for {
		select {
		case job := <-jq.queue:
			// Try to process the job with retries
			var err error
			for attempts := 0; attempts < 3; attempts++ {
				err = jq.handler(job)
				if err == nil {
					break
				}
				log.Printf("Job failed (attempt %d/3): %v", attempts+1, err)
				time.Sleep(time.Duration(attempts+1) * time.Second)
			}
			if err != nil {
				log.Printf("Job failed permanently: %v", err)
			}
		case <-jq.shutdown:
			return
		}
	}
}

func (jq *JobQueue) Submit(job Job) {
	select {
	case jq.queue <- job:
		// Job submitted successfully
	default:
		log.Printf("Job queue is full, dropping job: %v", job)
	}
}

func (jq *JobQueue) Shutdown() {
	close(jq.shutdown)
	jq.wg.Wait()
}
