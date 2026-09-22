package main

import (
	"fmt"
	"time"

	"github.com/adambiggs/gangline/core"
)

const maximumRetryDelay = 30 * time.Second

func (run *runtime) awaitDelivery(state core.State, id core.EnvelopeID) (core.State, error) {
	return awaitDelivery(state, id, time.Now, func(delay time.Duration) {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		<-timer.C
	}, run.recover)
}

// The command requesting startup, or the native asynchronous boundary hook,
// owns this bounded retry. No process remains once its delivery is settled.
func awaitDelivery(state core.State, id core.EnvelopeID, now func() time.Time, wait func(time.Duration), refresh func() (core.State, error)) (core.State, error) {
	delay := startupRetryInterval
	for {
		delivery, ok := state.Deliveries[id]
		if !ok {
			return core.State{}, fmt.Errorf("delivery %q is not recorded", id)
		}
		if delivery.Status != core.DeliveryQueued && delivery.Status != core.DeliveryDelivering {
			return state, nil
		}
		remaining := delivery.Deadline.Sub(now())
		pause := min(delay, max(remaining, 0))
		if pause > 0 {
			wait(pause)
		}
		var err error
		state, err = refresh()
		if err != nil {
			return core.State{}, err
		}
		if remaining <= 0 {
			current := state.Deliveries[id]
			if current.Status == core.DeliveryQueued || current.Status == core.DeliveryDelivering {
				return core.State{}, fmt.Errorf("delivery %q remained pending after its deadline", id)
			}
		}
		delay = min(delay*2, maximumRetryDelay)
	}
}

func (run *runtime) finishBoundaryDeliveries(state core.State, hitchID core.HitchID) error {
	for _, id := range state.DeliveryOrder {
		delivery := state.Deliveries[id]
		hitch := state.Hitches[hitchID]
		if hitch.Status != core.HitchActive || (hitch.Activity != core.ActivityIdle && hitch.Activity != core.ActivityBlocked) {
			return nil
		}
		if delivery.Envelope.To != hitch.Name || delivery.Status != core.DeliveryQueued || !delivery.NotBefore.IsZero() {
			continue
		}
		var err error
		state, err = run.awaitDelivery(state, id)
		if err != nil {
			return err
		}
	}
	return nil
}
