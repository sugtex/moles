package moles

import (
	"fmt"
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
		for task := range w.task {
			if task == nil {
				atomic.AddUint32(&w.cave.running, ^uint32(-(-1)-1))
				fmt.Println("回收到对象池")
				w.cave.cache.Put(w)
				break
			}
			task()
			if ok := w.cave.recycleWorker(w); !ok {
				break
			}
		}
	}()
}
