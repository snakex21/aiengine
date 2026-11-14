// world/player_test.go — Unit tests for player vertical (jump) physics.

package world

import (
	"math"
	"testing"
)

func TestJumpStartsFromGround(t *testing.T) {
	p := NewPlayerState(0, 0, 0)
	if !p.Grounded {
		t.Fatal("player should start grounded")
	}
	p.Jump()
	if p.Grounded {
		t.Error("player should be airborne after Jump")
	}
	if p.VelZ != JumpVelocity {
		t.Errorf("expected VelZ=%v after jump, got %v", JumpVelocity, p.VelZ)
	}
}

func TestJumpNoDoubleJump(t *testing.T) {
	p := NewPlayerState(0, 0, 0)
	p.Jump()
	p.UpdateVertical(0.05) // rise a little
	velBefore := p.VelZ
	p.Jump() // should be ignored while airborne
	if p.VelZ != velBefore {
		t.Errorf("double jump should be ignored: VelZ changed from %v to %v", velBefore, p.VelZ)
	}
}

func TestUpdateVerticalNoopWhenGrounded(t *testing.T) {
	p := NewPlayerState(0, 0, 0)
	p.UpdateVertical(1.0)
	if p.Z != 0 || p.VelZ != 0 || !p.Grounded {
		t.Errorf("grounded player should not move: Z=%v VelZ=%v Grounded=%v", p.Z, p.VelZ, p.Grounded)
	}
}

func TestJumpRisesThenLands(t *testing.T) {
	p := NewPlayerState(0, 0, 0)
	p.Jump()

	const dt = 1.0 / 60.0
	maxZ := 0.0
	landed := false
	for i := 0; i < 600; i++ { // up to 10 seconds
		p.UpdateVertical(dt)
		if p.Z > maxZ {
			maxZ = p.Z
		}
		if p.Grounded {
			landed = true
			break
		}
	}

	if !landed {
		t.Fatal("player never landed after jumping")
	}
	if maxZ <= 0 {
		t.Errorf("player never left the ground, maxZ=%v", maxZ)
	}
	if maxZ > MaxJumpZ+1e-9 {
		t.Errorf("jump exceeded MaxJumpZ: maxZ=%v limit=%v", maxZ, MaxJumpZ)
	}
	if p.Z != 0 || p.VelZ != 0 {
		t.Errorf("after landing expected Z=0 VelZ=0, got Z=%v VelZ=%v", p.Z, p.VelZ)
	}
}

func TestJumpHeightClamped(t *testing.T) {
	p := NewPlayerState(0, 0, 0)
	p.Jump()
	for i := 0; i < 1000 && !p.Grounded; i++ {
		p.UpdateVertical(1.0 / 120.0)
		if p.Z > MaxJumpZ+1e-9 {
			t.Fatalf("Z exceeded clamp during flight: Z=%v limit=%v", p.Z, MaxJumpZ)
		}
	}
}

func TestPitchClamped(t *testing.T) {
	p := NewPlayerState(0, 0, 0)
	p.AddPitch(100) // way past the limit
	if p.Pitch != PitchLimit {
		t.Errorf("pitch up should clamp to %v, got %v", PitchLimit, p.Pitch)
	}
	p.AddPitch(-100)
	if p.Pitch != -PitchLimit {
		t.Errorf("pitch down should clamp to %v, got %v", -PitchLimit, p.Pitch)
	}
}

func TestPitchAccumulates(t *testing.T) {
	p := NewPlayerState(0, 0, 0)
	p.AddPitch(0.1)
	p.AddPitch(0.05)
	if math.Abs(p.Pitch-0.15) > 1e-9 {
		t.Errorf("expected pitch 0.15, got %v", p.Pitch)
	}
}

func TestStanceSpeedMultiplier(t *testing.T) {
	p := NewPlayerState(0, 0, 0)
	if p.SpeedMultiplier() != 1.0 {
		t.Errorf("standing speed should be 1.0, got %v", p.SpeedMultiplier())
	}
	p.SetStance(StanceCrouch)
	if p.SpeedMultiplier() >= 1.0 {
		t.Errorf("crouch should be slower than standing, got %v", p.SpeedMultiplier())
	}
	p.SetStance(StanceProne)
	if p.SpeedMultiplier() >= speedCrouch {
		t.Errorf("prone should be slower than crouch, got %v", p.SpeedMultiplier())
	}
}

func TestStanceLowersEyeHeight(t *testing.T) {
	p := NewPlayerState(0, 0, 0)
	// Standing eye offset eases to ~0.
	p.UpdateVertical(1.0)
	if math.Abs(p.EyeOffset) > 0.01 {
		t.Errorf("standing eye offset should be ~0, got %v", p.EyeOffset)
	}
	// Crouch: eye height should ease downward over time.
	p.SetStance(StanceCrouch)
	for i := 0; i < 120; i++ {
		p.UpdateVertical(1.0 / 60.0)
	}
	if p.EyeOffset >= 0 {
		t.Errorf("crouch eye offset should be negative, got %v", p.EyeOffset)
	}
	crouchOffset := p.EyeOffset
	// Prone: lower still.
	p.SetStance(StanceProne)
	for i := 0; i < 120; i++ {
		p.UpdateVertical(1.0 / 60.0)
	}
	if p.EyeOffset >= crouchOffset {
		t.Errorf("prone eye offset (%v) should be below crouch (%v)", p.EyeOffset, crouchOffset)
	}
}

func TestProneCannotJump(t *testing.T) {
	p := NewPlayerState(0, 0, 0)
	p.SetStance(StanceProne)
	p.Jump()
	if !p.Grounded || p.VelZ != 0 {
		t.Errorf("prone player should not jump: Grounded=%v VelZ=%v", p.Grounded, p.VelZ)
	}
}

func TestCameraHeightCombinesJumpAndStance(t *testing.T) {
	p := NewPlayerState(0, 0, 0)
	p.Z = 0.2
	p.EyeOffset = -0.1
	if math.Abs(p.CameraHeight()-0.1) > 1e-9 {
		t.Errorf("expected camera height 0.1, got %v", p.CameraHeight())
	}
}
