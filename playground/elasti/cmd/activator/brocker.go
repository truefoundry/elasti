package main

import "sync/atomic"

type Breaker struct {
	inFlight   atomic.Int64
	totalSlots int64
}

func NewBreaker(totalSlots int64) *Breaker {
	return &Breaker{
		totalSlots: totalSlots,
	}
}
