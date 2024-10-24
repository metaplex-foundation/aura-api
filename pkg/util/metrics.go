package util

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

type runtimeMetricsEntry struct {
	title    string
	duration time.Duration
}

type RuntimeMetrics struct {
	title     string
	namespace string
	start     time.Time
	metrics   []runtimeMetricsEntry
	mx        sync.Mutex
}

func NewRuntimeMetrics() *RuntimeMetrics {
	return &RuntimeMetrics{
		start:   time.Now(),
		metrics: make([]runtimeMetricsEntry, 0),
	}
}

type runtimeCheckpoint struct {
	time  time.Time
	title string
}

func NewRuntimeCheckpoint(title string) runtimeCheckpoint {
	return runtimeCheckpoint{
		title: title,
		time:  time.Now(),
	}
}

func (rm *RuntimeMetrics) SetTitle(title string) {
	rm.title = title
}

func (rm *RuntimeMetrics) SetNamespace(namespace string) {
	rm.namespace = namespace
}

func (rm *RuntimeMetrics) AddCheckpoint(checkpoint runtimeCheckpoint) {
	rm.mx.Lock()
	rm.metrics = append(rm.metrics, runtimeMetricsEntry{
		title:    checkpoint.title,
		duration: time.Since(checkpoint.time),
	})
	rm.mx.Unlock()
}

func (rm *RuntimeMetrics) String() string {
	if rm == nil {
		return ""
	}

	rm.mx.Lock()
	defer rm.mx.Unlock()

	metrics := make([]string, 0, len(rm.metrics))
	for _, m := range rm.metrics {
		metrics = append(metrics, fmt.Sprintf("%s: %dms", m.title, m.duration.Milliseconds()))
	}

	return fmt.Sprintf("[%s][%s] %dms | %s", rm.namespace, rm.title, time.Since(rm.start).Milliseconds(), strings.Join(metrics, ", "))
}
