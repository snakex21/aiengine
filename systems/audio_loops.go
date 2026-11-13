// systems/audio_loops.go — looping sound effects (moving doors, lifts,
// crushers) and one-shots with a volume, both keyed so the game can start,
// re-volume and stop them. Loops reuse the music LoopReader on their own
// oto player each.

package systems

import (
	"encoding/binary"

	oto "github.com/ebitengine/oto/v3"
)

func pcmBytes(pcm []int16) []byte {
	data := make([]byte, len(pcm)*2)
	for i, v := range pcm {
		binary.LittleEndian.PutUint16(data[i*2:], uint16(v))
	}
	return data
}

// PlayLoop starts (or keeps) a looping sound under key at volume vol (0..1,
// scaled by the sfx volume). Playing a different sound under the same key
// replaces it.
func (ab *AudioBackend) PlayLoop(key, name string, vol float64) {
	ab.mu.Lock()
	if l, ok := ab.loops[key]; ok && l.name == name {
		l.vol = vol
		l.p.SetVolume(vol * ab.sfxVol)
		ab.mu.Unlock()
		return
	}
	ab.mu.Unlock()
	pcm := ab.resolvePCM(name)
	if len(pcm) == 0 {
		return
	}
	p := ab.ctx.NewPlayer(&LoopReader{Data: pcmBytes(pcm)})
	ab.StopLoop(key)
	ab.mu.Lock()
	if ab.loops == nil {
		ab.loops = map[string]*loopPlayer{}
	}
	p.SetVolume(vol * ab.sfxVol)
	ab.loops[key] = &loopPlayer{p: p, name: name, vol: vol}
	ab.mu.Unlock()
	p.Play()
}

// SetLoopVolume changes a running loop's volume.
func (ab *AudioBackend) SetLoopVolume(key string, vol float64) {
	ab.mu.Lock()
	defer ab.mu.Unlock()
	if l, ok := ab.loops[key]; ok {
		l.vol = vol
		l.p.SetVolume(vol * ab.sfxVol)
	}
}

// StopLoop stops the loop under key ("" = all loops).
func (ab *AudioBackend) StopLoop(key string) {
	ab.mu.Lock()
	defer ab.mu.Unlock()
	for k, l := range ab.loops {
		if key == "" || k == key {
			l.p.Close()
			delete(ab.loops, k)
		}
	}
}

// PlaySoundVolume plays a one-shot sound at volume vol (0..1 of sfx).
func (ab *AudioBackend) PlaySoundVolume(name string, vol float64) {
	pcm := ab.resolvePCM(name)
	if len(pcm) == 0 {
		return
	}
	p := ab.ctx.NewPlayer(bytesReader(pcmBytes(pcm)))
	ab.mu.Lock()
	p.SetVolume(vol * ab.sfxVol)
	ab.players = append(ab.players, p)
	ab.mu.Unlock()
	p.Play()
}

type loopPlayer struct {
	p    *oto.Player
	name string
	vol  float64
}

// --- AudioSystem wrappers (no-ops until the backend is up) ---

// PlayLoop starts a keyed looping sound (see AudioBackend.PlayLoop).
func (as *AudioSystem) PlayLoop(key, name string, vol float64) {
	if b := as.backend(); b != nil {
		b.PlayLoop(key, name, vol)
	}
}

// SetLoopVolume changes a running loop's volume.
func (as *AudioSystem) SetLoopVolume(key string, vol float64) {
	if b := as.backend(); b != nil {
		b.SetLoopVolume(key, vol)
	}
}

// StopLoop stops a keyed loop ("" = all).
func (as *AudioSystem) StopLoop(key string) {
	if b := as.backend(); b != nil {
		b.StopLoop(key)
	}
}

// PlaySoundVolume plays a one-shot sound at a volume (0..1).
func (as *AudioSystem) PlaySoundVolume(name string, vol float64) {
	if vol <= 0.01 {
		return
	}
	if b := as.backend(); b != nil {
		b.PlaySoundVolume(name, vol)
	}
}

// DistanceVolume is a simple falloff for positional sounds: full volume
// within near, fading to 0 at far.
func DistanceVolume(d, near, far float64) float64 {
	switch {
	case d <= near:
		return 1
	case d >= far:
		return 0
	}
	t := (far - d) / (far - near)
	return t * t
}
