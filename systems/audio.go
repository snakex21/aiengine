// systems/audio.go — Audio system for AIEngine.
// Provides procedural sound generation (the unique AIEngine feature),
// file-based audio playback (mp3/ogg/wav/flac), and a mixer for 3D spatial audio.
// Procedural sounds are described in .aie SOUND blocks and generated at runtime.

package systems

import (
	"fmt"
	"log"
	"math"
	"math/rand"
)

// --- Procedural Sound Generation ---

// WaveformType enumerates the oscillator shapes.
type WaveformType int

const (
	WaveSine     WaveformType = iota
	WaveSquare
	WaveSawtooth
	WaveTriangle
	WaveNoise     // white noise
	WaveImpact    // short burst with fast decay
	WaveClick     // single sharp tick
)

// SoundDef describes a procedural sound as parsed from .aie SOUND blocks.
type SoundDef struct {
	Name       string
	Type       string  // "impact", "click", "noise", or custom
	Waveform   WaveformType
	Frequency  float64 // Hz
	Decay      float64 // seconds
	Noise      float64 // 0-1 noise mix
	Reverb     float64 // 0-1 reverb amount
	Randomize  float64 // 0-1 random pitch variation
	Continuous bool    // looping sound
	Volume     float64 // 0-1 base volume

	// Envelope (ADSR).
	Attack  float64
	Hold    float64
	Release float64
	Sustain float64

	// Filter.
	FilterType   string  // "lowpass", "highpass"
	FilterCutoff float64 // Hz
}

// DefaultSoundDef returns a SoundDef with sensible defaults.
func DefaultSoundDef() SoundDef {
	return SoundDef{
		Waveform:  WaveSine,
		Frequency: 440,
		Decay:     0.3,
		Volume:    1.0,
		Attack:    0.005,
		Hold:      0.1,
		Release:   0.2,
		Sustain:   0.7,
	}
}

// SoundDefFromAIENode creates a SoundDef from parsed .aie data.
func SoundDefFromProps(name string, props map[string]string) SoundDef {
	sd := DefaultSoundDef()
	sd.Name = name

	if v, ok := props["TYPE"]; ok {
		sd.Type = v
		switch v {
		case "impact":
			sd.Waveform = WaveImpact
			sd.Decay = 0.1
		case "click":
			sd.Waveform = WaveClick
			sd.Decay = 0.05
		case "noise":
			sd.Waveform = WaveNoise
		}
	}
	if v, ok := props["FREQUENCY"]; ok {
		sd.Frequency = parseFloatProp(v)
	}
	if v, ok := props["DECAY"]; ok {
		sd.Decay = parseFloatProp(v)
	}
	if v, ok := props["NOISE"]; ok {
		sd.Noise = parseFloatProp(v)
	}
	if v, ok := props["REVERB"]; ok {
		sd.Reverb = parseFloatProp(v)
	}
	if v, ok := props["RANDOMIZE"]; ok {
		sd.Randomize = parseFloatProp(v)
	}
	if v, ok := props["CONTINUOUS"]; ok {
		sd.Continuous = v == "yes" || v == "true"
	}
	if v, ok := props["VOLUME"]; ok {
		sd.Volume = parseFloatProp(v)
	}
	if v, ok := props["ENVELOPE"]; ok {
		// Parse 4 floats: attack hold release sustain
		floats := parseFloats(v)
		if len(floats) >= 4 {
			sd.Attack = floats[0]
			sd.Hold = floats[1]
			sd.Release = floats[2]
			sd.Sustain = floats[3]
		}
	}
	if v, ok := props["WAVEFORM"]; ok {
		switch v {
		case "sine":
			sd.Waveform = WaveSine
		case "square":
			sd.Waveform = WaveSquare
		case "sawtooth":
			sd.Waveform = WaveSawtooth
		case "triangle":
			sd.Waveform = WaveTriangle
		case "noise":
			sd.Waveform = WaveNoise
		}
	}
	if v, ok := props["FILTER"]; ok {
		parts := splitSpace(v)
		if len(parts) >= 2 {
			sd.FilterType = parts[0]
			sd.FilterCutoff = parseFloatProp(parts[1])
		}
	}

	return sd
}

