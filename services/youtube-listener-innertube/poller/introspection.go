// Package poller — read-only introspection of a Poller's collaborators.
//
// This is ordinary (non-_test) code because it is used from another package's
// tests, which the _test.go suffix would not permit. It adds no behaviour: four
// nil checks, no setters.
//
// These exist so the canary package can assert the invariant its whole design
// rests on: the poller it builds has no Publisher, no Repository and no
// Metrics, and therefore cannot reach chat:raw or move a production counter.
// That invariant otherwise lives only in a struct literal and a comment, so a
// future edit adding one of those collaborators would silently start publishing
// a 24/7 channel's chat into the pipeline with no test failing.
//
// Kept to read-only accessors on unexported fields; they add no behaviour.
package poller

// HasPublisher reports whether a lifecycle publisher is configured.
func (p *Poller) HasPublisher() bool { return p.publisher != nil }

// HasRepository reports whether a Redis repository is configured.
func (p *Poller) HasRepository() bool { return p.repository != nil }

// HasMetrics reports whether production metrics are configured.
func (p *Poller) HasMetrics() bool { return p.metrics != nil }

// AlwaysSleeps reports whether the loop waits the full interval even after a
// poll that returned messages.
func (p *Poller) AlwaysSleeps() bool { return p.alwaysSleep }
