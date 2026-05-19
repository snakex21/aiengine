// systems/physics.go — Physics system for AIEngine.
// Provides gravity simulation, collision detection (AABB, ray, sphere),
// rigidbody dynamics, and ragdoll/death behavior physics.
// All parameters are configurable via .aie MAP_PROPS.

package systems

import (
	"math"
)

// Vec2 represents a 2D vector or position.
type Vec2 struct {
	X, Y float64
}

// Vec3 represents a 3D vector.
type Vec3 struct {
	X, Y, Z float64
}

// Add returns v + other.
func (v Vec3) Add(other Vec3) Vec3 {
	return Vec3{v.X + other.X, v.Y + other.Y, v.Z + other.Z}
}

// Sub returns v - other.
func (v Vec3) Sub(other Vec3) Vec3 {
	return Vec3{v.X - other.X, v.Y - other.Y, v.Z - other.Z}
}

// Scale multiplies v by s.
func (v Vec3) Scale(s float64) Vec3 {
	return Vec3{v.X * s, v.Y * s, v.Z * s}
}

// Length returns the magnitude of v.
func (v Vec3) Length() float64 {
	return math.Sqrt(v.X*v.X + v.Y*v.Y + v.Z*v.Z)
}

// Normalize returns a unit vector in the direction of v.
func (v Vec3) Normalize() Vec3 {
	l := v.Length()
	if l < 0.0001 {
		return Vec3{}
	}
	return Vec3{v.X / l, v.Y / l, v.Z / l}
}

// Dot returns the dot product.
func (v Vec3) Dot(other Vec3) float64 {
	return v.X*other.X + v.Y*other.Y + v.Z*other.Z
}

// Cross returns the cross product.
func (v Vec3) Cross(other Vec3) Vec3 {
	return Vec3{
		v.Y*other.Z - v.Z*other.Y,
		v.Z*other.X - v.X*other.Z,
		v.X*other.Y - v.Y*other.X,
	}
}

// AABB represents an axis-aligned bounding box.
type AABB struct {
	Min, Max Vec3
}

// NewAABB creates an AABB from center and half-extents.
func NewAABB(center Vec3, halfExtents Vec3) AABB {
	return AABB{
		Min: center.Sub(halfExtents),
		Max: center.Add(halfExtents),
	}
}

// Intersects returns true if two AABBs overlap.
func (a AABB) Intersects(b AABB) bool {
	return a.Min.X <= b.Max.X && a.Max.X >= b.Min.X &&
		a.Min.Y <= b.Max.Y && a.Max.Y >= b.Min.Y &&
		a.Min.Z <= b.Max.Z && a.Max.Z >= b.Min.Z
}

// Contains returns true if the point is inside the AABB.
func (a AABB) Contains(p Vec3) bool {
	return p.X >= a.Min.X && p.X <= a.Max.X &&
		p.Y >= a.Min.Y && p.Y <= a.Max.Y &&
		p.Z >= a.Min.Z && p.Z <= a.Max.Z
}

// ClosestPoint returns the closest point on the AABB to the given point.
func (a AABB) ClosestPoint(p Vec3) Vec3 {
	clamp := func(v, lo, hi float64) float64 {
		if v < lo {
			return lo
		}
		if v > hi {
			return hi
		}
		return v
	}
	return Vec3{
		clamp(p.X, a.Min.X, a.Max.X),
		clamp(p.Y, a.Min.Y, a.Max.Y),
		clamp(p.Z, a.Min.Z, a.Max.Z),
	}
}

// Sphere represents a bounding sphere.
type Sphere struct {
	Center Vec3
	Radius float64
}

// IntersectsSphere returns true if two spheres overlap.
func (s Sphere) IntersectsSphere(other Sphere) bool {
	dist := s.Center.Sub(other.Center).Length()
	return dist < s.Radius+other.Radius
}

// IntersectsAABB returns true if a sphere intersects an AABB.
func (s Sphere) IntersectsAABB(a AABB) bool {
	closest := a.ClosestPoint(s.Center)
	dist := s.Center.Sub(closest).Length()
	return dist < s.Radius
}

