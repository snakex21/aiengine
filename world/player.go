package world

import "math"

// Stance enumerates the player's body posture, which controls eye height and
// movement speed.
type Stance int

const (
	StanceStand Stance = iota
	StanceCrouch
	StanceProne
)

// PlayerState holds the player's position and orientation for the raycaster.
type PlayerState struct {
	X        float64
	Y        float64
	Z        float64 // jump height above ground, in wall-height units (0 = on floor)
	VelZ     float64 // vertical velocity for jumping/falling
	Grounded bool    // true while standing on the ground

	Angle float64 // yaw (left/right look), radians
	Pitch float64 // vertical look, as a fraction of screen height [-PitchLimit, PitchLimit]

	Stance    Stance  // current posture
	EyeOffset float64 // smoothed eye-height offset for the current stance (<= 0)

	// Ground is the absolute height of the floor the player stands on (or
	// falls towards); Z is measured from it. StepOffset smooths the camera
	// after walking up a step (negative, eases back to 0). LastFall is the
	// height of the last completed fall (set on landing; the host reads and
	// clears it for fall damage).
	Ground     float64
	StepOffset float64
	LastFall   float64
	airPeak    float64 // highest absolute feet height of the current flight
	jumpTop    float64 // absolute feet height where the current jump stops rising

	FOV       float64
	MoveSpeed float64
	RotSpeed  float64
}

func NewPlayerState(x, y, angle float64) *PlayerState {
	return &PlayerState{
		X:         x,
		Y:         y,
		Z:         0,
		Grounded:  true,
		Angle:     angle,
		Stance:    StanceStand,
		FOV:       math.Pi / 2.7, // ~66 degrees
		MoveSpeed: 3.0,
		RotSpeed:  3.0,
	}
}

// Jump physics tuning. Z is in wall-height units where 0 is standing on the
// floor. Values are chosen so a jump peaks at roughly half a wall height.
const (
	JumpVelocity = 4.5  // initial upward velocity (units/sec)
	GravityAccel = 16.0 // downward acceleration (units/sec^2)
	MaxJumpZ     = 0.45 // clamp on peak height
)

// Look and stance tuning.
const (
	PitchLimit = 0.35 // max up/down look as a fraction of screen height

	eyeStand  = 0.0   // eye-height offset per stance (wall-height units, <= 0)
	eyeCrouch = -0.18 // crouched: lower camera
	eyeProne  = -0.34 // prone: lowest camera
	eyeLerp   = 10.0  // eye-height smoothing rate (per second)
	stepLerp  = 18.0  // step-up camera smoothing rate (per second)

	speedCrouch = 0.5  // movement speed multiplier while crouched
	speedProne  = 0.28 // movement speed multiplier while prone
)

// Jump starts an upward jump if the player is currently grounded.
// It is a no-op while airborne (no double jump) or while prone.
func (p *PlayerState) Jump() {
	if p.Grounded && p.Stance != StanceProne {
		p.VelZ = JumpVelocity
		p.Grounded = false
		p.airPeak = p.Ground + p.Z
		p.jumpTop = p.Ground + p.Z + MaxJumpZ
	}
}

// SetStance changes the player's posture. Jumping is only allowed when
// standing, and the eye height eases toward the new stance in UpdateVertical.
func (p *PlayerState) SetStance(s Stance) {
	p.Stance = s
}

// stanceEyeTarget returns the eye-height offset for the current stance.
func (p *PlayerState) stanceEyeTarget() float64 {
	switch p.Stance {
	case StanceCrouch:
		return eyeCrouch
	case StanceProne:
		return eyeProne
	default:
		return eyeStand
	}
}

// SpeedMultiplier returns the movement-speed scale for the current stance.
func (p *PlayerState) SpeedMultiplier() float64 {
	switch p.Stance {
	case StanceCrouch:
		return speedCrouch
	case StanceProne:
		return speedProne
	default:
		return 1.0
	}
}

// CameraHeight is the total camera height above the floor: jump height plus the
// (negative) stance eye offset. Used for distance-scaled vertical parallax.
func (p *PlayerState) CameraHeight() float64 {
	return p.Z + p.EyeOffset + p.StepOffset
}

