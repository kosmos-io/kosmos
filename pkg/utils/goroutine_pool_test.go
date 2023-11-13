package utils

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestGoroutinePool_Submit(t *testing.T) {
	type fields struct {
		pool      chan int
		funcChan  chan RoutineFunc
		waitGroup *sync.WaitGroup
	}
	type args struct {
		f    interface{}
		args []interface{}
	}
	tests := []struct {
		name   string
		fields fields
		args   []args
	}{
		// func has no parameters
		{"Test1", fields{
			pool:      make(chan int, 5),
			funcChan:  make(chan RoutineFunc, 5),
			waitGroup: &sync.WaitGroup{},
		}, []args{
			{
				f: func() {
					for i := 0; i < 10; i++ {
						fmt.Println(i)
						time.Sleep(1000)
					}
				},
				args: nil,
			}, {
				f: func() {
					for i := 10; i < 20; i++ {
						fmt.Println(i)
						time.Sleep(1000)
					}
				},
				args: nil,
			},
		}},
		// func has parameters
		{"Test2", fields{
			pool:      make(chan int, 5),
			funcChan:  make(chan RoutineFunc, 5),
			waitGroup: &sync.WaitGroup{},
		}, []args{
			{
				f: func(args ...interface{}) {
					for _, arg := range args {
						fmt.Println(arg)
						time.Sleep(1000)
					}
				},
				args: []interface{}{"a", "b", "c", "d"},
			}, {
				f: func(args ...interface{}) {
					for _, arg := range args {
						fmt.Println(arg)
						time.Sleep(1000)
					}
				},
				args: []interface{}{"e", "f", "g", "h"},
			},
		}},
		// the thread capacity is 1
		{"Test3", fields{
			pool:      make(chan int, 1),
			funcChan:  make(chan RoutineFunc, 1),
			waitGroup: &sync.WaitGroup{},
		}, []args{
			{
				f: func() {
					for i := 0; i < 10; i++ {
						fmt.Println(i)
						time.Sleep(1000)
					}
				},
				args: nil,
			}, {
				f: func() {
					for i := 10; i < 20; i++ {
						fmt.Println(i)
						time.Sleep(1000)
					}
				},
				args: nil,
			},
		}},
		//incorrect func parameter
		{"Test4", fields{
			pool:      make(chan int, 5),
			funcChan:  make(chan RoutineFunc, 5),
			waitGroup: &sync.WaitGroup{},
		}, []args{
			{
				f: func(a int) {
					fmt.Println(a)
				},
				args: []interface{}{"hello"},
			},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &GoroutinePool{
				pool:      tt.fields.pool,
				funcChan:  tt.fields.funcChan,
				waitGroup: tt.fields.waitGroup,
			}
			for _, arg := range tt.args {
				g.Submit(arg.f, arg.args...)
			}
			g.Wait()
			g.Shutdown()
			fmt.Println("success!")
		})
	}
}
