package moles

import (
	"sync/atomic"
	"time"
)

type worker struct {
	// 所属协程池
	cave *Cave

	// 任务
	task chan func()

	// 回收时间
	recycleTime time.Time
}

// worker运行
func (w *worker) run() {
	atomic.AddUint32(&w.cave.running, 1)
	go func() {
		for {
			task := <-w.task
			if task == nil {
				atomic.AddUint32(&w.cave.running, -1)
				w.cave.cache.Put(w)
				break
			}
			task()
			if ok := w.cave.recycleWorker(w); ok {
				break
			}
		}
	}()
}
