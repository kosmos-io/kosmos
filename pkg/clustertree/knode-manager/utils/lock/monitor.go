package lock

import (
	"sync"
)

func NewMonitorVariable() MonitorVariable {
	mv := &monitorVariable{
		versionInvalidationChannel: make(chan struct{}),
	}
	return mv
}

type MonitorVariable interface {
	Set(value interface{})
	Subscribe() Subscription
}

type Subscription interface {
	NewValueReady() <-chan struct{}
	Value() Value
}

type Value struct {
	Value   interface{}
	Version int64
}

type monitorVariable struct {
	lock                       sync.Mutex
	currentValue               interface{}
	currentVersion             int64
	versionInvalidationChannel chan struct{}
}

func (m *monitorVariable) Set(newValue interface{}) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.currentValue = newValue
	m.currentVersion++
	close(m.versionInvalidationChannel)
	m.versionInvalidationChannel = make(chan struct{})
}

func (m *monitorVariable) Subscribe() Subscription {
	m.lock.Lock()
	defer m.lock.Unlock()
	sub := &subscription{
		mv: m,
	}
	if m.currentVersion > 0 {
		closedCh := make(chan struct{})
		close(closedCh)
		sub.lastVersionReadInvalidationChannel = closedCh
	} else {
		sub.lastVersionReadInvalidationChannel = m.versionInvalidationChannel
	}

	return sub
}

type subscription struct {
	mv                                 *monitorVariable
	lastVersionRead                    int64
	lastVersionReadInvalidationChannel chan struct{}
}

func (s *subscription) NewValueReady() <-chan struct{} {
	s.mv.lock.Lock()
	defer s.mv.lock.Unlock()
	return s.lastVersionReadInvalidationChannel
}

func (s *subscription) Value() Value {
	s.mv.lock.Lock()
	defer s.mv.lock.Unlock()
	val := Value{
		Value:   s.mv.currentValue,
		Version: s.mv.currentVersion,
	}
	s.lastVersionRead = s.mv.currentVersion
	s.lastVersionReadInvalidationChannel = s.mv.versionInvalidationChannel
	return val
}
