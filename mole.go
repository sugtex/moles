package moles

import (
	"errors"
	"time"
)

const (
	// 默认协程池大小
	DEFAULT_MOLES_CAVE_SIZE = 1000000

	// 每个worker的过期时间
	DEFAULT_EXPIRY_TIME = 8

	// 默认最大阻塞数量
	DEFAULT_MAX_BLOCKING_TASKS = 10000

	// 关闭状态
	CLOSED = 1
)

var (
	ErrInvalidCaveCapacity = errors.New("无效协程池容量")

	ErrLackCaveFunc = errors.New("未提供功能")

	ErrInvalidCaveExpiry = errors.New("无法设置过期时间,不可为负数")

	ErrCaveBusy = errors.New("太多任务阻塞中")

	ErrCaveClosed = errors.New("协程池早已关闭")

	// 为用户提供一个默认的协程池
	defaultMolesCave, _ = NewCave(DEFAULT_MOLES_CAVE_SIZE)
)

// 提供设置接口
type Option func(opts *Options)

// 设置
type Options struct {
	// 是否预分配内存
	IsPreAlloc bool

	// 是否为非阻塞模式
	IsNonBlocking bool

	// 每个worker的过期时间
	ExpiryDuration time.Duration

	// Submit允许阻塞的最大限制(默认为0)
	MaxBlockingTasks uint32
}

// 用户可以声明Options结构体定义变量
func WithOptions(options *Options) Option {
	return func(opts *Options) {
		options = opts
	}
}

// 设置过期时间
func WithExpiryDuration(expiryDuration time.Duration) Option {
	return func(opts *Options) {
		opts.ExpiryDuration = expiryDuration
	}
}

// 设置Submit允许阻塞的最大限制
func WithMaxBlockingTasks(maxBlockingTasks uint32) Option {
	return func(opts *Options) {
		opts.MaxBlockingTasks = maxBlockingTasks
	}
}

// 设置是否阻塞
func WithNonblocking(nonBlocking bool) Option {
	return func(opts *Options) {
		opts.IsNonBlocking = nonBlocking
	}
}

// 默认协程池

// 提交任务
func Submit(task func()) error {
	return defaultMolesCave.SubmitTask(task)
}

// 获取当前正在运行的worker数量
func GetRunningWorkers() uint32 {
	return defaultMolesCave.GetRunningWorkers()
}

// 获得默认协程池的容量大小
func Cap() uint32 {
	return defaultMolesCave.Cap()
}

// 获得当前还可以增加的worker数量
func GetFree() uint32 {
	return defaultMolesCave.GetFree()
}

// 释放默认协程池
func Release() {
	defaultMolesCave.Release()
}
