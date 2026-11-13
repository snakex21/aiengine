// renderer/shader.go — OpenGL shader compilation and program management.
// Provides helpers to compile vertex/fragment shaders, link programs,
// and retrieve uniform locations for the AIEngine renderer.

package renderer

import (
	"fmt"
	"strings"

	"github.com/go-gl/gl/v4.1-core/gl"
)

// ShaderProgram wraps an OpenGL shader program with cached uniform locations.
type ShaderProgram struct {
	ID       uint32
	uniforms map[string]int32
}

// NewShaderProgram compiles vertex and fragment shaders and links them into a program.
func NewShaderProgram(vertexSrc, fragmentSrc string) (*ShaderProgram, error) {
	vShader, err := compileShader(vertexSrc, gl.VERTEX_SHADER)
	if err != nil {
		return nil, fmt.Errorf("vertex shader: %w", err)
	}
	defer gl.DeleteShader(vShader)

	fShader, err := compileShader(fragmentSrc, gl.FRAGMENT_SHADER)
	if err != nil {
		return nil, fmt.Errorf("fragment shader: %w", err)
	}
	defer gl.DeleteShader(fShader)

	prog := gl.CreateProgram()
	gl.AttachShader(prog, vShader)
	gl.AttachShader(prog, fShader)
	gl.LinkProgram(prog)

	var status int32
	gl.GetProgramiv(prog, gl.LINK_STATUS, &status)
	if status == gl.FALSE {
		var logLen int32
		gl.GetProgramiv(prog, gl.INFO_LOG_LENGTH, &logLen)
		log := strings.Repeat("\x00", int(logLen+1))
		gl.GetProgramInfoLog(prog, logLen, nil, gl.Str(log))
		gl.DeleteProgram(prog)
		return nil, fmt.Errorf("link failed: %s", log)
	}

	return &ShaderProgram{ID: prog, uniforms: make(map[string]int32)}, nil
}

// Use activates the shader program.
func (sp *ShaderProgram) Use() {
	gl.UseProgram(sp.ID)
}

// UniformLocation returns the cached uniform location, querying OpenGL if needed.
func (sp *ShaderProgram) UniformLocation(name string) int32 {
	if loc, ok := sp.uniforms[name]; ok {
		return loc
	}
	loc := gl.GetUniformLocation(sp.ID, gl.Str(name+"\x00"))
	sp.uniforms[name] = loc
	return loc
}

// SetUniform1i sets an integer uniform.
func (sp *ShaderProgram) SetUniform1i(name string, v int32) {
	gl.Uniform1i(sp.UniformLocation(name), v)
}

// SetUniform1f sets a float uniform.
func (sp *ShaderProgram) SetUniform1f(name string, v float32) {
	gl.Uniform1f(sp.UniformLocation(name), v)
}

// SetUniform2f sets a vec2 uniform.
func (sp *ShaderProgram) SetUniform2f(name string, x, y float32) {
	gl.Uniform2f(sp.UniformLocation(name), x, y)
}

// SetUniform3f sets a vec3 uniform.
func (sp *ShaderProgram) SetUniform3f(name string, x, y, z float32) {
	gl.Uniform3f(sp.UniformLocation(name), x, y, z)
}

// SetUniform4f sets a vec4 uniform.
func (sp *ShaderProgram) SetUniform4f(name string, x, y, z, w float32) {
	gl.Uniform4f(sp.UniformLocation(name), x, y, z, w)
}

// SetUniformMat4 sets a mat4 uniform.
func (sp *ShaderProgram) SetUniformMat4(name string, mat *[16]float32) {
	gl.UniformMatrix4fv(sp.UniformLocation(name), 1, false, &mat[0])
}

// Delete releases the shader program.
func (sp *ShaderProgram) Delete() {
	gl.DeleteProgram(sp.ID)
}

