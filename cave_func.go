package moles

import (
	"sync"
	"sync/atomic"
	"time"
)

type CaveWithFunc struct {
	// 是否为非阻塞模式
	isNonBlocking bool

	// 当关闭该Pool支持通知所有worker退出运行以防goroutine泄露
	isRelease uint32

	// 容量
	capacity uint32

	// 运行的worker数量
	running uint32

	// Submit允许阻塞的最大限制
	maxBlockingTasks uint32

	// 当前阻塞的worker数量
	currentBlocking uint32

	// worker的过期时间
	expiryDuration time.Duration

	// 互斥锁
	lock sync.Mutex

	// 信号量
	cond *sync.Cond

	// 确保关闭操作只执行一次
	once sync.Once

	// 空闲的worker队列
	workers []*workerWithFunc

	// 对象池作为缓存
	cache sync.Pool

	// 单一方法
	caveFunc func(interface{})
}

// 新建协程池
func NewCaveWithFunc(cap uint32, cf func(interface{}), options ...Option) (*CaveWithFunc, error) {
	// 容量不可为0
	if cap == 0 {
		return nil, ErrInvalidCaveCapacity
	}
	if cf == nil {
		return nil, ErrLackCaveFunc
	}

	// 将所有设置函数调用后赋值
	opts := new(Options)
	for _, option := range options {
		option(opts)
	}

	// 判断过期时间是否合适
	if expiry := opts.ExpiryDuration; expiry < 0 {
		return nil, ErrInvalidCaveExpiry
	} else if expiry == 0 {
		opts.ExpiryDuration = DEFAULT_EXPIRY_TIME * time.Second
	}

	// 未设定最大阻塞值则为默认值
	if opts.MaxBlockingTasks == 0 {
		opts.MaxBlockingTasks = DEFAULT_MAX_BLOCKING_TASKS
	}

	// 将数据赋值
	c := &CaveWithFunc{
		capacity:         cap,
		expiryDuration:   opts.ExpiryDuration,
		isNonBlocking:    opts.IsNonBlocking,
		maxBlockingTasks: opts.MaxBlockingTasks,
	}

	// 是否预分配内存
	if opts.IsPreAlloc {
		c.workers = make([]*workerWithFunc, 0, cap)
	}

	// 信号量和互斥锁绑定
	c.cond = sync.NewCond(&c.lock)

	// 开启周期性销毁过期worker
	go c.periodicCleaning()

	return c, nil
}

// 获取当前正在运行的worker数量
func (cwf *CaveWithFunc) GetRunningWorkers() uint32 {
	return atomic.LoadUint32(&cwf.running)
}

// 获得默认协程池的容量大小
func (cwf *CaveWithFunc) Cap() uint32 {
	return atomic.LoadUint32(&cwf.capacity)
}

// 获得当前还可以增加的worker数量
func (cwf *CaveWithFunc) GetFree() uint32 {
	return atomic.LoadUint32(&cwf.capacity) - atomic.LoadUint32(&cwf.running)
}

// 提交参数
func (cwf *CaveWithFunc) SubmitArg(arg interface{}) error {
	if atomic.LoadUint32(&cwf.isRelease) == CLOSED {
		return ErrCaveClosed
	}
	if w := cwf.getWorker(); w != nil {
		// 并发点,就算协程池关闭了,把task给worker也没用了,所以不考虑并发问题
		w.args <- arg
	} else {
		return ErrCaveBusy
	}
	return nil
}

// 动态缩扩容量
func (cwf *CaveWithFunc) ChangeCap(cap uint32) error {
	if cap == 0 {
		return ErrInvalidCaveCapacity
	} else if cap != cwf.capacity {
		atomic.StoreUint32(&cwf.capacity, cap)
		surplus := int(atomic.LoadUint32(&cwf.running)) - int(cap)
		for i := 0; i < surplus; i++ {
			cwf.getWorker().args <- nil
		}
	}
	return nil
}

// 释放默认协程池
func (cwf *CaveWithFunc) Release() {
	cwf.once.Do(func() {
		atomic.StoreUint32(&cwf.isRelease, 1)
		cwf.lock.Lock()
		idleWorkers := cwf.workers
		for i, v := range idleWorkers {
			v.args <- nil
			idleWorkers[i] = nil
		}
		cwf.workers = nil
		cwf.lock.Unlock()
	})
}

// 定期清理过期的worker
func (cwf *CaveWithFunc) periodicCleaning() {
	for {
		if atomic.LoadUint32(&cwf.isRelease) == CLOSED {
			break
		}
		time.Sleep(cwf.expiryDuration)
		now := time.Now()
		cwf.lock.Lock()
		idleWorkers := cwf.workers
		var temp []*workerWithFunc
		for i, v := range idleWorkers {
			if now.Sub(v.recycleTime) > cwf.expiryDuration {
				v.args <- nil
				idleWorkers[i] = nil
			} else {
				temp = append(temp, v)
			}
		}
		cwf.workers = temp
		cwf.lock.Unlock()
	}
}

// 获得空闲的worker
func (cwf *CaveWithFunc) getWorker() *workerWithFunc {
	var w *workerWithFunc
	cwf.lock.Lock()
	// 首先看running是否到达容量限制和是否存在空闲worker
	idles := cwf.workers
	if cwf.running < cwf.capacity && len(idles) == 0 {
		if cacheWorker := cwf.cache.Get(); cacheWorker != nil {
			w = cacheWorker.(*workerWithFunc)
		} else {
			w = &workerWithFunc{
				cave: cwf,
				args: make(chan interface{}),
			}
		}
		w.run()
	} else if cwf.running < cwf.capacity && len(idles) != 0 {
		cwf.currentBlocking++
		cwf.cond.Wait()
		cwf.currentBlocking--
		w = idles[0]
		cwf.workers = idles[1:]
	} else if cwf.running >= cwf.capacity {
		// 在阻塞模式下需要满足小于最大阻塞任务数量
		if (!cwf.isNonBlocking && cwf.currentBlocking < cwf.maxBlockingTasks) || cwf.isNonBlocking {
			cwf.currentBlocking++
			cwf.cond.Wait()
			cwf.currentBlocking--
			w = idles[0]
			cwf.workers = idles[1:]
		}
	}
	cwf.lock.Unlock()
	return w
}

//  回收worker
func (cwf *CaveWithFunc) recycleWorker(w *workerWithFunc) bool {
	if atomic.LoadUint32(&cwf.isRelease) == CLOSED {
		return false
	}
	w.recycleTime = time.Now()
	cwf.lock.Lock()
	cwf.workers = append(cwf.workers, w)
	cwf.cond.Signal()
	cwf.lock.Unlock()
	return true
}