// Ray represents a ray with origin and direction.
type Ray struct {
	Origin    Vec3
	Direction Vec3
}

// RaycastResult holds the result of a ray intersection test.
type RaycastResult struct {
	Hit      bool
	Distance float64
	Point    Vec3
	Normal   Vec3
}

// IntersectAABB tests a ray against an AABB.
func (r Ray) IntersectAABB(a AABB) RaycastResult {
	dirFrac := Vec3{
		1.0 / r.Direction.X,
		1.0 / r.Direction.Y,
		1.0 / r.Direction.Z,
	}

	t1 := (a.Min.X - r.Origin.X) * dirFrac.X
	t2 := (a.Max.X - r.Origin.X) * dirFrac.X
	t3 := (a.Min.Y - r.Origin.Y) * dirFrac.Y
	t4 := (a.Max.Y - r.Origin.Y) * dirFrac.Y
	t5 := (a.Min.Z - r.Origin.Z) * dirFrac.Z
	t6 := (a.Max.Z - r.Origin.Z) * dirFrac.Z

	tMin := math.Max(math.Max(math.Min(t1, t2), math.Min(t3, t4)), math.Min(t5, t6))
	tMax := math.Min(math.Min(math.Max(t1, t2), math.Max(t3, t4)), math.Max(t5, t6))

	if tMax < 0 || tMin > tMax {
		return RaycastResult{Hit: false}
	}

	return RaycastResult{
		Hit:      true,
		Distance: tMin,
		Point:    r.Origin.Add(r.Direction.Scale(tMin)),
	}
}

// --- Rigidbody ---

// Rigidbody holds physics state for a dynamic entity.
type Rigidbody struct {
	Position     Vec3
	Velocity     Vec3
	Acceleration Vec3
	Mass         float64
	UseGravity   bool
	IsKinematic  bool // kinematic bodies are not affected by forces
	IsStatic     bool // static bodies never move
	Grounded     bool
	Friction     float64
	Bounciness   float64
	Drag         float64 // linear damping
}

// NewRigidbody creates a dynamic rigidbody at the given position.
func NewRigidbody(pos Vec3, mass float64) *Rigidbody {
	return &Rigidbody{
		Position:   pos,
		Mass:       mass,
		UseGravity: true,
		Friction:   0.8,
		Bounciness: 0.1,
		Drag:       0.95,
	}
}

// --- PhysicsWorld ---

// PhysicsWorld manages all physics objects and global settings.
type PhysicsWorld struct {
	Gravity    float64
	AirDrag    float64
	Bodies     []*Rigidbody
	StaticAABB []AABB // collision geometry for walls/floors
}

// NewPhysicsWorld creates a physics world with default gravity (Earth-like).
func NewPhysicsWorld() *PhysicsWorld {
	return &PhysicsWorld{
		Gravity: 9.8,
		AirDrag: 0.01,
	}
}

// AddBody registers a rigidbody with the world.
func (pw *PhysicsWorld) AddBody(rb *Rigidbody) {
	pw.Bodies = append(pw.Bodies, rb)
}

// RemoveBody removes a rigidbody from the simulation.
func (pw *PhysicsWorld) RemoveBody(rb *Rigidbody) {
	for i, b := range pw.Bodies {
		if b == rb {
			pw.Bodies = append(pw.Bodies[:i], pw.Bodies[i+1:]...)
			return
		}
	}
}

// AddStaticCollider adds a static AABB for collision.
func (pw *PhysicsWorld) AddStaticCollider(a AABB) {
	pw.StaticAABB = append(pw.StaticAABB, a)
}

