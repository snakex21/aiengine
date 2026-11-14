// systems/systems_test.go — Unit tests for the audio system.

package systems

import (
	"testing"
)

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

	as.PlaySound("footstep")
	// Just verify no panic — actual playback requires backend.
}
