package moles

import (
	"sync/atomic"
	"time"
)

type workerWithFunc struct {
	// 所属协程池
	cave *CaveWithFunc

	// 参数
	args chan interface{}

	// 回收时间
	recycleTime time.Time
}

// worker运行
func (wwf *workerWithFunc) run() {
	atomic.AddUint32(&wwf.cave.running, 1)
	go func() {
		for {
			arg := <-wwf.args
			if arg == nil {
				atomic.AddUint32(&wwf.cave.running, ^uint32(-(-1)-1))
				wwf.cave.cache.Put(wwf)
				break
			}
			wwf.cave.caveFunc(arg)
			if ok := wwf.cave.recycleWorker(wwf); !ok {
				break
			}
		}
	}()
}