// Step advances the physics simulation by dt seconds.
func (pw *PhysicsWorld) Step(dt float64) {
	for _, rb := range pw.Bodies {
		if rb.IsStatic {
			continue
		}

		// Apply gravity.
		if rb.UseGravity && !rb.Grounded {
			rb.Acceleration.Z -= pw.Gravity
		}

		// Apply drag.
		rb.Velocity = rb.Velocity.Scale(1.0 - pw.AirDrag*dt)

		// Semi-implicit Euler integration.
		rb.Velocity = rb.Velocity.Add(rb.Acceleration.Scale(dt))
		rb.Position = rb.Position.Add(rb.Velocity.Scale(dt))

		// Reset acceleration.
		rb.Acceleration = Vec3{}

		// Ground check and collision resolution.
		rb.Grounded = false
		playerAABB := NewAABB(rb.Position, Vec3{0.3, 0.3, 0.9})
		for _, a := range pw.StaticAABB {
			if playerAABB.Intersects(a) {
				rg := pw.ResolveAABB(rb, playerAABB, a)
				if rg.Normal.Z > 0.5 {
					rb.Grounded = true
					rb.Velocity.Z = 0
				}
				_ = rg
			}
		}

		// Floor clamp.
		if rb.Position.Z < 0 {
			rb.Position.Z = 0
			rb.Velocity.Z = 0
			rb.Grounded = true
		}
	}
}

// ResolveAABB pushes a rigidbody out of a static AABB and applies bounce.
func (pw *PhysicsWorld) ResolveAABB(rb *Rigidbody, dynamic, static AABB) RaycastResult {
	// Calculate overlap and push out along the minimum penetration axis.
	overlapX := math.Min(dynamic.Max.X-static.Min.X, static.Max.X-dynamic.Min.X)
	overlapY := math.Min(dynamic.Max.Y-static.Min.Y, static.Max.Y-dynamic.Min.Y)
	overlapZ := math.Min(dynamic.Max.Z-static.Min.Z, static.Max.Z-dynamic.Min.Z)

	var normal Vec3
	var penetration float64

	if overlapX < overlapY && overlapX < overlapZ {
		penetration = overlapX
		if dynamic.Max.X-static.Min.X < static.Max.X-dynamic.Min.X {
			normal = Vec3{-1, 0, 0}
		} else {
			normal = Vec3{1, 0, 0}
		}
	} else if overlapY < overlapZ {
		penetration = overlapY
		if dynamic.Max.Y-static.Min.Y < static.Max.Y-dynamic.Min.Y {
			normal = Vec3{0, -1, 0}
		} else {
			normal = Vec3{0, 1, 0}
		}
	} else {
		penetration = overlapZ
		if dynamic.Max.Z-static.Min.Z < static.Max.Z-dynamic.Min.Z {
			normal = Vec3{0, 0, -1}
		} else {
			normal = Vec3{0, 0, 1}
		}
	}

	// Push out.
	rb.Position = rb.Position.Add(normal.Scale(penetration))

	// Reflect velocity with bounce.
	velDotNormal := rb.Velocity.Dot(normal)
	if velDotNormal < 0 {
		rb.Velocity = rb.Velocity.Sub(normal.Scale((1 + rb.Bounciness) * velDotNormal))
		// Friction.
		tangent := rb.Velocity.Sub(normal.Scale(rb.Velocity.Dot(normal)))
		rb.Velocity = rb.Velocity.Sub(tangent.Scale(rb.Friction))
	}

	return RaycastResult{
		Hit:      true,
		Normal:   normal,
		Distance: penetration,
		Point:    rb.Position,
	}
}

// Raycast tests a ray against all static colliders.
func (pw *PhysicsWorld) Raycast(ray Ray) RaycastResult {
	closest := RaycastResult{Hit: false, Distance: math.MaxFloat64}

	for _, a := range pw.StaticAABB {
		result := ray.IntersectAABB(a)
		if result.Hit && result.Distance < closest.Distance {
			closest = result
		}
	}

	return closest
}

// --- Ragdoll System ---

// RagdollBone represents a single bone in a ragdoll chain.
type RagdollBone struct {
	Position     Vec3
	PrevPosition Vec3 // Verlet integration
	Parent       *RagdollBone
	Length       float64
	Mass         float64
	Fixed        bool // if true, bone doesn't move
}

// Ragdoll represents a chain of bones for death physics.
type Ragdoll struct {
	Bones        []*RagdollBone
	Gravity      float64
	Damping      float64
	Iterations   int // constraint solver iterations
}