// GenerateSamples produces PCM float32 samples for this sound definition.
// sampleRate is in Hz (e.g., 44100).
func (sd *SoundDef) GenerateSamples(sampleRate int) []float32 {
	duration := sd.Decay
	if sd.Continuous {
		duration = 2.0 // generate 2 seconds for looping sounds
	}
	if duration <= 0 {
		duration = 0.1
	}

	numSamples := int(float64(sampleRate) * duration)
	samples := make([]float32, numSamples)

	// Randomize frequency slightly.
	freq := sd.Frequency
	if sd.Randomize > 0 {
		freq += (rand.Float64()*2 - 1) * sd.Randomize * freq
	}

	for i := 0; i < numSamples; i++ {
		t := float64(i) / float64(sampleRate)
		phase := 2.0 * math.Pi * freq * t

		// Waveform.
		var sample float64
		switch sd.Waveform {
		case WaveSine:
			sample = math.Sin(phase)
		case WaveSquare:
			if math.Sin(phase) >= 0 {
				sample = 1.0
			} else {
				sample = -1.0
			}
		case WaveSawtooth:
			sample = 2.0*(freq*t-math.Floor(freq*t+0.5))
		case WaveTriangle:
			sample = 2.0*math.Abs(2.0*(freq*t-math.Floor(freq*t+0.5))) - 1.0
		case WaveNoise:
			sample = rand.Float64()*2 - 1
		case WaveImpact:
			// Impact: sine with fast exponential decay.
			sample = math.Sin(phase) * math.Exp(-t/sd.Decay*3)
		case WaveClick:
			// Click: single sharp pulse.
			pulseWidth := 0.002
			if t < pulseWidth {
				sample = 1.0 - t/pulseWidth
			} else {
				sample = 0
			}
		default:
			sample = math.Sin(phase)
		}

		// Mix in noise.
		if sd.Noise > 0 {
			sample = sample*(1-sd.Noise) + (rand.Float64()*2-1)*sd.Noise
		}

		// Envelope (ADSR).
		env := 1.0
		if sd.Attack > 0 && t < sd.Attack {
			env = t / sd.Attack
		} else if t < sd.Attack+sd.Hold {
			env = sd.Sustain + (1.0-sd.Sustain)*(1.0-(t-sd.Attack)/sd.Hold)
		} else if sd.Release > 0 {
			decayT := t - sd.Attack - sd.Hold
			if decayT > 0 {
				env = sd.Sustain * math.Exp(-decayT/sd.Release)
				if env < 0.001 {
					env = 0
				}
			}
		}
		sample *= env

		// Volume.
		sample *= sd.Volume

		// Clamp.
		if sample > 1.0 {
			sample = 1.0
		} else if sample < -1.0 {
			sample = -1.0
		}

		samples[i] = float32(sample)
	}

	// Simple reverb (delay line).
	if sd.Reverb > 0 {
		delaySamples := int(float64(sampleRate) * 0.05) // 50ms delay
		feedback := sd.Reverb * 0.5
		for i := delaySamples; i < numSamples; i++ {
			samples[i] += float32(feedback) * samples[i-delaySamples]
		}
	}

	return samples
}

// --- Audio Mixer ---

// AudioClip is a playing or playable sound instance.
type AudioClip struct {
	Samples    []float32
	Position   int     // current playback position in samples
	Volume     float32
	Pitch      float32 // speed multiplier
	Loop       bool
	Playing    bool
	SampleRate int
}

// NewAudioClip creates a clip from generated samples.
func NewAudioClip(samples []float32, sampleRate int) *AudioClip {
	return &AudioClip{
		Samples:    samples,
		Volume:     1.0,
		Pitch:      1.0,
		Playing:    true,
		SampleRate: sampleRate,
	}
}

// Mix fills a buffer with this clip's audio, advancing the playback position.
func (ac *AudioClip) Mix(buf []float32) {
	if !ac.Playing || len(ac.Samples) == 0 {
		return
	}

	step := float64(ac.Pitch)
	for i := 0; i < len(buf); i++ {
		idx := int(float64(ac.Position))
		if idx >= len(ac.Samples) {
			if ac.Loop {
				ac.Position = 0
				idx = 0
			} else {
				ac.Playing = false
				break
			}
		}

		sample := ac.Samples[idx]

		// Linear interpolation for pitch shifting.
		nextIdx := idx + 1
		if nextIdx >= len(ac.Samples) {
			if ac.Loop {
				nextIdx = 0
			} else {
				nextIdx = idx
			}
		}
		frac := float64(ac.Position) - float64(idx)
		sample = sample*(1.0-float32(frac)) + ac.Samples[nextIdx]*float32(frac)

		buf[i] += sample * ac.Volume
		ac.Position = int(float64(ac.Position) + step)
	}
}

// --- AudioSystem ---

// AudioSystem manages all audio playback and procedural generation.
type AudioSystem struct {
	SampleRate int
	MasterVol  float32
	Clips      []*AudioClip
	MusicClip  *AudioClip

	// Audio backend for real-time playback.
	Backend *AudioBackend

	// Procedural sound library.
	SoundDefs map[string]SoundDef

	// Sound file search paths.
	SoundPaths []string

	// Spatial audio parameters.
	ListenerPos Vec3
}

