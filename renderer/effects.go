// renderer/effects.go — Post-processing effects configuration and management.
// Provides the EffectsConfig struct used to control screen-space effects
// (vignette, blur, chromatic aberration, film grain, color grading).
// Effects are applied by the raycaster's post-process shader.

package renderer

// EffectsConfig holds all post-processing effect parameters.
// Values range from 0.0 (off) to 1.0 (full), except Contrast/Saturation/Brightness
// which use 1.0 as the neutral value.
type EffectsConfig struct {
	Vignette            float64 // 0.0–1.0, darkens screen edges
	BlurAmount          float64 // 0.0–1.0, box blur intensity
	FilmGrain           float64 // 0.0–1.0, noise overlay
	ChromaticAberration float64 // 0.0–1.0, RGB shift at edges
	RedTint             float64 // 0.0–1.0, red color tint (damage effect)
	BlueTint            float64 // 0.0–1.0, blue color tint (underwater)
	Desaturate          float64 // 0.0–1.0, desaturation amount
	Contrast            float64 // neutral = 1.0, higher = more contrast
	Saturation          float64 // neutral = 1.0, 0.0 = grayscale
	Brightness          float64 // neutral = 1.0
}

// DefaultEffects returns a neutral effects configuration (no effects active).
func DefaultEffects() *EffectsConfig {
	return &EffectsConfig{
		Contrast:   1.0,
		Saturation: 1.0,
		Brightness: 1.0,
	}
}

// NoireEffects returns a noir-style preset (desaturated, high contrast, vignette).
func NoireEffects() *EffectsConfig {
	return &EffectsConfig{
		Vignette:   0.4,
		FilmGrain:  0.08,
		Desaturate: 0.7,
		Contrast:   1.2,
		Saturation: 0.5,
		Brightness: 0.9,
	}
}

// DamageEffects returns a red-flash damage indicator preset.
func DamageEffects(amount float64) *EffectsConfig {
	return &EffectsConfig{
		RedTint:    amount,
		Vignette:   amount * 0.5,
		Contrast:   1.0 + amount*0.3,
		Saturation: 1.0,
		Brightness: 1.0,
	}
}

// LowHPEffects returns a critical-health preset (heavy vignette, red tint, pulse-ready).
func LowHPEffects() *EffectsConfig {
	return &EffectsConfig{
		Vignette:   0.8,
		RedTint:    0.3,
		Desaturate: 0.2,
		Contrast:   1.1,
		Saturation: 0.8,
		Brightness: 0.85,
	}
}

// DeathEffects returns a death-screen preset (full desaturation, heavy blur, darken).
func DeathEffects() *EffectsConfig {
	return &EffectsConfig{
		Desaturate: 1.0,
		BlurAmount: 0.8,
		Vignette:   1.0,
		Brightness: 0.3,
		Contrast:   0.8,
		Saturation: 0.0,
	}
}

// UnderwaterEffects returns an underwater preset (blue tint, blur, distortion).
func UnderwaterEffects() *EffectsConfig {
	return &EffectsConfig{
		BlueTint:   0.4,
		BlurAmount: 0.3,
		Vignette:   0.5,
		Contrast:   0.9,
		Saturation: 0.7,
		Brightness: 0.8,
	}
}

// BulletTimeEffects returns a slow-motion preset (chromatic aberration, saturation boost).
func BulletTimeEffects() *EffectsConfig {
	return &EffectsConfig{
		ChromaticAberration: 0.8,
		Vignette:            0.6,
		Saturation:          1.3,
		Contrast:            1.15,
		Brightness:          1.05,
	}
}

// WarmEffects returns a warm color grading preset.
func WarmEffects() *EffectsConfig {
	return &EffectsConfig{
		RedTint:    0.1,
		Vignette:   0.2,
		Contrast:   1.05,
		Saturation: 1.1,
		Brightness: 1.02,
	}
}

// ColdEffects returns a cold color grading preset.
func ColdEffects() *EffectsConfig {
	return &EffectsConfig{
		BlueTint:   0.1,
		Vignette:   0.2,
		Contrast:   1.05,
		Saturation: 0.9,
		Brightness: 1.02,
	}
}

// SepiaEffects returns a sepia color grading preset.
func SepiaEffects() *EffectsConfig {
	return &EffectsConfig{
		Desaturate: 0.5,
		RedTint:    0.15,
		Vignette:   0.3,
		Contrast:   1.05,
		Saturation: 0.7,
		Brightness: 1.0,
	}
}

// SetFromPlayerState configures effects based on a player state string.
// Supported states: "normal", "low-hp", "dead", "underwater", "bullet-time".
func (cfg *EffectsConfig) SetFromPlayerState(state string) {
	switch state {
	case "normal":
		*cfg = *DefaultEffects()
	case "low-hp":
		*cfg = *LowHPEffects()
	case "dead":
		*cfg = *DeathEffects()
	case "underwater":
		*cfg = *UnderwaterEffects()
	case "bullet-time":
		*cfg = *BulletTimeEffects()
	}
}

// SetFromCSSProps applies CSS property values to the effects config.
// Used to bridge the CSS parser output into the renderer.
func (cfg *EffectsConfig) SetFromCSSProps(props map[string]string) {
	if v := props["vignette"]; v != "" {
		cfg.Vignette = parseFloat(v)
	}
	if v := props["film-grain"]; v != "" {
		cfg.FilmGrain = parseFloat(v)
	}
	if v := props["blur"]; v != "" {
		cfg.BlurAmount = parseFloat(v)
	}
	if v := props["chromatic-aberration"]; v != "" {
		cfg.ChromaticAberration = parseFloat(v)
	}
	if v := props["red-tint"]; v != "" {
		cfg.RedTint = parseFloat(v)
	}
	if v := props["blue-tint"]; v != "" {
		cfg.BlueTint = parseFloat(v)
	}
	if v := props["desaturate"]; v != "" {
		cfg.Desaturate = parseFloat(v)
	}
	if v := props["contrast"]; v != "" {
		cfg.Contrast = parseFloat(v)
	}
	if v := props["saturation"]; v != "" {
		cfg.Saturation = parseFloat(v)
	}
	if v := props["brightness"]; v != "" {
		cfg.Brightness = parseFloat(v)
	}
}

func parseFloat(s string) float64 {
	if s == "" {
		return 0
	}
	// Simple string-to-float conversion.
	var result float64
	var frac float64 = 1.0
	decimal := false
	sign := 1.0

	for i, ch := range s {
		if i == 0 && ch == '-' {
			sign = -1.0
			continue
		}
		if ch == '.' {
			decimal = true
			continue
		}
		if ch < '0' || ch > '9' {
			continue
		}
		digit := float64(ch - '0')
		if decimal {
			frac *= 0.1
			result += digit * frac
		} else {
			result = result*10 + digit
		}
	}
	return result * sign
}
