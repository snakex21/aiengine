// systems/audio_backend.go — Audio playback backend using oto/v3.
// Each sound gets its own Player — no mixer, no ring buffer, no goroutines.
// Three sound sources: file cache, procedural cache, live procedural.

package systems

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	oto "github.com/ebitengine/oto/v3"
	mp3 "github.com/hajimehoshi/go-mp3"
	"github.com/jfreymuth/oggvorbis"
)

// SoundCache stores pre-generated/decoded PCM data keyed by sound name.
type SoundCache struct {
	mu     sync.RWMutex
	sounds map[string][]int16 // int16 PCM samples
}

func newSoundCache() *SoundCache {
	return &SoundCache{sounds: make(map[string][]int16)}
}

func (sc *SoundCache) Get(name string) ([]int16, bool) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	s, ok := sc.sounds[name]
	return s, ok
}

func (sc *SoundCache) Set(name string, samples []int16) {
	sc.mu.Lock()
	sc.sounds[name] = make([]int16, len(samples))
	copy(sc.sounds[name], samples)
	sc.mu.Unlock()
}

func (sc *SoundCache) Has(name string) bool {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	_, ok := sc.sounds[name]
	return ok
}

// AudioBackend manages oto context and sound playback.
type AudioBackend struct {
	mu sync.Mutex

	sampleRate int
	ctx        *oto.Context
	cache      *SoundCache
	soundDefs  map[string]SoundDef
	soundPaths []string

	// Active players (for cleanup).
	players []*oto.Player

	// Volumes (0..1): effective sound-effect and music gain.
	sfxVol   float64
	musicVol float64
	music    *oto.Player
	musicKey string
	loops    map[string]*loopPlayer // keyed looping effects (audio_loops.go)

	running bool
	quit    chan struct{}
}

// NewAudioBackend creates the audio backend, loads cached sounds, and starts.
func NewAudioBackend(sampleRate int, soundPaths []string) (*AudioBackend, error) {
	ctx, ready, err := oto.NewContext(&oto.NewContextOptions{
		SampleRate:   sampleRate,
		ChannelCount: 1,
		Format:       oto.FormatSignedInt16LE,
	})
	if err != nil {
		return nil, fmt.Errorf("oto context: %w", err)
	}

	select {
	case <-ready:
	case <-time.After(3 * time.Second):
		return nil, fmt.Errorf("audio device timeout")
	}

	ab := &AudioBackend{
		sampleRate: sampleRate,
		ctx:        ctx,
		cache:      newSoundCache(),
		soundDefs:  make(map[string]SoundDef),
		soundPaths: soundPaths,
		running:    true,
		quit:       make(chan struct{}),
		sfxVol:     1,
		musicVol:   1,
	}

	// Background goroutine to clean up finished players.
	go ab.reaper()

	log.Printf("[Audio] Backend ready: %dHz mono 16bit", sampleRate)
	return ab, nil
}

// reaper periodically closes finished players.
func (ab *AudioBackend) reaper() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for { // stops when quit is closed (reading ab.running here raced with Shutdown)
		select {
		case <-ab.quit:
			return
		case <-ticker.C:
			ab.mu.Lock()
			var alive []*oto.Player
			for _, p := range ab.players {
				if p.IsPlaying() {
					alive = append(alive, p)
				} else {
					p.Close()
				}
			}
			ab.players = alive
			ab.mu.Unlock()
		}
	}
}

// --- Sound Source Registration ---

// CacheFile loads an audio file, decodes it, and stores it in the cache.
func (ab *AudioBackend) CacheFile(name, path string) error {
	samples, _, err := LoadAudioFile(path)
	if err != nil {
		return err
	}
	// Convert float32 → int16.
	pcm := float32ToInt16(samples)
	ab.cache.Set(name, pcm)
	log.Printf("[Audio] Cached file: %s (%d samples)", name, len(pcm))
	return nil
}

// CacheProcedural generates a procedural sound and caches it.
func (ab *AudioBackend) CacheProcedural(name string, sd SoundDef) {
	ab.soundDefs[name] = sd
	samples := sd.GenerateSamples(ab.sampleRate)
	pcm := float32ToInt16(samples)
	ab.cache.Set(name, pcm)
	log.Printf("[Audio] Cached procedural: %s (%d samples)", name, len(pcm))
}

// RegisterLive registers a procedural sound for live generation (not cached).
func (ab *AudioBackend) RegisterLive(name string, sd SoundDef) {
	ab.soundDefs[name] = sd
}

// --- Playback ---

