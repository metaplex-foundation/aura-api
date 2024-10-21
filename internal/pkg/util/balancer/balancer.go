package balancer

import (
	"sync"
)

type RoundRobin[T comparable] struct {
	mx      *sync.Mutex
	targets []T
	counter int
}

func NewRoundRobin[T comparable](targets []T) *RoundRobin[T] {
	return &RoundRobin[T]{
		mx:      &sync.Mutex{},
		targets: targets,
	}
}

func (r *RoundRobin[T]) GetNext() (t T) {
	if len(r.targets) == 0 {
		return t
	}

	r.mx.Lock()

	r.counter %= len(r.targets)
	t = r.targets[r.counter]
	r.counter++

	r.mx.Unlock()

	return
}

func (r *RoundRobin[T]) IsAvailable() bool {
	return len(r.targets) != 0
}
func (r *RoundRobin[T]) GetTargetsCount() int {
	return len(r.targets)
}
