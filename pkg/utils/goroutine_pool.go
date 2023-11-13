package utils

import (
	"fmt"
	"sync"
)

type GoroutinePool struct {
	pool      chan int
	funcChan  chan RoutineFunc
	waitGroup *sync.WaitGroup
}

type RoutineFunc struct {
	f    interface{}
	args []interface{}
}

func NewGoroutinePool(size int) *GoroutinePool {
	return &GoroutinePool{
		pool:      make(chan int, size),
		funcChan:  make(chan RoutineFunc, size),
		waitGroup: &sync.WaitGroup{},
	}
}

func (g *GoroutinePool) Submit(f interface{}, args ...interface{}) {
	g.funcChan <- RoutineFunc{f: f, args: args}
	g.pool <- 1
	g.waitGroup.Add(1)

	go func() {
		task := <-g.funcChan
		switch f := task.f.(type) {
		case func():
			f()
		case func(args ...interface{}):
			f(task.args...)
		default:
			fmt.Println("Invalid task type")
		}
		defer g.Done()
	}()
}

func (g *GoroutinePool) Wait() {
	g.waitGroup.Wait()
}

func (g *GoroutinePool) Done() {
	<-g.pool
	g.waitGroup.Done()
}

func (g *GoroutinePool) Shutdown() {
	close(g.pool)
}
