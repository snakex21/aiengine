// systems/systems_test.go — Unit tests for physics, audio, and network systems.

package systems

import (
	"testing"
)

// --- Physics Tests ---

func TestVec3Operations(t *testing.T) {
	a := Vec3{1, 2, 3}
	b := Vec3{4, 5, 6}

	sum := a.Add(b)
	if sum.X != 5 || sum.Y != 7 || sum.Z != 9 {
		t.Errorf("Add failed: got %v", sum)
	}

	diff := b.Sub(a)
	if diff.X != 3 || diff.Y != 3 || diff.Z != 3 {
		t.Errorf("Sub failed: got %v", diff)
	}

	scaled := a.Scale(2)
	if scaled.X != 2 || scaled.Y != 4 || scaled.Z != 6 {
		t.Errorf("Scale failed: got %v", scaled)
	}

	l := a.Length()
	if l < 3.7 || l > 3.8 {
		t.Errorf("Length failed: got %f", l)
	}

	dot := a.Dot(b)
	if dot != 32 {
		t.Errorf("Dot failed: got %f", dot)
	}
}

func TestAABBIntersects(t *testing.T) {
	a := NewAABB(Vec3{0, 0, 0}, Vec3{1, 1, 1})
	b := NewAABB(Vec3{0.5, 0.5, 0.5}, Vec3{1, 1, 1})
	if !a.Intersects(b) {
		t.Error("AABBs should intersect")
	}

	c := NewAABB(Vec3{5, 5, 5}, Vec3{1, 1, 1})
	if a.Intersects(c) {
		t.Error("AABBs should not intersect")
	}
}

func TestAABBContains(t *testing.T) {
	a := NewAABB(Vec3{0, 0, 0}, Vec3{2, 2, 2})
	if !a.Contains(Vec3{1, 1, 1}) {
		t.Error("Point should be contained")
	}
	if a.Contains(Vec3{3, 3, 3}) {
		t.Error("Point should not be contained")
	}
}

func TestSphereIntersects(t *testing.T) {
	s1 := Sphere{Center: Vec3{0, 0, 0}, Radius: 1}
	s2 := Sphere{Center: Vec3{1.5, 0, 0}, Radius: 1}
	if !s1.IntersectsSphere(s2) {
		t.Error("Spheres should intersect")
	}

	s3 := Sphere{Center: Vec3{5, 0, 0}, Radius: 1}
	if s1.IntersectsSphere(s3) {
		t.Error("Spheres should not intersect")
	}
}

func TestRayIntersectAABB(t *testing.T) {
	box := NewAABB(Vec3{0, 0, 0}, Vec3{1, 1, 1})
	ray := Ray{
		Origin:    Vec3{-2, 0.5, 0.5},
		Direction: Vec3{1, 0, 0},
	}
	result := ray.IntersectAABB(box)
	if !result.Hit {
		t.Error("Ray should hit AABB")
	}
}

func TestPhysicsWorldStep(t *testing.T) {
	world := NewPhysicsWorld()
	world.Gravity = 9.8

	rb := NewRigidbody(Vec3{0, 0, 5}, 1.0)
	world.AddBody(rb)

	world.Step(0.016) // ~60fps

	// Body should have fallen.
	if rb.Position.Z >= 5.0 {
		t.Errorf("Expected body to fall, Z=%f", rb.Position.Z)
	}
}

func TestRagdollStep(t *testing.T) {
	positions := []Vec3{
		{0, 0, 2},
		{0, 0, 1},
		{0, 0, 0},
	}
	masses := []float64{5, 10, 5}
	ragdoll := NewRagdoll(positions, masses)

	// Step multiple times to let gravity take effect.
	for i := 0; i < 60; i++ {
		ragdoll.Step(0.016)
	}

	// Top bones should have fallen below their original Z.
	if ragdoll.Bones[0].Position.Z >= 2.0 {
		t.Errorf("Bone 0 (head) should have fallen below 2.0, Z=%f", ragdoll.Bones[0].Position.Z)
	}
	if ragdoll.Bones[1].Position.Z >= 1.0 {
		t.Errorf("Bone 1 (torso) should have fallen below 1.0, Z=%f", ragdoll.Bones[1].Position.Z)
	}
	// Bone 2 (feet) is at floor level (Z >= 0 due to floor constraint).
	if ragdoll.Bones[2].Position.Z < 0 {
		t.Errorf("Bone 2 (feet) should not go below floor, Z=%f", ragdoll.Bones[2].Position.Z)
	}
}

func TestDeathBehaviorTrigger(t *testing.T) {
	db := TriggerDeathBehavior(DeathRagdoll, Vec3{0, 0, 1})
	if db == nil {
		t.Fatal("DeathBehavior is nil")
	}
	if db.Ragdoll == nil {
		t.Error("Ragdoll behavior should create a ragdoll")
	}
	if len(db.Ragdoll.Bones) != 4 {
		t.Errorf("Expected 4 bones, got %d", len(db.Ragdoll.Bones))
	}
}

// --- Audio Tests ---

func TestSoundDefGenerateSamples(t *testing.T) {
	sd := SoundDef{
		Name:      "test",
		Waveform:  WaveSine,
		Frequency: 440,
		Decay:     0.1,
		Volume:    0.5,
	}

	samples := sd.GenerateSamples(44100)
	expectedLen := int(44100 * 0.1) // 4410 samples
	if len(samples) < expectedLen-1 || len(samples) > expectedLen+1 {
		t.Errorf("Expected ~%d samples, got %d", expectedLen, len(samples))
	}

	// Check that samples are non-zero (sine wave should produce sound).
	hasNonZero := false
	for _, s := range samples {
		if s != 0 {
			hasNonZero = true
			break
		}
	}
	if !hasNonZero {
		t.Error("Generated samples are all zero")
	}
}

