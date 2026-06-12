// renderer/player_test.go — Unit tests for player vertical (jump) physics.

package renderer

import "testing"

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
