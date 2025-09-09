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
		// Set initial state
		n.mut.Lock()
		n.flowRate = 10
		n.decreaseBy = 0
		
		// Simulate multiple close operations
		var lastDecrease int64
		for i := 0; i < 20; i++ {
			n.close()
			
			// Once we hit zero, decreaseBy should stop changing
			if n.flowRate == 0 {
				if lastDecrease != 0 && n.decreaseBy != lastDecrease {
					t.Errorf("decreaseBy changed after reaching flowRate=0: was %d, now %d", 
						lastDecrease, n.decreaseBy)
				}
				lastDecrease = n.decreaseBy
			}
		}
		
		// Verify we're at the boundary
		if n.flowRate != 0 {
			t.Errorf("expected flowRate to be 0, got %d", n.flowRate)
		}
		n.mut.Unlock()
	})

	t.Run("opening stops at 100", func(t *testing.T) {
		// Reset state
		n.mut.Lock()
		n.flowRate = 90
		n.decreaseBy = 0
		
		// Simulate multiple open operations
		var lastDecrease int64
		for i := 0; i < 20; i++ {
			n.open()
			
			// Once we hit 100, decreaseBy should stop changing
			if n.flowRate == 100 {
				if lastDecrease != 0 && n.decreaseBy != lastDecrease {
					t.Errorf("decreaseBy changed after reaching flowRate=100: was %d, now %d", 
						lastDecrease, n.decreaseBy)
				}
				lastDecrease = n.decreaseBy
			}
		}
		
		// Verify we're at the boundary
		if n.flowRate != 100 {
			t.Errorf("expected flowRate to be 100, got %d", n.flowRate)
		}
		n.mut.Unlock()
	})
}

// TestNozzleNoOverflowAtBoundaries verifies that prolonged boundary conditions don't cause overflow.
func TestNozzleNoOverflowAtBoundaries(t *testing.T) {
	n := New[any](Options[any]{
		Interval:              5 * time.Millisecond,
		AllowedFailurePercent: 10,
	})
	defer n.Close()

	t.Run("extended failure at zero", func(t *testing.T) {
		n.mut.Lock()
		n.flowRate = 5
		n.decreaseBy = 0
		
		// Drive to zero
		for n.flowRate > 0 {
			n.close()
		}
		
		initialDecrease := n.decreaseBy
		
		// Stay at zero for many iterations
		for i := 0; i < 100; i++ {
			n.close()
		}
		
		// decreaseBy should not have changed
		if n.decreaseBy != initialDecrease {
			t.Errorf("decreaseBy changed during extended zero state: was %d, now %d", 
				initialDecrease, n.decreaseBy)
		}
		
		// Verify decreaseBy is reasonable (not approaching overflow)
		const reasonable = 1000000 // Much less than overflow range
		if n.decreaseBy < -reasonable || n.decreaseBy > reasonable {
			t.Errorf("decreaseBy has unreasonable value: %d", n.decreaseBy)
		}
		n.mut.Unlock()
	})

	t.Run("extended success at 100", func(t *testing.T) {
		n.mut.Lock()
		n.flowRate = 95
		n.decreaseBy = 0
		
		// Drive to 100
		for n.flowRate < 100 {
			n.open()
		}
		
		initialDecrease := n.decreaseBy
		
		// Stay at 100 for many iterations
		for i := 0; i < 100; i++ {
			n.open()
		}
		
		// decreaseBy should not have changed
		if n.decreaseBy != initialDecrease {
			t.Errorf("decreaseBy changed during extended 100 state: was %d, now %d", 
				initialDecrease, n.decreaseBy)
		}
		
		// Verify decreaseBy is reasonable
		const reasonable = 1000000
		if n.decreaseBy < -reasonable || n.decreaseBy > reasonable {
			t.Errorf("decreaseBy has unreasonable value: %d", n.decreaseBy)
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