// Feet is the absolute height of the player's feet.
func (p *PlayerState) Feet() float64 { return p.Ground + p.Z }

// EyeZ is the absolute camera height (feet + eye height + stance/step).
func (p *PlayerState) EyeZ() float64 { return p.Ground + EyeHeight + p.CameraHeight() }

// Headroom is the free height the current stance needs.
func (p *PlayerState) Headroom() float64 {
	switch p.Stance {
	case StanceCrouch:
		return HeadroomCrouch
	case StanceProne:
		return HeadroomProne
	default:
		return HeadroomStand
	}
}

// SetGround tells the player the floor height under it after moving.
// Stepping up (the host's collision allows it up to MaxStepUp, or onto a
// ledge below the feet while jumping) lands the player there with a smooth
// camera; stepping off a ledge starts a fall.
func (p *PlayerState) SetGround(h float64) {
	if h == p.Ground {
		return
	}
	feet := p.Ground + p.Z
	if h > feet { // walked up a step
		p.StepOffset -= h - feet
		p.StepOffset = math.Max(p.StepOffset, -2*MaxStepUp)
		p.Ground, p.Z = h, 0
		if !p.Grounded {
			p.land()
		}
		return
	}
	// Floor at or below the feet: the feet stay where they are.
	p.Ground, p.Z = h, feet-h
	if p.Z <= 1e-9 {
		p.Z = 0
		return
	}
	if p.Grounded {
		p.Grounded = false
		p.VelZ = 0
		p.airPeak = feet
		p.jumpTop = feet
	}
}

// Ride moves a player standing on a moving floor with it: when the floor
// under the feet went from before to after (a lift, a platform), the player
// keeps standing on it (no fall, no step smoothing). It reports whether the
// player was carried.
func (p *PlayerState) Ride(before, after float64) bool {
	if !p.Grounded || before == after || math.Abs(p.Feet()-before) > 1e-6 {
		return false
	}
	p.Ground, p.Z = after, 0
	return true
}

// PlaceAt puts the player on the ground at height h (spawn, teleport): no
// fall, no camera smoothing.
func (p *PlayerState) PlaceAt(h float64) {
	p.Ground, p.Z, p.VelZ, p.StepOffset, p.LastFall = h, 0, 0, 0, 0
	p.Grounded = true
}

func (p *PlayerState) land() {
	p.LastFall = math.Max(0, p.airPeak-(p.Ground+p.Z))
	p.Z, p.VelZ, p.Grounded = 0, 0, true
}

// AddPitch adjusts the vertical look angle and clamps it to the allowed range.
func (p *PlayerState) AddPitch(delta float64) {
	p.Pitch += delta
	if p.Pitch > PitchLimit {
		p.Pitch = PitchLimit
	}
	if p.Pitch < -PitchLimit {
		p.Pitch = -PitchLimit
	}
}

// UpdateVertical advances the player's vertical position by dt seconds,
// applying gravity while airborne and easing the eye height toward the current
// stance. Landing settles the player on the ground at Z=0.
func (p *PlayerState) UpdateVertical(dt float64) {
	// Ease eye height toward the stance target.
	target := p.stanceEyeTarget()
	t := eyeLerp * dt
	if t > 1 {
		t = 1
	}
	p.EyeOffset += (target - p.EyeOffset) * t
	// Ease the camera back up after a step (fast, ~0.1 s).
	if p.StepOffset != 0 {
		p.StepOffset -= p.StepOffset * math.Min(1, stepLerp*dt)
		if math.Abs(p.StepOffset) < 1e-3 {
			p.StepOffset = 0
		}
	}

	if p.Grounded {
		return
	}
	p.VelZ -= GravityAccel * dt
	p.Z += p.VelZ * dt
	if p.VelZ > 0 && p.Ground+p.Z >= p.jumpTop {
		p.Z = p.jumpTop - p.Ground
		p.VelZ = 0
	}
	p.airPeak = math.Max(p.airPeak, p.Ground+p.Z)
	if p.Z <= 0 {
		p.Z = 0
		p.land()
	}
}
