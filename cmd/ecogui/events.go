package main

import (
	"time"
)

// Custom gi events

const (
	eventSetFrame = iota
	eventUpdateInitProgress
	eventDCRDSyncStatus
	eventServiceStatus
	eventAppsUpdated
)

// customEvent implements oswin.Event
type customEvent struct {
	eType     int
	stamp     time.Time
	processed bool
	data      interface{}
}

func newCustomEvent(eType int, data interface{}) *customEvent {
	return &customEvent{
		eType: eType,
		stamp: time.Now(),
		data:  data,
	}
}
