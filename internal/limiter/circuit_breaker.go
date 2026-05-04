package limiter

import (
	"errors"
	"sync"
	"time"
)

// State represents circuit breaker state
type State int

const (
	StateClosed   State = iota // Normal — requests flow through
	StateOpen                  // Tripped — requests fail fast
	StateHalfOpen              // Testing — one request allowed through
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateOpen:
		return "OPEN"
	case StateHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

var ErrCircuitOpen = errors.New("circuit breaker is open")

// CircuitBreaker prevents cascade failures by fast-failing when
// downstream (Redis) is unhealthy.
type CircuitBreaker struct {
	mu sync.Mutex

	// Config
	maxFailures  int           // failures before opening
	resetTimeout time.Duration // how long to stay open before trying again

	// State
	state       State
	failures    int
	lastFailure time.Time
	successes   int // successes in half-open state
}

func NewCircuitBreaker(maxFailures int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		maxFailures:  maxFailures,
		resetTimeout: resetTimeout,
		state:        StateClosed,
	}
}

// Execute runs fn if circuit is closed/half-open.
// Returns ErrCircuitOpen immediately if circuit is open.
func (cb *CircuitBreaker) Execute(fn func() error) error {
	cb.mu.Lock()
	state := cb.currentState()
	cb.mu.Unlock()

	if state == StateOpen {
		return ErrCircuitOpen // fast fail — don't call Redis
	}

	err := fn()

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.onFailure()
	} else {
		cb.onSuccess()
	}

	return err
}

// currentState checks if it's time to transition from Open → HalfOpen.
// Must be called with mu held.
func (cb *CircuitBreaker) currentState() State {
	if cb.state == StateOpen {
		if time.Since(cb.lastFailure) >= cb.resetTimeout {
			cb.state = StateHalfOpen
			cb.successes = 0
		}
	}
	return cb.state
}

// onFailure records a failure and potentially opens the circuit.
// Must be called with mu held.
func (cb *CircuitBreaker) onFailure() {
	cb.failures++
	cb.lastFailure = time.Now()

	if cb.state == StateHalfOpen {
		// Failed during probe — reopen immediately
		cb.state = StateOpen
		return
	}

	if cb.failures >= cb.maxFailures {
		cb.state = StateOpen
	}
}

// onSuccess records a success and potentially closes the circuit.
// Must be called with mu held.
func (cb *CircuitBreaker) onSuccess() {
	if cb.state == StateHalfOpen {
		cb.successes++
		if cb.successes >= 2 { // 2 consecutive successes → close
			cb.state = StateClosed
			cb.failures = 0
		}
		return
	}
	cb.failures = 0 // reset on success in closed state
}

// Stats returns current circuit breaker state (for metrics)
func (cb *CircuitBreaker) Stats() (State, int, time.Time) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state, cb.failures, cb.lastFailure
}