// PlaySound plays a sound by name. Resolution order:
//  1. Cache hit → play immediately
//  2. File exists on disk → load, cache, play
//  3. Procedural def (cached or live) → generate, play
func (ab *AudioBackend) PlaySound(name string) {
	// 1. Check cache.
	if data, ok := ab.cache.Get(name); ok {
		ab.playPCM(data)
		return
	}

	// 2. Check file on disk.
	if IsAudioFile(name) {
		path := ab.findFile(name)
		if path != "" {
			samples, _, err := LoadAudioFile(path)
			if err == nil {
				pcm := float32ToInt16(samples)
				ab.cache.Set(name, pcm)
				ab.playPCM(pcm)
				return
			}
		}
	}

	// 3. Try procedural definition (live generation).
	if sd, ok := ab.soundDefs[name]; ok {
		samples := sd.GenerateSamples(ab.sampleRate)
		pcm := float32ToInt16(samples)
		ab.playPCM(pcm)
		return
	}

	// Try stripping extension.
	base := strings.TrimSuffix(name, filepath.Ext(name))
	if sd, ok := ab.soundDefs[base]; ok {
		samples := sd.GenerateSamples(ab.sampleRate)
		ab.playPCM(float32ToInt16(samples))
		return
	}

	log.Printf("[Audio] Sound not found: %s", name)
}

// playPCM creates a player for PCM data and starts playback.
func (ab *AudioBackend) playPCM(pcm []int16) {
	if len(pcm) == 0 {
		return
	}

	// Convert int16 to byte slice.
	data := make([]byte, len(pcm)*2)
	for i, v := range pcm {
		binary.LittleEndian.PutUint16(data[i*2:], uint16(v))
	}

	reader := bytes.NewReader(data)
	player := ab.ctx.NewPlayer(reader)
	ab.mu.Lock()
	player.SetVolume(ab.sfxVol)
	ab.players = append(ab.players, player)
	ab.mu.Unlock()
	player.Play()
}

// resolvePCM finds samples for name: cache, audio file, or procedural def.
func (ab *AudioBackend) resolvePCM(name string) []int16 {
	if data, ok := ab.cache.Get(name); ok {
		return data
	}
	if IsAudioFile(name) {
		if path := ab.findFile(name); path != "" {
			if samples, _, err := LoadAudioFile(path); err == nil {
				pcm := float32ToInt16(samples)
				ab.cache.Set(name, pcm)
				return pcm
			}
		}
	}
	if sd, ok := ab.soundDefs[name]; ok {
		pcm := float32ToInt16(sd.GenerateSamples(ab.sampleRate))
		ab.cache.Set(name, pcm)
		return pcm
	}
	return nil
}

// PlayMusic loops a sound (procedural def or audio file) as background music,
// replacing the current track. Playing the same track again is a no-op.
func (ab *AudioBackend) PlayMusic(name string) {
	ab.mu.Lock()
	same := ab.music != nil && ab.musicKey == name
	ab.mu.Unlock()
	if same {
		return
	}
	pcm := ab.resolvePCM(name)
	if len(pcm) == 0 {
		log.Printf("[Audio] Music not found: %s", name)
		return
	}
	data := make([]byte, len(pcm)*2)
	for i, v := range pcm {
		binary.LittleEndian.PutUint16(data[i*2:], uint16(v))
	}
	player := ab.ctx.NewPlayer(&LoopReader{Data: data})
	ab.StopMusic()
	ab.mu.Lock()
	player.SetVolume(ab.musicVol)
	ab.music, ab.musicKey = player, name
	ab.mu.Unlock()
	player.Play()
}

// StopMusic stops the background track.
func (ab *AudioBackend) StopMusic() {
	ab.mu.Lock()
	defer ab.mu.Unlock()
	if ab.music != nil {
		ab.music.Close()
		ab.music, ab.musicKey = nil, ""
	}
}

// SetVolumes sets the effective sfx and music gains (0..1). Music changes
// apply immediately; sfx applies to sounds started afterwards.
func (ab *AudioBackend) SetVolumes(sfx, music float64) {
	ab.mu.Lock()
	defer ab.mu.Unlock()
	ab.sfxVol, ab.musicVol = sfx, music
	if ab.music != nil {
		ab.music.SetVolume(music)
	}
}

func bytesReader(b []byte) io.Reader { return bytes.NewReader(b) }

// LoopReader endlessly repeats Data (for looping music players).
type LoopReader struct {
	Data []byte
	pos  int
}