// compileShader compiles a single shader of the given type.
func compileShader(source string, shaderType uint32) (uint32, error) {
	shader := gl.CreateShader(shaderType)

	csources, free := gl.Strs(source + "\x00")
	defer free()
	gl.ShaderSource(shader, 1, csources, nil)
	gl.CompileShader(shader)

	var status int32
	gl.GetShaderiv(shader, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {
		var logLen int32
		gl.GetShaderiv(shader, gl.INFO_LOG_LENGTH, &logLen)
		log := strings.Repeat("\x00", int(logLen+1))
		gl.GetShaderInfoLog(shader, logLen, nil, gl.Str(log))
		gl.DeleteShader(shader)
		return 0, fmt.Errorf("compile error: %s", log)
	}

	return shader, nil
}

// --- Standard shader sources used by the renderer ---

// wallVertexShader transforms wall column vertices.
const wallVertexShader = `#version 330 core
layout (location = 0) in vec2 aPos;
layout (location = 1) in vec2 aTexCoord;
out vec2 TexCoord;
uniform vec2 uScreenSize;
uniform float uColumnX;
uniform float uWallTop;
uniform float uWallBottom;
uniform float uTexX;

void main() {
    float px = (uColumnX / uScreenSize.x) * 2.0 - 1.0;
    // Screen Y: 0=top, height=bottom. NDC Y: -1=bottom, 1=top.
    // So we flip: ndcY = 1.0 - (screenY / screenH) * 2.0
    float screenY = (aPos.y < 0.5) ? uWallTop : uWallBottom;
    float py = 1.0 - (screenY / uScreenSize.y) * 2.0;
    gl_Position = vec4(px, py, 0.0, 1.0);
    TexCoord = vec2(uTexX, aTexCoord.y);
}
` + "\x00"

// wallFragmentShader samples the wall texture.
const wallFragmentShader = `#version 330 core
in vec2 TexCoord;
out vec4 FragColor;
uniform sampler2D uTexture;
uniform float uShade; // distance-based shading

void main() {
    vec4 texColor = texture(uTexture, TexCoord);
    FragColor = vec4(texColor.rgb * uShade, texColor.a);
}
` + "\x00"

// spriteVertexShader for billboard sprites.
const spriteVertexShader = `#version 330 core
layout (location = 0) in vec2 aPos;
layout (location = 1) in vec2 aTexCoord;
out vec2 TexCoord;
uniform vec2 uScreenSize;
uniform vec2 uSpritePos;   // screen-space center
uniform vec2 uSpriteSize;  // width, height
uniform float uTexX;
uniform int uUseFixedTexX;
uniform int uFlipY;

void main() {
    vec2 pos = uSpritePos + aPos * uSpriteSize;
    float px = (pos.x / uScreenSize.x) * 2.0 - 1.0;
    float py = 1.0 - (pos.y / uScreenSize.y) * 2.0; // flip screen Y to NDC
    gl_Position = vec4(px, py, 0.0, 1.0);
    float ty = uFlipY == 1 ? (1.0 - aTexCoord.y) : aTexCoord.y;
    TexCoord = vec2(uUseFixedTexX == 1 ? uTexX : aTexCoord.x, ty);
}
` + "\x00"

// spriteFragmentShader with alpha blending.
const spriteFragmentShader = `#version 330 core
in vec2 TexCoord;
out vec4 FragColor;
uniform sampler2D uTexture;
uniform float uAlpha;
uniform float uShade;

void main() {
    vec4 texColor = texture(uTexture, TexCoord);
    // Only skip fully transparent texels: a 0.1 cut-off chopped the soft
    // outer part of glows/shadows into hard-edged, truncated shapes.
    if (texColor.a < 0.004) discard;
    FragColor = vec4(texColor.rgb * uShade, texColor.a * uAlpha);
}
` + "\x00"

// postProcessVertexShader renders a fullscreen quad.
const postProcessVertexShader = `#version 330 core
layout (location = 0) in vec2 aPos;
layout (location = 1) in vec2 aTexCoord;
out vec2 TexCoord;

void main() {
    gl_Position = vec4(aPos, 0.0, 1.0);
    TexCoord = aTexCoord;
}
` + "\x00"

// postProcessFragmentShader applies screen effects.
const postProcessFragmentShader = `#version 330 core
in vec2 TexCoord;
out vec4 FragColor;
uniform sampler2D uScreenTexture;
uniform float uVignette;
uniform float uBlurAmount;
uniform float uFilmGrain;
uniform float uChromaticAberration;
uniform float uRedTint;
uniform float uBlueTint;
uniform float uDesaturate;
uniform float uContrast;
uniform float uSaturation;
uniform float uBrightness;
uniform float uTime;
uniform vec2 uTexelSize;

float random(vec2 uv) {
    return fract(sin(dot(uv, vec2(12.9898, 78.233))) * 43758.5453);
}

vec3 applyContrastSaturationBrightness(vec3 color) {
    // Contrast
    color = (color - 0.5) * uContrast + 0.5;
    // Saturation
    float gray = dot(color, vec3(0.299, 0.587, 0.114));
    color = mix(vec3(gray), color, uSaturation);
    // Brightness
    color *= uBrightness;
    return color;
}

void main() {
    vec2 uv = TexCoord;
    vec3 color;

    // Chromatic aberration
    if (uChromaticAberration > 0.0) {
        float offset = uChromaticAberration * 0.01;
        float r = texture(uScreenTexture, uv + vec2(offset, 0.0)).r;
        float g = texture(uScreenTexture, uv).g;
        float b = texture(uScreenTexture, uv - vec2(offset, 0.0)).b;
        color = vec3(r, g, b);
    } else {
        color = texture(uScreenTexture, uv).rgb;
    }

    // Vignette
    if (uVignette > 0.0) {
        vec2 dist = uv - 0.5;
        float vignette = 1.0 - dot(dist, dist) * uVignette * 2.0;
        color *= max(vignette, 0.0);
    }

    // Film grain
    if (uFilmGrain > 0.0) {
        float grain = random(uv * uTime) * uFilmGrain;
        color += grain - uFilmGrain * 0.5;
    }

    // Simple blur (box blur approximation)
    if (uBlurAmount > 0.0) {
        vec3 blurColor = vec3(0.0);
        float samples = 0.0;
        for (int x = -2; x <= 2; x++) {
            for (int y = -2; y <= 2; y++) {
                vec2 offset = vec2(float(x), float(y)) * uTexelSize * uBlurAmount * 5.0;
                blurColor += texture(uScreenTexture, uv + offset).rgb;
                samples += 1.0;
            }
        }
        color = mix(color, blurColor / samples, uBlurAmount);
    }

    // Color tints
    if (uRedTint > 0.0) {
        color = mix(color, color * vec3(1.5, 0.5, 0.5), uRedTint);
    }
    if (uBlueTint > 0.0) {
        color = mix(color, color * vec3(0.5, 0.5, 1.5), uBlueTint);
    }

    // Desaturate
    if (uDesaturate > 0.0) {
        float gray = dot(color, vec3(0.299, 0.587, 0.114));
        color = mix(color, vec3(gray), uDesaturate);
    }

    // Contrast, saturation, brightness
    color = applyContrastSaturationBrightness(color);

    FragColor = vec4(clamp(color, 0.0, 1.0), 1.0);
}
` + "\x00"