// NewAudioSystem creates an audio system with default sample rate.
// Call InitBackend() to enable actual audio output.
func NewAudioSystem() *AudioSystem {
	return &AudioSystem{
		SampleRate: 44100,
		MasterVol:  1.0,
		SoundDefs:  make(map[string]SoundDef),
		SoundPaths: []string{"projects/demo/sounds", "."},
	}
}

// InitBackend initializes the oto audio backend for real playback.
func (as *AudioSystem) InitBackend() error {
	backend, err := NewAudioBackend(as.SampleRate, as.SoundPaths)
	if err != nil {
		return err
	}
	as.Backend = backend

	// Cache all registered procedural sounds.
	for name, sd := range as.SoundDefs {
		backend.CacheProcedural(name, sd)
	}

	return nil
}

// RegisterSoundDef stores a procedural sound definition. If backend is already
// running, also caches it immediately.
func (as *AudioSystem) RegisterSoundDef(sd SoundDef) {
	as.SoundDefs[sd.Name] = sd
	if as.Backend != nil {
		as.Backend.CacheProcedural(sd.Name, sd)
	}
}

// PlaySoundDef plays a procedural sound by name (for backward compat).
func (as *AudioSystem) PlaySoundDef(name string) *AudioClip {
	as.PlaySound(name)
	return nil
}

// PlaySound plays a sound by name. Delegates to the backend.
func (as *AudioSystem) PlaySound(name string) {
	if as.Backend != nil {
		as.Backend.PlaySound(name)
	} else {
		log.Printf("[Audio] Backend not ready, cannot play: %s", name)
	}
}

// StopAll stops all audio playback.
func (as *AudioSystem) StopAll() {
	if as.Backend != nil {
		as.Backend.StopAll()
	}
}

// Shutdown releases audio resources.
func (as *AudioSystem) Shutdown() {
	if as.Backend != nil {
		as.Backend.Shutdown()
	}
}

// PlayGenerated generates samples from a SoundDef and starts playback.
func (as *AudioSystem) PlayGenerated(sd SoundDef) *AudioClip {
	samples := sd.GenerateSamples(as.SampleRate)
	clip := NewAudioClip(samples, as.SampleRate)
	clip.Volume = float32(sd.Volume)
	if sd.Continuous {
		clip.Loop = true
	}
	as.Clips = append(as.Clips, clip)
	return clip
}

// PlayMusic loops a generated or file-based music track.
func (as *AudioSystem) PlayMusic(clip *AudioClip) {
	if as.MusicClip != nil {
		as.MusicClip.Playing = false
	}
	clip.Loop = true
	clip.Volume = 0.7
	as.MusicClip = clip
	as.Clips = append(as.Clips, clip)
}

// StopMusic stops the current music.
func (as *AudioSystem) StopMusic() {
	if as.MusicClip != nil {
		as.MusicClip.Playing = false
		as.MusicClip = nil
	}
}

// Mix renders all active clips into a stereo buffer.
func (as *AudioSystem) Mix(buf []float32) {
	// Clear buffer.
	for i := range buf {
		buf[i] = 0
	}

	// Mix all active clips.
	var remaining []*AudioClip
	for _, c := range as.Clips {
		if c.Playing {
			c.Mix(buf)
			remaining = append(remaining, c)
		}
	}
	as.Clips = remaining

	// Apply master volume and clamp.
	for i := range buf {
		buf[i] *= as.MasterVol
		if buf[i] > 1.0 {
			buf[i] = 1.0
		} else if buf[i] < -1.0 {
			buf[i] = -1.0
		}
	}
}

// SetMasterVolume adjusts the global volume.
func (as *AudioSystem) SetMasterVolume(vol float32) {
	if vol < 0 {
		vol = 0
	}
	if vol > 1.0 {
		vol = 1.0
	}
	as.MasterVol = vol
}

// SetListenerPosition updates the 3D audio listener position.
func (as *AudioSystem) SetListenerPosition(pos Vec3) {
	as.ListenerPos = pos
}

// --- Spatial Audio Helper ---

// SpatialVolume calculates volume attenuation based on distance.
// maxDist is the distance at which volume reaches 0.
func SpatialVolume(sourcePos, listenerPos Vec3, maxDist float64) float32 {
	dist := sourcePos.Sub(listenerPos).Length()
	if dist >= maxDist {
		return 0
	}
	// Inverse square attenuation, but clamped.
	attenuation := float32(1.0 / (1.0 + dist*dist*0.1))
	return attenuation
}

// --- Utility ---

func parseFloatProp(s string) float64 {
	var result float64
	fmt.Sscanf(s, "%f", &result)
	return result
}

func parseFloats(s string) []float64 {
	var result []float64
	parts := splitSpace(s)
	for _, p := range parts {
		result = append(result, parseFloatProp(p))
	}
	return result
}

func splitSpace(s string) []string {
	var parts []string
	current := ""
	for _, ch := range s {
		if ch == ' ' || ch == '\t' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}
