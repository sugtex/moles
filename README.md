<h1 align='center'>moles</h1>
<p align="center">
<img src="https://github.com/CatBluePoor/moles/blob/master/picture/moles.jpg"/>
<b>A goroutine cave for Go</b>
<br/><br/>
	
## 📖 简介

`moles`是一个高性能的协程池，实现了对大规模 goroutine 的调度管理、goroutine 复用，
允许使用者在开发并发程序的时候限制协程数量，复用资源，达到更高效执行任务的效果。

## 🚀 功能

- 自动调度海量的 goroutines，复用 goroutines
- 定时清理过期的 goroutines，进一步节省资源
- 提供了友好的接口：任务提交、获取运行中的协程数量、动态调整协程池大小
- 资源复用，极大节省内存使用量；在大规模批量并发任务场景下比原生 goroutine 并发具有[更高的性能]
- 非阻塞机制

## 🧰 安装
``` powershell
go get -u github.com/CatBluePoor/moles
```

## 🛠 使用

### 使用默认协程池(只支持非限定任务)。默认配置:容量=1000000,过期时间=8秒,阻塞模式,最大阻塞数量=10000
```go
package main

import (
	"fmt"
	"github.com/CatBluePoor/moles"
)

// 使用默认协程池
func main() {
	defer moles.Release() // 释放协程池
	moles.Submit(Test)    // 提交任务
}
func Test() {
	fmt.Println("test")
}
```

### 用户也可自定义容量大小
```go
cave, err := moles.NewCave(10000) // 自定义大小协程池（参数须为大于0）
```

### 整体配置
```go
moles.WithOptions()// 整体配置
```
#### 例子
```go
// 配置
opts := &moles.Options{
	IsPreAlloc:       false, // 是否预分配内存
	IsNonBlocking:    false, // 是否为非阻塞模式
	ExpiryDuration:   5,     // 每个worker的过期时间(秒)
	MaxBlockingTasks: 1000,  // 允许阻塞的最大限制
}
// 用户使用moles.WithOptions()传入一个moles.Options配置结构体进行设置协程池
cave, err := moles.NewCave(10000, moles.WithOptions(opts))
defer cave.Release()
```

### 单个配置
```go
cave, err := moles.NewCave(10000, moles.WithExpiryDuration(10))      // 设置worker过期时间
cave, err := moles.NewCave(10000, moles.WithNonblocking(true))       // 设定是否为非阻塞模式
cave, err := moles.NewCave(10000, moles.WithMaxBlockingTasks(10000)) // 设置最大阻塞数量(仅在阻塞模式生效)
```

### 可限定单一任务的协程池
```go
package main

import (
	"fmt"
	"github.com/CatBluePoor/moles"
)

func main() {
	// 新建限定任务协程池
	cave, err := moles.NewCaveWithFunc(10000, Test)
	defer cave.Release()
	if err != nil {
		fmt.Println(err)
	}

}
func Test(arg interface{}) {
	fmt.Println(arg)
}
```

## 📚 参考
此项目参考ants制作而成，如需功能更加完整的高性能协程池请移步https://github.com/CatBluePoor/ants
