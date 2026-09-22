package main

import (
	"errors"
	"testing"
	"time"
)

func TestNativeWitnessRemainsPendingAcrossHoursOfLiveWork(t *testing.T) {
	witnessed := make(chan nativeWitnessResult, 1)
	checkpoints := make(chan time.Time, 1)
	now := time.Unix(1000, 0)
	checkpoints <- now.Add(6 * time.Hour)
	checks := 0
	got, err := awaitNativeWitness(witnessed, checkpoints, func() error {
		checks++
		if checks < 3 {
			checkpoints <- now.Add(time.Duration(checks+1) * 6 * time.Hour)
		} else {
			witnessed <- nativeWitnessResult{data: []byte("native receipt")}
		}
		return nil
	})
	if err != nil || checks != 3 || string(got.data) != "native receipt" {
		t.Fatalf("live witness expired: %q checks=%d error=%v", got.data, checks, err)
	}
}

func TestNativeWitnessStopsWhenRecipientIsDropped(t *testing.T) {
	checkpoints := make(chan time.Time, 1)
	checkpoints <- time.Unix(1000, 0)
	dropped := refuseError("delivery failed: recipient was dropped")
	_, err := awaitNativeWitness(make(chan nativeWitnessResult), checkpoints, func() error { return dropped })
	if !errors.Is(err, dropped) {
		t.Fatalf("drop was not visible: %v", err)
	}
}
