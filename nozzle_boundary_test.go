package nozzle

import (
	"testing"
	"time"
)

// TestNozzleBoundaryBehavior verifies that changeBy stops growing at flow rate boundaries.
func TestNozzleBoundaryBehavior(t *testing.T) {
	n := New[any](Options[any]{
		Interval:              10 * time.Millisecond,
		AllowedFailurePercent: 10,
	})
	defer n.Close()

	t.Run("closing stops at zero", func(t *testing.T) {
		n.mut.Lock()
		n.flowRate = 10
		n.changeBy = 0
		
		// Drive to zero
		for n.flowRate > 0 {
			n.close()
		}
		
		// Record changeBy when we hit zero
		changeAtZero := n.changeBy
		
		// Stay at zero for many iterations (simulating extended outage)
		for i := 0; i < 100; i++ {
			n.close()
		}
		
		// changeBy should not have changed
		if n.changeBy != changeAtZero {
			t.Errorf("changeBy changed after reaching flowRate=0: was %d, now %d", 
				changeAtZero, n.changeBy)
		}
		
		// Verify we're at the boundary
		if n.flowRate != 0 {
			t.Errorf("expected flowRate to be 0, got %d", n.flowRate)
		}
		
		// Verify changeBy is reasonable (should be small since it stops at boundary)
		if n.changeBy < -100 || n.changeBy > 100 {
			t.Errorf("changeBy has unexpected value: %d (should be small)", n.changeBy)
		}
		
		n.mut.Unlock()
	})

	t.Run("opening stops at 100", func(t *testing.T) {
		n.mut.Lock()
		n.flowRate = 90
		n.changeBy = 0
		
		// Drive to 100
		for n.flowRate < 100 {
			n.open()
		}
		
		// Record changeBy when we hit 100
		changeAt100 := n.changeBy
		
		// Stay at 100 for many iterations (simulating continued success)
		for i := 0; i < 100; i++ {
			n.open()
		}
		
		// changeBy should not have changed
		if n.changeBy != changeAt100 {
			t.Errorf("changeBy changed after reaching flowRate=100: was %d, now %d", 
				changeAt100, n.changeBy)
		}
		
		// Verify we're at the boundary
		if n.flowRate != 100 {
			t.Errorf("expected flowRate to be 100, got %d", n.flowRate)
		}
		
		// Verify changeBy is reasonable
		if n.changeBy < -100 || n.changeBy > 100 {
			t.Errorf("changeBy has unexpected value: %d (should be small)", n.changeBy)
		}
		
		n.mut.Unlock()
	})
}

// TestNozzleRecoveryFromBoundaries verifies proper recovery behavior.
func TestNozzleRecoveryFromBoundaries(t *testing.T) {
	n := New[any](Options[any]{
		Interval:              10 * time.Millisecond,
		AllowedFailurePercent: 10,
	})
	defer n.Close()

	t.Run("recovery from zero", func(t *testing.T) {
		n.mut.Lock()
		
		// Drive to zero with failures
		n.flowRate = 10
		n.changeBy = 0
		for n.flowRate > 0 {
			n.close()
		}
		
		// Record state at zero
		changeAtZero := n.changeBy
		
		// Start recovery
		n.open()
		
		// Should start opening gradually
		if n.flowRate <= 0 {
			t.Errorf("flowRate should have increased from 0, got %d", n.flowRate)
		}
		
		// changeBy should have flipped sign and reset to small value
		if n.changeBy <= 0 {
			t.Errorf("changeBy should be positive during recovery, got %d", n.changeBy)
		}
		
		if n.changeBy > 10 {
			t.Errorf("changeBy should start small during recovery, got %d", n.changeBy)
		}
		
		t.Logf("Recovery: flowRate went from 0 to %d, changeBy from %d to %d", 
			n.flowRate, changeAtZero, n.changeBy)
		
		n.mut.Unlock()
	})

	t.Run("failure from 100", func(t *testing.T) {
		n.mut.Lock()
		
		// Drive to 100 with successes
		n.flowRate = 90
		n.changeBy = 0
		for n.flowRate < 100 {
			n.open()
		}
		
		// Record state at 100
		changeAt100 := n.changeBy
		
		// Start closing
		n.close()
		
		// Should start closing gradually
		if n.flowRate >= 100 {
			t.Errorf("flowRate should have decreased from 100, got %d", n.flowRate)
		}
		
		// changeBy should have flipped sign and reset to small value
		if n.changeBy >= 0 {
			t.Errorf("changeBy should be negative during closing, got %d", n.changeBy)
		}
		
		if n.changeBy < -10 {
			t.Errorf("changeBy should start small during closing, got %d", n.changeBy)
		}
		
		t.Logf("Closing: flowRate went from 100 to %d, changeBy from %d to %d", 
			n.flowRate, changeAt100, n.changeBy)
		
		n.mut.Unlock()
	})
}

// TestNozzleSymmetricBehavior verifies that open and close have symmetric boundary behavior.
func TestNozzleSymmetricBehavior(t *testing.T) {
	n := New[any](Options[any]{
		Interval:              10 * time.Millisecond,
		AllowedFailurePercent: 10,
	})
	defer n.Close()

	// Test that both functions return early at their respective boundaries
	t.Run("boundary early returns", func(t *testing.T) {
		n.mut.Lock()
		
		// Test close at zero
		n.flowRate = 0
		n.changeBy = -64
		originalChange := n.changeBy
		n.close()
		if n.changeBy != originalChange {
			t.Errorf("close() should not modify changeBy when flowRate=0: was %d, now %d",
				originalChange, n.changeBy)
		}
		
		// Test open at 100
		n.flowRate = 100
		n.changeBy = 64
		originalChange = n.changeBy
		n.open()
		if n.changeBy != originalChange {
			t.Errorf("open() should not modify changeBy when flowRate=100: was %d, now %d",
				originalChange, n.changeBy)
		}
		
		n.mut.Unlock()
	})
}