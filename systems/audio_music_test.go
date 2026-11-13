package systems

import "testing"

func TestLoopReaderRepeats(t *testing.T) {
	r := &LoopReader{Data: []byte{1, 2, 3}}
	buf := make([]byte, 7)
	n, err := r.Read(buf)
	if err != nil || n != 7 || string(buf) != string([]byte{1, 2, 3, 1, 2, 3, 1}) {
		t.Fatalf("n=%d err=%v buf=%v", n, err, buf)
	}
	if _, err := (&LoopReader{}).Read(buf); err == nil {
		t.Fatal("empty loop should report EOF")
	}
}

func TestSetVolumesWithoutBackend(t *testing.T) {
	as := NewAudioSystem()
	as.SetVolumes(0.5, 0.4, 0.8) // backend not started: stored, no panic
	if as.MasterVol != 0.5 || as.musicLevel != 0.2 || as.sfxLevel != 0.4 {
		t.Fatalf("levels: master %v music %v sfx %v", as.MasterVol, as.musicLevel, as.sfxLevel)
	}
	as.PlayMusicTrack("ambient")
	if as.wantMusic != "ambient" {
		t.Fatal("music request not remembered for when the backend starts")
	}
}