func (r *LoopReader) Read(p []byte) (int, error) {
	if len(r.Data) == 0 {
		return 0, io.EOF
	}
	n := 0
	for n < len(p) {
		c := copy(p[n:], r.Data[r.pos:])
		n += c
		r.pos = (r.pos + c) % len(r.Data)
	}
	return n, nil
}

// findFile searches sound paths for the given filename.
func (ab *AudioBackend) findFile(filename string) string {
	if _, err := os.Stat(filename); err == nil {
		return filename
	}
	for _, dir := range ab.soundPaths {
		path := filepath.Join(dir, filename)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

// StopAll closes all active players.
func (ab *AudioBackend) StopAll() {
	ab.mu.Lock()
	defer ab.mu.Unlock()
	for _, p := range ab.players {
		p.Close()
	}
	ab.players = nil
}

// Shutdown stops everything and releases the audio context.
func (ab *AudioBackend) Shutdown() {
	ab.running = false
	close(ab.quit)
	ab.StopMusic()
	ab.StopLoop("")
	ab.StopAll()
	log.Println("[Audio] Backend shut down")
}

// --- Helpers ---

func float32ToInt16(samples []float32) []int16 {
	pcm := make([]int16, len(samples))
	for i, s := range samples {
		if s > 1.0 {
			s = 1.0
		} else if s < -1.0 {
			s = -1.0
		}
		pcm[i] = int16(s * 32767)
	}
	return pcm
}

// --- File Decoding ---

func IsAudioFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".wav", ".mp3", ".ogg", ".flac":
		return true
	}
	return false
}

func LoadAudioFile(path string) ([]float32, int, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".wav":
		return decodeWAV(path)
	case ".mp3":
		return decodeMP3(path)
	case ".ogg":
		return decodeOGG(path)
	default:
		return nil, 0, fmt.Errorf("unsupported format: %s", ext)
	}
}

// --- WAV Decoder ---

func decodeWAV(path string) ([]float32, int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, err
	}
	if len(data) < 44 || string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return nil, 0, fmt.Errorf("invalid WAV")
	}
	var sampleRate, numChannels, bitsPerSample int
	var dataOffset, dataSize int
	offset := 12
	for offset < len(data)-8 {
		id := string(data[offset : offset+4])
		size := int(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		switch id {
		case "fmt ":
			audioFmt := binary.LittleEndian.Uint16(data[offset+8 : offset+10])
			if audioFmt != 1 {
				return nil, 0, fmt.Errorf("only PCM WAV supported")
			}
			numChannels = int(binary.LittleEndian.Uint16(data[offset+10 : offset+12]))
			sampleRate = int(binary.LittleEndian.Uint32(data[offset+12 : offset+16]))
			bitsPerSample = int(binary.LittleEndian.Uint16(data[offset+22 : offset+24]))
		case "data":
			dataOffset = offset + 8
			dataSize = size
		}
		offset += 8 + size
	}
	if dataOffset == 0 {
		return nil, 0, fmt.Errorf("no data chunk")
	}
	sampleData := data[dataOffset : dataOffset+dataSize]
	var samples []float32
	switch bitsPerSample {
	case 8:
		samples = make([]float32, len(sampleData))
		for i, b := range sampleData {
			samples[i] = float32(int(b)-128) / 128.0
		}
	case 16:
		pairs := len(sampleData) / 2
		samples = make([]float32, 0, pairs/numChannels)
		for i := 0; i < pairs; i += numChannels {
			v := int16(binary.LittleEndian.Uint16(sampleData[i*2:]))
			samples = append(samples, float32(v)/32768.0)
		}
	}
	return samples, sampleRate, nil
}

// --- MP3 Decoder ---

func decodeMP3(path string) ([]float32, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	dec, err := mp3.NewDecoder(f)
	if err != nil {
		return nil, 0, err
	}
	var samples []float32
	buf := make([]byte, 4096)
	for {
		n, err := dec.Read(buf)
		if n > 0 {
			pairs := n / 2
			for i := 0; i < pairs; i++ {
				v := int16(binary.LittleEndian.Uint16(buf[i*2:]))
				samples = append(samples, float32(v)/32768.0)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, 0, err
		}
	}
	return samples, dec.SampleRate(), nil
}

// --- OGG Vorbis Decoder ---

func decodeOGG(path string) ([]float32, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	dec, err := oggvorbis.NewReader(f)
	if err != nil {
		return nil, 0, err
	}
	var samples []float32
	buf := make([]float32, 4096)
	for {
		n, err := dec.Read(buf)
		if n > 0 {
			samples = append(samples, buf[:n]...)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, 0, err
		}
	}
	return samples, dec.SampleRate(), nil
}
