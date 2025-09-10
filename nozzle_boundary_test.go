//nolint:testpackage // Need access to internal fields for boundary condition testing
package nozzle

import (
	"testing"
	"time"
)

// maxExpectedChangeBy is the maximum expected absolute value for changeBy after
// reaching boundaries. Since changeBy doubles on each iteration until boundaries
// are reached, we expect it to stabilize at a reasonable value (typically < 100).
const maxExpectedChangeBy = 100

// TestNozzleBoundaryBehavior verifies that changeBy stops growing at flow rate boundaries.
func TestNozzleBoundaryBehavior(t *testing.T) {
	t.Parallel()

	noz, err := New[any](Options[any]{
		Interval:              10 * time.Millisecond,
		AllowedFailurePercent: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Cleanup(func() {
		if err := noz.Close(); err != nil {
			t.Errorf("failed to close nozzle: %v", err)
		}
	})

	t.Run("closing stops at zero", func(t *testing.T) {
		t.Parallel()

		noz.mut.Lock()
		noz.flowRate = 10
		noz.changeBy = 0

		// Drive to zero
		for noz.flowRate > 0 {
			noz.close()
		}

		// Record changeBy when we hit zero
		changeAtZero := noz.changeBy

		// Stay at zero for many iterations (simulating extended outage)
		for range 100 {
			noz.close()
		}

		// changeBy should not have changed
		if noz.changeBy != changeAtZero {
			t.Errorf("changeBy changed after reaching flowRate=0: was %d, now %d",
				changeAtZero, noz.changeBy)
		}

		// Verify we're at the boundary
		if noz.flowRate != 0 {
			t.Errorf("expected flowRate to be 0, got %d", noz.flowRate)
		}

		// Verify changeBy is reasonable (should be small since it stops at boundary)
		if noz.changeBy < -maxExpectedChangeBy || noz.changeBy > maxExpectedChangeBy {
			t.Errorf("changeBy has unexpected value: %d (expected abs value <= %d)", noz.changeBy, maxExpectedChangeBy)
		}

		noz.mut.Unlock()
	})

	t.Run("opening stops at 100", func(t *testing.T) {
		t.Parallel()

		noz.mut.Lock()
		noz.flowRate = 90
		noz.changeBy = 0

		// Drive to 100
		for noz.flowRate < 100 {
			noz.open()
		}

		// Record changeBy when we hit 100
		changeAt100 := noz.changeBy

		// Stay at 100 for many iterations (simulating continued success)
		for range 100 {
			noz.open()
		}

		// changeBy should not have changed
		if noz.changeBy != changeAt100 {
			t.Errorf("changeBy changed after reaching flowRate=100: was %d, now %d",
				changeAt100, noz.changeBy)
		}

		// Verify we're at the boundary
		if noz.flowRate != 100 {
			t.Errorf("expected flowRate to be 100, got %d", noz.flowRate)
		}

		// Verify changeBy is reasonable
		if noz.changeBy < -maxExpectedChangeBy || noz.changeBy > maxExpectedChangeBy {
			t.Errorf("changeBy has unexpected value: %d (expected abs value <= %d)", noz.changeBy, maxExpectedChangeBy)
		}

		noz.mut.Unlock()
	})
}

// TestNozzleRecoveryFromBoundaries verifies proper recovery behavior.
func TestNozzleRecoveryFromBoundaries(t *testing.T) {
	t.Parallel()

	noz, err := New[any](Options[any]{
		Interval:              10 * time.Millisecond,
		AllowedFailurePercent: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Cleanup(func() {
		if err := noz.Close(); err != nil {
			t.Errorf("failed to close nozzle: %v", err)
		}
	})

	t.Run("recovery from zero", func(t *testing.T) {
		t.Parallel()

		noz.mut.Lock()

		// Drive to zero with failures
		noz.flowRate = 10

		noz.changeBy = 0
		for noz.flowRate > 0 {
			noz.close()
		}

		// Record state at zero
		changeAtZero := noz.changeBy

		// Start recovery
		noz.open()

		// Should start opening gradually
		if noz.flowRate <= 0 {
			t.Errorf("flowRate should have increased from 0, got %d", noz.flowRate)
		}

		// changeBy should have flipped sign and reset to small value
		if noz.changeBy <= 0 {
			t.Errorf("changeBy should be positive during recovery, got %d", noz.changeBy)
		}

		if noz.changeBy > 10 {
			t.Errorf("changeBy should start small during recovery, got %d", noz.changeBy)
		}

		t.Logf("Recovery: flowRate went from 0 to %d, changeBy from %d to %d",
			noz.flowRate, changeAtZero, noz.changeBy)

		noz.mut.Unlock()
	})

	t.Run("failure from 100", func(t *testing.T) {
		t.Parallel()

		noz.mut.Lock()

		// Drive to 100 with successes
		noz.flowRate = 90

		noz.changeBy = 0
		for noz.flowRate < 100 {
			noz.open()
		}

		// Record state at 100
		changeAt100 := noz.changeBy

		// Start closing
		noz.close()

		// Should start closing gradually
		if noz.flowRate >= 100 {
			t.Errorf("flowRate should have decreased from 100, got %d", noz.flowRate)
		}

		// changeBy should have flipped sign and reset to small value
		if noz.changeBy >= 0 {
			t.Errorf("changeBy should be negative during closing, got %d", noz.changeBy)
		}

		if noz.changeBy < -10 {
			t.Errorf("changeBy should start small during closing, got %d", noz.changeBy)
		}

		t.Logf("Closing: flowRate went from 100 to %d, changeBy from %d to %d",
			noz.flowRate, changeAt100, noz.changeBy)

		noz.mut.Unlock()
	})
}

// TestNozzleSymmetricBehavior verifies that open and close have symmetric boundary behavior.
func TestNozzleSymmetricBehavior(t *testing.T) {
	t.Parallel()

	noz, err := New[any](Options[any]{
		Interval:              10 * time.Millisecond,
		AllowedFailurePercent: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Cleanup(func() {
		if err := noz.Close(); err != nil {
			t.Errorf("failed to close nozzle: %v", err)
		}
	})

	// Test that both functions return early at their respective boundaries
	t.Run("boundary early returns", func(t *testing.T) {
		t.Parallel()

		noz.mut.Lock()

		// Test close at zero
		noz.flowRate = 0
		noz.changeBy = -64
		originalChange := noz.changeBy
		noz.close()

		if noz.changeBy != originalChange {
			t.Errorf("close() should not modify changeBy when flowRate=0: was %d, now %d",
				originalChange, noz.changeBy)
		}

		// Test open at 100
		noz.flowRate = 100
		noz.changeBy = 64
		originalChange = noz.changeBy
		noz.open()

		if noz.changeBy != originalChange {
			t.Errorf("open() should not modify changeBy when flowRate=100: was %d, now %d",
				originalChange, noz.changeBy)
		}

		noz.mut.Unlock()
	})
}
