package nominatim

import (
	"context"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	outboundRateLimitQueueDepth = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "datascience",
		Subsystem: "nominatim",
		Name:      "outbound_rate_limit_queue_depth",
		Help:      "Current number of outbound Nominatim HTTP requests waiting for rate-limit capacity.",
	})
	outboundRateLimitWaitDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: "datascience",
		Subsystem: "nominatim",
		Name:      "outbound_rate_limit_wait_seconds",
		Help:      "Time spent waiting for outbound Nominatim rate-limit capacity.",
		Buckets:   []float64{0.001, 0.01, 0.05, 0.1, 0.5, 1, 5, 30, 60, 180},
	})
	outboundRateLimitGrantedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "datascience",
		Subsystem: "nominatim",
		Name:      "outbound_rate_limit_granted_total",
		Help:      "Total number of outbound Nominatim HTTP requests granted by the rate limiter.",
	})
)

func MetricsCollectors() []prometheus.Collector {
	return []prometheus.Collector{
		outboundRateLimitQueueDepth,
		outboundRateLimitWaitDuration,
		outboundRateLimitGrantedTotal,
	}
}

type requestLimiter interface {
	Wait(ctx context.Context) error
}

type outboundRateLimiter struct {
	capacity int
	window   time.Duration

	mu      sync.Mutex
	waiters []*limiterWaiter
	grants  []time.Time
	wake    chan struct{}
}

type limiterWaiter struct {
	ctx        context.Context
	ready      chan struct{}
	enqueuedAt time.Time
	granted    bool
	canceled   bool
}

func newOutboundRateLimiter(capacity int, window time.Duration) requestLimiter {
	if capacity <= 0 || window <= 0 {
		outboundRateLimitQueueDepth.Set(0)
		return nil
	}

	limiter := &outboundRateLimiter{
		capacity: capacity,
		window:   window,
		wake:     make(chan struct{}, 1),
	}

	outboundRateLimitQueueDepth.Set(0)

	go limiter.run()

	return limiter
}

func (l *outboundRateLimiter) Wait(ctx context.Context) error {
	waiter := &limiterWaiter{
		ctx:        ctx,
		ready:      make(chan struct{}),
		enqueuedAt: time.Now(),
	}

	l.mu.Lock()
	l.waiters = append(l.waiters, waiter)
	outboundRateLimitQueueDepth.Set(float64(len(l.waiters)))
	l.mu.Unlock()

	l.signal()

	select {
	case <-waiter.ready:
		return nil
	case <-ctx.Done():
		if l.cancel(waiter) {
			return nil
		}
		return ctx.Err()
	}
}

func (l *outboundRateLimiter) cancel(waiter *limiterWaiter) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if waiter.granted {
		return true
	}

	if waiter.canceled {
		return false
	}

	waiter.canceled = true
	l.removeCanceledLocked()
	outboundRateLimitQueueDepth.Set(float64(len(l.waiters)))

	select {
	case l.wake <- struct{}{}:
	default:
	}

	return false
}

func (l *outboundRateLimiter) run() {
	var timer *time.Timer

	for {
		delay := l.process()
		if delay < 0 {
			<-l.wake
			continue
		}

		if timer == nil {
			timer = time.NewTimer(delay)
		} else {
			timer.Reset(delay)
		}

		select {
		case <-timer.C:
		case <-l.wake:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
		}
	}
}

func (l *outboundRateLimiter) process() time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	l.pruneGrantedLocked(now)
	l.removeCanceledLocked()

	for len(l.waiters) > 0 && len(l.grants) < l.capacity {
		waiter := l.waiters[0]
		l.waiters = l.waiters[1:]

		if waiter.canceled || waiter.ctx.Err() != nil {
			continue
		}

		waiter.granted = true
		l.grants = append(l.grants, now)
		outboundRateLimitGrantedTotal.Inc()
		outboundRateLimitWaitDuration.Observe(now.Sub(waiter.enqueuedAt).Seconds())
		close(waiter.ready)
	}

	outboundRateLimitQueueDepth.Set(float64(len(l.waiters)))

	if len(l.waiters) == 0 {
		return -1
	}

	if len(l.grants) < l.capacity {
		return 0
	}

	waitDuration := l.grants[0].Add(l.window).Sub(now)
	if waitDuration < 0 {
		return 0
	}

	return waitDuration
}

func (l *outboundRateLimiter) pruneGrantedLocked(now time.Time) {
	index := 0
	for index < len(l.grants) && !l.grants[index].Add(l.window).After(now) {
		index++
	}

	if index == 0 {
		return
	}

	l.grants = append([]time.Time(nil), l.grants[index:]...)
}

func (l *outboundRateLimiter) removeCanceledLocked() {
	filtered := l.waiters[:0]
	for _, waiter := range l.waiters {
		if waiter.canceled || waiter.ctx.Err() != nil {
			continue
		}

		filtered = append(filtered, waiter)
	}
	l.waiters = filtered
}

func (l *outboundRateLimiter) signal() {
	select {
	case l.wake <- struct{}{}:
	default:
	}
}