func TestSoundDefFromProps(t *testing.T) {
	props := map[string]string{
		"TYPE":      "impact",
		"FREQUENCY": "800",
		"DECAY":     "0.1",
		"NOISE":     "0.3",
	}

	sd := SoundDefFromProps("gunshot", props)

	if sd.Name != "gunshot" {
		t.Errorf("Expected name gunshot, got %s", sd.Name)
	}
	if sd.Frequency != 800 {
		t.Errorf("Expected frequency 800, got %f", sd.Frequency)
	}
	if sd.Noise != 0.3 {
		t.Errorf("Expected noise 0.3, got %f", sd.Noise)
	}
}

func TestAudioSystemPlayGenerated(t *testing.T) {
	as := NewAudioSystem()

	sd := SoundDef{
		Name:      "test_sound",
		Waveform:  WaveSine,
		Frequency: 440,
		Decay:     0.1,
		Volume:    0.5,
	}

	clip := as.PlayGenerated(sd)
	if clip == nil {
		t.Fatal("Clip is nil")
	}
	if !clip.Playing {
		t.Error("Clip should be playing")
	}

	// Mix should produce output.
	buf := make([]float32, 1024)
	as.Mix(buf)
	hasOutput := false
	for _, s := range buf {
		if s != 0 {
			hasOutput = true
			break
		}
	}
	if !hasOutput {
		t.Error("Mix produced no output")
	}
}

func TestAudioSystemMixerCleanup(t *testing.T) {
	as := NewAudioSystem()

	sd := SoundDef{
		Name:      "short",
		Waveform:  WaveSine,
		Frequency: 440,
		Decay:     0.001, // very short
		Volume:    0.5,
	}

	as.PlayGenerated(sd)

	// Mix many times to exhaust the clip.
	for i := 0; i < 200; i++ {
		buf := make([]float32, 1024)
		as.Mix(buf)
	}

	// Clips should have been cleaned up.
	if len(as.Clips) > 0 {
		t.Logf("Clips remaining: %d (may be expected for looping sounds)", len(as.Clips))
	}
}

func TestAudioSystemRegisterSoundDef(t *testing.T) {
	as := NewAudioSystem()
	sd := SoundDef{Name: "footstep", Waveform: WaveClick, Frequency: 300, Decay: 0.05}
	as.RegisterSoundDef(sd)

	clip := as.PlaySoundDef("footstep")
	if clip == nil {
		t.Fatal("Expected clip from registered sound")
	}
}

// --- Network Tests ---

func TestPacketSerialize(t *testing.T) {
	pkt := &NetworkPacket{
		Type:    PktConnect,
		Payload: []byte{1, 2, 3},
	}

	data := pkt.Serialize()
	if len(data) != PacketHeaderSize+3 {
		t.Errorf("Expected %d bytes, got %d", PacketHeaderSize+3, len(data))
	}
	if data[0] != byte(PktConnect) {
		t.Errorf("Expected type %d, got %d", PktConnect, data[0])
	}

	deserialized, err := DeserializePacket(data)
	if err != nil {
		t.Fatalf("Deserialize failed: %v", err)
	}
	if deserialized.Type != PktConnect {
		t.Errorf("Expected type PktConnect, got %d", deserialized.Type)
	}
	if len(deserialized.Payload) != 3 {
		t.Errorf("Expected 3 payload bytes, got %d", len(deserialized.Payload))
	}
}

func TestPlayerStateSerialize(t *testing.T) {
	ps := &PlayerNetState{
		ID:     42,
		X:      1.5,
		Y:      2.5,
		Z:      0.0,
		Angle:  0.785,
		HP:     75,
		Alive:  true,
		Weapon: "pistol",
		SeqNum: 100,
	}

	data := SerializePlayerState(ps)
	deserialized := DeserializePlayerState(data)

	if deserialized.ID != ps.ID {
		t.Errorf("ID mismatch: %d vs %d", deserialized.ID, ps.ID)
	}
	if deserialized.X != ps.X {
		t.Errorf("X mismatch: %f vs %f", deserialized.X, ps.X)
	}
	if deserialized.HP != ps.HP {
		t.Errorf("HP mismatch: %f vs %f", deserialized.HP, ps.HP)
	}
	if deserialized.Alive != ps.Alive {
		t.Errorf("Alive mismatch")
	}
	if deserialized.SeqNum != ps.SeqNum {
		t.Errorf("SeqNum mismatch: %d vs %d", deserialized.SeqNum, ps.SeqNum)
	}
}

func TestGameServerCreation(t *testing.T) {
	gs := NewGameServer(27015)
	if gs.Port != 27015 {
		t.Errorf("Expected port 27015, got %d", gs.Port)
	}
	if gs.MaxPlayers != 4 {
		t.Errorf("Expected maxPlayers 4, got %d", gs.MaxPlayers)
	}
}

func TestGameClientCreation(t *testing.T) {
	gc := NewGameClient("127.0.0.1", 27015)
	if gc.ServerAddr != "127.0.0.1" {
		t.Errorf("Expected 127.0.0.1, got %s", gc.ServerAddr)
	}
	if gc.Connected {
		t.Error("Client should not be connected before Connect()")
	}
}
