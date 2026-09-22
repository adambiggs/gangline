package core

func Step(state State, event Event) (State, []Effect) {
	next := cloneState(state)

	switch event := event.(type) {
	case CapacityDetected:
		return stepCapacityDetected(next, event)
	case CapacityRetryRequested:
		return stepCapacityRetryRequested(next, event)
	case CapacityExpired:
		return stepCapacityExpired(next, event)
	case CapacityCleared:
		return stepCapacityCleared(next, event)
	case HitchRequested:
		return stepHitchRequested(next, event)
	case AdoptRequested:
		return stepAdoptRequested(next, event)
	case RenameRequested:
		return stepRenameRequested(next, event)
	case HitchSpawned:
		return stepHitchSpawned(next, event)
	case HitchReady:
		return stepHitchReady(next, event)
	case HitchLaunchFailed:
		return stepHitchLaunchFailed(next, event)
	case TurnStarted:
		return stepTurnStarted(next, event)
	case TurnBoundaryReached:
		return stepTurnBoundary(next, event)
	case BlockedDetected:
		return stepBlockedDetected(next, event)
	case BlockedCleared:
		return stepBlockedCleared(next, event)
	case SendRequested:
		return stepSendRequested(next, event)
	case TimedDeliveryReleased:
		return stepTimedDeliveryReleased(next, event)
	case TimedDeliveriesCleared:
		return stepTimedDeliveriesCleared(next, event)
	case DeliveryInputStarted:
		return stepDeliveryInputStarted(next, event)
	case DeliverySucceeded:
		return stepDeliverySucceeded(next, event)
	case DeliveryRetryRequested:
		return stepDeliveryRetryRequested(next, event)
	case DeliveryDeferred:
		return stepDeliveryDeferred(next, event)
	case DeliveryFailedEvent:
		return stepDeliveryFailed(next, event)
	case DeliveryUnverifiedEvent:
		return stepDeliveryUnverified(next, event)
	case CompactionRequested:
		return stepCompactionRequested(next, event)
	case CompactionCompleted:
		return stepCompactionCompleted(next, event)
	case CompactionSubmitted:
		return stepCompactionSubmitted(next, event)
	case CompactionUnverifiedEvent:
		return stepCompactionUnverified(next, event)
	case CompactionFailedEvent:
		return stepCompactionFailed(next, event)
	case InterruptRequested:
		return stepInterruptRequested(next, event)
	case InterruptSucceeded:
		return stepInterruptSucceeded(next, event)
	case InterruptFailed:
		return stepInterruptFailed(next, event)
	case DropRequested:
		return stepDropRequested(next, event)
	case DropSucceeded:
		return stepDropSucceeded(next, event)
	case DropFailed:
		return stepDropFailed(next, event)
	case PaneVanished:
		return stepPaneVanished(next, event)
	case WedgeDetected:
		return stepWedgeDetected(next, event)
	case WedgeCleared:
		return stepWedgeCleared(next, event)
	case OperationTimedOut:
		return stepTimedOut(next, event)
	case CurfewSet:
		return stepCurfewSet(next, event)
	case CurfewCleared:
		return stepCurfewCleared(next, event)
	case Observation:
		return next, nil
	case NativeHook:
		return next, nil
	case TransitionRejected:
		return next, nil
	}

	return next, nil
}
