package main

import (
	"fmt"
	"time"

	"github.com/adambiggs/gangline/core"
)

const maximumRetryDelay = 30 * time.Second

func (run *runtime) awaitDelivery(state core.State, id core.EnvelopeID) (core.State, error) {
	return awaitDelivery(state, id, func(delay time.Duration) {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		<-timer.C
	}, run.refreshDeliveries)
}

// The command requesting startup, or the native asynchronous boundary hook,
// owns retries until acceptance or drop. Elapsed time never expires the send.
func awaitDelivery(state core.State, id core.EnvelopeID, wait func(time.Duration), refresh func() (core.State, error)) (core.State, error) {
	delay := startupRetryInterval
	for {
		delivery, ok := state.Deliveries[id]
		if !ok {
			return core.State{}, fmt.Errorf("delivery %q is not recorded", id)
		}
		if delivery.Status != core.DeliveryQueued && delivery.Status != core.DeliveryDelivering {
			return state, nil
		}
		wait(delay)
		var err error
		state, err = refresh()
		if err != nil {
			return core.State{}, err
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