// NewRagdoll creates a ragdoll from a chain of positions.
func NewRagdoll(positions []Vec3, masses []float64) *Ragdoll {
	r := &Ragdoll{
		Gravity:    9.8,
		Damping:    0.98,
		Iterations: 5,
	}

	var parent *RagdollBone
	for i, pos := range positions {
		mass := 1.0
		if i < len(masses) {
			mass = masses[i]
		}
		bone := &RagdollBone{
			Position:     pos,
			PrevPosition: pos,
			Parent:       parent,
			Mass:         mass,
		}
		if parent != nil {
			bone.Length = parent.Position.Sub(pos).Length()
		}
		r.Bones = append(r.Bones, bone)
		parent = bone
	}

	return r
}

// Step advances the ragdoll simulation.
func (r *Ragdoll) Step(dt float64) {
	// Verlet integration.
	for _, bone := range r.Bones {
		if bone.Fixed {
			continue
		}
		vel := bone.Position.Sub(bone.PrevPosition).Scale(r.Damping)
		bone.PrevPosition = bone.Position
		bone.Position = bone.Position.Add(vel)
		// Gravity.
		bone.Position.Z -= r.Gravity * dt * dt
	}

	// Constraint solving (distance constraints).
	for iter := 0; iter < r.Iterations; iter++ {
		for _, bone := range r.Bones {
			if bone.Parent == nil || bone.Fixed {
				continue
			}
			parent := bone.Parent

			delta := bone.Position.Sub(parent.Position)
			dist := delta.Length()
			if dist < 0.0001 {
				continue
			}

			correction := delta.Scale((bone.Length - dist) / dist * 0.5)

			totalMass := bone.Mass + parent.Mass
			ratioA := parent.Mass / totalMass
			ratioB := bone.Mass / totalMass

			if !parent.Fixed {
				parent.Position = parent.Position.Sub(correction.Scale(ratioB))
			}
			bone.Position = bone.Position.Add(correction.Scale(ratioA))
		}
	}

	// Floor constraint.
	for _, bone := range r.Bones {
		if bone.Position.Z < 0 {
			bone.Position.Z = 0
			bone.PrevPosition.Z = bone.Position.Z
		}
	}
}

// --- Death Behavior Helpers ---

// DeathBehaviorType enumerates ragdoll/collapse/explode behaviors.
type DeathBehaviorType int

const (
	DeathRagdoll  DeathBehaviorType = iota // fall limp
	DeathDissolve                           // fade away
	DeathExplode                            // burst outward
	DeathCollapse                           // sink down
	DeathShatter                            // break into pieces
)

// DeathBehavior configures what happens when an entity dies.
type DeathBehavior struct {
	Type           DeathBehaviorType
	Ragdoll        *Ragdoll
	DissolveTime   float64
	ExplodeForce   float64
	CollapseSpeed  float64
	ShatterPieces  int
	EarthquakeIntensity float64
	Active         bool
	Timer          float64
}

// UpdateDeathBehavior advances death animation timers.
func (db *DeathBehavior) UpdateDeathBehavior(dt float64) {
	if !db.Active {
		return
	}
	db.Timer += dt
	if db.Ragdoll != nil {
		db.Ragdoll.Step(dt)
	}
}

// TriggerDeathBehavior activates the death behavior for an entity.
func TriggerDeathBehavior(behaviorType DeathBehaviorType, position Vec3) *DeathBehavior {
	db := &DeathBehavior{
		Type:   behaviorType,
		Active: true,
	}

	switch behaviorType {
	case DeathRagdoll:
		// Create a simple ragdoll from a few segments.
		positions := []Vec3{
			position.Add(Vec3{0, 0, 1.8}), // head
			position.Add(Vec3{0, 0, 1.2}), // torso
			position.Add(Vec3{0, 0, 0.6}), // hips
			position.Add(Vec3{0, 0, 0.0}), // feet
		}
		masses := []float64{5, 15, 10, 5}
		db.Ragdoll = NewRagdoll(positions, masses)

	case DeathDissolve:
		db.DissolveTime = 3.0

	case DeathExplode:
		db.ExplodeForce = 10.0

	case DeathCollapse:
		db.CollapseSpeed = 2.0

	case DeathShatter:
		db.ShatterPieces = 8
	}

	return db
}
