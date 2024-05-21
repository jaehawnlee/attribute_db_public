package dataQ

import (
	"container/list"
	"fmt"
	"sync"
	"time"
)

type Queue struct {
	data *list.List
	mut  *sync.Mutex
}

func NewQueue() *Queue {
	return &Queue{&list.List{}, &sync.Mutex{}}
}

func (q *Queue) Push(v interface{}) {
	q.mut.Lock()
	defer q.mut.Unlock()

	switch data := v.(type) {
	case string:
		t := time.Now()
		data = fmt.Sprintf("[%s] %s", t.Format("2006-01-02 15:04:05"), data)
		q.data.PushBack(data)
	default:
		q.data.PushBack(data)
	}

}

func (q *Queue) Pop() interface{} {
	q.mut.Lock()
	defer q.mut.Unlock()

	front := q.data.Front()
	if front == nil {
		return nil
	}

	return q.data.Remove(front)
}
