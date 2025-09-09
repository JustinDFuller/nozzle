package nozzle

import (
	"testing"
	"time"
)

// TestNozzleBoundaryBehavior verifies that decreaseBy stops growing at flow rate boundaries.
func TestNozzleBoundaryBehavior(t *testing.T) {
	n := New[any](Options[any]{
		Interval:              10 * time.Millisecond,
		AllowedFailurePercent: 10,
	})
	defer n.Close()

	t.Run("closing stops at zero", func(t *testing.T) {
		n.mut.Lock()
		n.flowRate = 10
		n.decreaseBy = 0
		
		// Drive to zero
		for n.flowRate > 0 {
			n.close()
		}
		
		// Record decreaseBy when we hit zero
		decreaseAtZero := n.decreaseBy
		
		// Stay at zero for many iterations (simulating extended outage)
		for i := 0; i < 100; i++ {
			n.close()
		}
		
		// decreaseBy should not have changed
		if n.decreaseBy != decreaseAtZero {
			t.Errorf("decreaseBy changed after reaching flowRate=0: was %d, now %d", 
				decreaseAtZero, n.decreaseBy)
		}
		
		// Verify we're at the boundary
		if n.flowRate != 0 {
			t.Errorf("expected flowRate to be 0, got %d", n.flowRate)
		}
		
		// Verify decreaseBy is reasonable (should be small since it stops at boundary)
		if n.decreaseBy < -100 || n.decreaseBy > 100 {
			t.Errorf("decreaseBy has unexpected value: %d (should be small)", n.decreaseBy)
		}
		
		n.mut.Unlock()
	})

	t.Run("opening stops at 100", func(t *testing.T) {
		n.mut.Lock()
		n.flowRate = 90
		n.decreaseBy = 0
		
		// Drive to 100
		for n.flowRate < 100 {
			n.open()
		}
		
		// Record decreaseBy when we hit 100
		decreaseAt100 := n.decreaseBy
		
		// Stay at 100 for many iterations (simulating continued success)
		for i := 0; i < 100; i++ {
			n.open()
		}
		
		// decreaseBy should not have changed
		if n.decreaseBy != decreaseAt100 {
			t.Errorf("decreaseBy changed after reaching flowRate=100: was %d, now %d", 
				decreaseAt100, n.decreaseBy)
		}
		
		// Verify we're at the boundary
		if n.flowRate != 100 {
			t.Errorf("expected flowRate to be 100, got %d", n.flowRate)
		}
		
		// Verify decreaseBy is reasonable
		if n.decreaseBy < -100 || n.decreaseBy > 100 {
			t.Errorf("decreaseBy has unexpected value: %d (should be small)", n.decreaseBy)
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
		n.decreaseBy = 0
		for n.flowRate > 0 {
			n.close()
		}
		
		// Record state at zero
		decreaseAtZero := n.decreaseBy
		
		// Start recovery
		n.open()
		
		// Should start opening gradually
		if n.flowRate <= 0 {
			t.Errorf("flowRate should have increased from 0, got %d", n.flowRate)
		}
		
		// decreaseBy should have flipped sign and reset to small value
		if n.decreaseBy <= 0 {
			t.Errorf("decreaseBy should be positive during recovery, got %d", n.decreaseBy)
		}
		
		if n.decreaseBy > 10 {
			t.Errorf("decreaseBy should start small during recovery, got %d", n.decreaseBy)
		}
		
		t.Logf("Recovery: flowRate went from 0 to %d, decreaseBy from %d to %d", 
			n.flowRate, decreaseAtZero, n.decreaseBy)
		
		n.mut.Unlock()
	})

	t.Run("failure from 100", func(t *testing.T) {
		n.mut.Lock()
		
		// Drive to 100 with successes
		n.flowRate = 90
		n.decreaseBy = 0
		for n.flowRate < 100 {
			n.open()
		}
		
		// Record state at 100
		decreaseAt100 := n.decreaseBy
		
		// Start closing
		n.close()
		
		// Should start closing gradually
		if n.flowRate >= 100 {
			t.Errorf("flowRate should have decreased from 100, got %d", n.flowRate)
		}
		
		// decreaseBy should have flipped sign and reset to small value
		if n.decreaseBy >= 0 {
			t.Errorf("decreaseBy should be negative during closing, got %d", n.decreaseBy)
		}
		
		if n.decreaseBy < -10 {
			t.Errorf("decreaseBy should start small during closing, got %d", n.decreaseBy)
		}
		
		t.Logf("Closing: flowRate went from 100 to %d, decreaseBy from %d to %d", 
			n.flowRate, decreaseAt100, n.decreaseBy)
		
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
		n.decreaseBy = -64
		originalDecrease := n.decreaseBy
		n.close()
		if n.decreaseBy != originalDecrease {
			t.Errorf("close() should not modify decreaseBy when flowRate=0: was %d, now %d",
				originalDecrease, n.decreaseBy)
		}
		
		// Test open at 100
		n.flowRate = 100
		n.decreaseBy = 64
		originalDecrease = n.decreaseBy
		n.open()
		if n.decreaseBy != originalDecrease {
			t.Errorf("open() should not modify decreaseBy when flowRate=100: was %d, now %d",
				originalDecrease, n.decreaseBy)
		}
		
		n.mut.Unlock()
	})
}