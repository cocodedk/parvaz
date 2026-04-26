package relay

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/cocodedk/parvaz/core/protocol"
)

// CoalescerConfig configures batch coalescing behavior. Both fields
// must be set; zero values get safe minimums (Window=0 means flush on
// the next channel tick, MaxBatch<1 degenerates to single-mode).
type CoalescerConfig struct {
	// Window is the maximum time the first submission in a batch will
	// wait for siblings before being flushed alone.
	Window time.Duration
	// MaxBatch caps how many submissions ride one envelope. Reaching
	// this number triggers a flush before Window elapses.
	MaxBatch int
}

// Coalescer queues protocol.Requests so multiple in-flight callers
// share one Apps Script invocation via Relay.DoBatch. Submit blocks
// until the caller's slot completes (or its ctx fires); the underlying
// flush is fire-and-forget so a slow Apps Script call does not block
// new submissions from being queued for the next batch.
type Coalescer struct {
	relay    *Relay
	cfg      CoalescerConfig
	submit   chan *coalescerSubmission
	done     chan struct{}
	closeOnce sync.Once
	flushers sync.WaitGroup
}

type coalescerSubmission struct {
	req    protocol.Request
	result chan coalescerResult
}

type coalescerResult struct {
	resp *protocol.Response
	err  error
}

// NewCoalescer starts the coalescer goroutine. Callers must Close()
// when done so in-flight batches complete cleanly.
func NewCoalescer(r *Relay, cfg CoalescerConfig) *Coalescer {
	if cfg.MaxBatch < 1 {
		cfg.MaxBatch = 1
	}
	if cfg.Window < 0 {
		cfg.Window = 0
	}
	c := &Coalescer{
		relay:  r,
		cfg:    cfg,
		submit: make(chan *coalescerSubmission),
		done:   make(chan struct{}),
	}
	go c.run()
	return c
}

// Do hands req to the coalescer and blocks until either the batch
// completes or ctx fires. If ctx fires before the request reaches the
// coalescer goroutine, no batch resources are consumed. The signature
// matches Relay.Do so *Coalescer satisfies the same Relayer interfaces
// (mitm.Relayer, parvazd.Relayer) — drop-in replacement.
func (c *Coalescer) Do(ctx context.Context, req protocol.Request) (*protocol.Response, error) {
	sub := &coalescerSubmission{req: req, result: make(chan coalescerResult, 1)}
	select {
	case c.submit <- sub:
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.done:
		return nil, errors.New("relay: coalescer closed")
	}
	select {
	case res := <-sub.result:
		return res.resp, res.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Close stops accepting new submissions, drains any pending batch,
// and waits for in-flight flushes to complete.
func (c *Coalescer) Close() {
	c.closeOnce.Do(func() { close(c.done) })
	c.flushers.Wait()
}

func (c *Coalescer) run() {
	for {
		var first *coalescerSubmission
		select {
		case first = <-c.submit:
		case <-c.done:
			return
		}
		pending := []*coalescerSubmission{first}

		if c.cfg.MaxBatch <= 1 {
			c.flush(pending)
			continue
		}

		timer := time.NewTimer(c.cfg.Window)
	collect:
		for len(pending) < c.cfg.MaxBatch {
			select {
			case sub := <-c.submit:
				pending = append(pending, sub)
			case <-timer.C:
				break collect
			case <-c.done:
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				c.flush(pending)
				return
			}
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		c.flush(pending)
	}
}

// flush spawns dispatch in a goroutine so the run loop can immediately
// start collecting the next batch — otherwise a slow Apps Script call
// stalls all subsequent submissions.
func (c *Coalescer) flush(pending []*coalescerSubmission) {
	c.flushers.Add(1)
	go func() {
		defer c.flushers.Done()
		c.dispatch(pending)
	}()
}

func (c *Coalescer) dispatch(pending []*coalescerSubmission) {
	items := make([]protocol.Request, len(pending))
	for i, s := range pending {
		items[i] = s.req
	}
	bresp, err := c.relay.DoBatch(context.Background(), protocol.BatchRequest{Items: items})
	if err != nil {
		for _, s := range pending {
			s.result <- coalescerResult{err: err}
		}
		return
	}
	for i, s := range pending {
		item := bresp.Items[i]
		s.result <- coalescerResult{resp: item.Response, err: item.Err}
	}
}
