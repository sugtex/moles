package moles

import (
	"sync"
	"sync/atomic"
	"time"
)

type Cave struct {
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
	workers []*worker

	// 对象池作为缓存
	cache sync.Pool
}

// 新建协程池
func NewCave(cap uint32, options ...Option) (*Cave, error) {
	// 容量不可为0
	if cap == 0 {
		return nil, ErrInvalidCaveCapacity
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
	c := &Cave{
		capacity:         cap,
		expiryDuration:   opts.ExpiryDuration,
		isNonBlocking:    opts.IsNonBlocking,
		maxBlockingTasks: opts.MaxBlockingTasks,
	}

	// 是否预分配内存
	if opts.IsPreAlloc {
		c.workers = make([]*worker, 0, cap)
	}

	// 信号量和互斥锁绑定
	c.cond = sync.NewCond(&c.lock)

	// 开启周期性销毁过期worker
	go c.periodicCleaning()

	return c, nil
}

// 获取当前正在运行的worker数量
func (c *Cave) GetRunningWorkers() uint32 {
	return atomic.LoadUint32(&c.running)
}

// 获得默认协程池的容量大小
func (c *Cave) Cap() uint32 {
	return atomic.LoadUint32(&c.capacity)
}

// 获得当前还可以增加的worker数量
func (c *Cave) GetFree() uint32 {
	return atomic.LoadUint32(&c.capacity) - atomic.LoadUint32(&c.running)
}

// 提交任务
func (c *Cave) SubmitTask(task func()) error {
	if atomic.LoadUint32(&c.isRelease) == CLOSED {
		return ErrCaveClosed
	}
	if w := c.getWorker(); w != nil {
		w.task <- task
	} else {
		return ErrCaveBusy
	}
	return nil
}

// 动态缩扩容量
func (c *Cave) ChangeCap(cap uint32) error {
	if cap == 0 {
		return ErrInvalidCaveCapacity
	} else if cap != c.capacity {
		atomic.StoreUint32(&c.capacity, cap)
		surplus := int(atomic.LoadUint32(&c.running)) - int(cap)
		for i := 0; i < surplus; i++ {
			c.getWorker().task <- nil
		}
	}
	return nil
}

// 释放默认协程池
func (c *Cave) Release() {
	c.once.Do(func() {
		atomic.StoreUint32(&c.isRelease, 1)
		c.lock.Lock()
		idleWorkers := c.workers
		for i, v := range idleWorkers {
			v.task <- nil
			idleWorkers[i] = nil
		}
		c.workers = nil
		c.lock.Unlock()
	})
}

// 定期清理过期的worker
func (c *Cave) periodicCleaning() {
	for {
		if atomic.LoadUint32(&c.isRelease) == CLOSED {
			break
		}
		time.Sleep(c.expiryDuration)
		now := time.Now()
		c.lock.Lock()
		idleWorkers := c.workers
		var temp []*worker
		for i, v := range idleWorkers {
			if now.Sub(v.recycleTime) > c.expiryDuration {
				v.task <- nil
				idleWorkers[i] = nil
			} else {
				temp = append(temp, v)
			}
		}
		c.workers = temp
		c.lock.Unlock()
	}
}

// 获得空闲的worker
func (c *Cave) getWorker() *worker {
	var w *worker
	c.lock.Lock()
	// 首先看running是否到达容量限制和是否存在空闲worker
	idles := c.workers
	if c.running < c.capacity && len(idles) == 0 {
		if cacheWorker := c.cache.Get(); cacheWorker != nil {
			w = cacheWorker.(*worker)
		} else {
			w = &worker{
				cave: c,
				task: make(chan func()),
			}
		}
		w.run()
	} else if c.running < c.capacity && len(idles) != 0 {
		c.currentBlocking++
		c.cond.Wait()
		c.currentBlocking--
		w = idles[0]
		c.workers = idles[1:]
	} else if c.running >= c.capacity {
		// 在阻塞模式下需要满足小于最大阻塞任务数量
		if (!c.isNonBlocking && c.currentBlocking < c.maxBlockingTasks) || c.isNonBlocking {
			c.currentBlocking++
			c.cond.Wait()
			c.currentBlocking--
			w = idles[0]
			c.workers = idles[1:]
		}
	}
	c.lock.Unlock()
	return w
}

//  回收worker
func (c *Cave) recycleWorker(w *worker) bool {
	if atomic.LoadUint32(&c.isRelease) == CLOSED {
		return false
	}
	w.recycleTime = time.Now()
	c.lock.Lock()
	c.workers = append(c.workers, w)
	c.cond.Signal()
	c.lock.Unlock()
	return true
}
