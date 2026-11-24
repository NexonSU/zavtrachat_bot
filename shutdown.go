package main

import (
	"errors"
	"time"
)

const botShutdownTimeout = 30 * time.Second

var errShutdownTimeout = errors.New("shutdown timed out")

func stopWithin(timeout time.Duration, stops ...func() error) error {
	done := make(chan error, 1)
	go func() {
		errs := make([]error, 0, len(stops))
		for _, stop := range stops {
			if err := stop(); err != nil {
				errs = append(errs, err)
			}
		}
		done <- errors.Join(errs...)
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case err := <-done:
		return err
	case <-timer.C:
		return errShutdownTimeout
	}
}
