package collector

import "context"

// Kind identifies the source-specific shape of a RawSignal payload.
type Kind string

// Collector signal kinds.
const (
	KindContainerList Kind = "container_list"
	KindInspect       Kind = "inspect"
	KindLogLine       Kind = "log_line"
	KindStats         Kind = "stats"
)

// RawSignal is a source-specific observation before normalization.
// Payload types are owned by the emitting collector (see poll.go, etc.).
type RawSignal struct {
	Kind      Kind
	Timestamp int64 // unix nano
	Payload   any
}

// Collector produces raw signals from one observation source.
type Collector interface {
	Name() string
	Start(ctx context.Context, out chan<- RawSignal) error
}
