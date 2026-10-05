package app

import (
	"fmt"
	"runtime/debug"
)

// job is work running on a goroutine of its own, whose result the interface
// picks up once it is there. Reading the game files and building a model take
// long enough to stall the window, so they run as jobs.
type job[T any] struct {
	done chan jobResult[T]
}

type jobResult[T any] struct {
	value T
	err   error
}

// start runs work on a goroutine of its own.
//
// A panic in it comes back as an error rather than taking the whole window
// down: the files being read are whatever the user opened, and a file that
// trips up the readers should cost that file, not the application.
func start[T any](work func() (T, error)) *job[T] {
	started := &job[T]{done: make(chan jobResult[T], 1)}

	go func() {
		var result jobResult[T]

		defer func() {
			if recovered := recover(); recovered != nil {
				warn(fmt.Errorf("%v\n%s", recovered, debug.Stack()))
				result = jobResult[T]{err: fmt.Errorf("reading the files failed unexpectedly: %v", recovered)}
			}

			started.done <- result
		}()

		result.value, result.err = work()
	}()

	return started
}

// poll returns the job's result once it is there.
func (j *job[T]) poll() (value T, err error, done bool) {
	select {
	case result := <-j.done:
		return result.value, result.err, true
	default:
		return value, nil, false
	}
}
