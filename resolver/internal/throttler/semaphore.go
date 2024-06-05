package throttler

import (
	"context"
	"sync/atomic"
)

type semaphore struct {
	state atomic.Uint64
	queue chan struct{}
}

// newSemaphore creates a semaphore with the desired initial capacity.
func newSemaphore(maxCapacity, initialCapacity int) *semaphore {
	queue := make(chan struct{}, maxCapacity)
	sem := &semaphore{queue: queue}
	sem.updateCapacity(initialCapacity)
	return sem
}

// acquire acquires capacity from the semaphore.
func (s *semaphore) acquire(ctx context.Context) error {
	for {
		old := s.state.Load()
		capacity, in := unpack(old)

		if in >= capacity {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-s.queue:
			}
			// Force reload state.
			continue
		}

		in++
		if s.state.CompareAndSwap(old, pack(capacity, in)) {
			return nil
		}
	}
}

// release releases capacity in the semaphore.
// If the semaphore capacity was reduced in between and as a result inFlight is greater
// than capacity, we don't wake up goroutines as they'd not get any capacity anyway.
func (s *semaphore) release() {
	for {
		old := s.state.Load()
		capacity, in := unpack(old)

		if in == 0 {
			panic("release and acquire are not paired")
		}

		in--
		if s.state.CompareAndSwap(old, pack(capacity, in)) {
			if in < capacity {
				select {
				case s.queue <- struct{}{}:
				default:
					// We generate more wakeups than we might need as we don't know
					// how many goroutines are waiting here. It is therefore okay
					// to drop the poke on the floor here as this case would mean we
					// have enough wakeups to wake up as many goroutines as this semaphore
					// can take, which is guaranteed to be enough.
				}
			}
			return
		}
	}
}

// updateCapacity updates the capacity of the semaphore to the desired size.
func (s *semaphore) updateCapacity(size int) {
	s64 := uint64(size)
	for {
		old := s.state.Load()
		capacity, in := unpack(old)

		if capacity == s64 {
			// Nothing to do, exit early.
			return
		}

		if s.state.CompareAndSwap(old, pack(s64, in)) {
			if s64 > capacity {
				for i := uint64(0); i < s64-capacity; i++ {
					select {
					case s.queue <- struct{}{}:
					default:
						// See comment in `release` for explanation of this case.
					}
				}
			}
			return
		}
	}
}

// Capacity is the capacity of the semaphore.
func (s *semaphore) Capacity() int {
	capacity, _ := unpack(s.state.Load())
	return int(capacity)
}

// unpack takes an uint64 and returns two uint32 (as uint64) comprised of the leftmost
// and the rightmost bits respectively.
func unpack(in uint64) (uint64, uint64) {
	return in >> 32, in & 0xffffffff
}

// pack takes two uint32 (as uint64 to avoid casting) and packs them into a single uint64
// at the leftmost and the rightmost bits respectively.
// It's up to the caller to ensure that left and right actually fit into 32 bit.
func pack(left, right uint64) uint64 {
	return left<<32 | right
}
