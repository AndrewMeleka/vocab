// Package tts wraps the system text-to-speech binaries that vocab shells out
// to (used by the spelling test and the story --spell flag).
package tts

import (
	"context"
	"errors"
	"os/exec"
)

// ErrNoTTS is returned by Speak when no system TTS binary is available.
var ErrNoTTS = errors.New("no TTS found — install espeak on Linux, say is bundled on macOS")

// Detect returns the first available system text-to-speech binary along with
// any prefix args it needs. ok is false when none are installed.
func Detect() (bin string, prefixArgs []string, ok bool) {
	if p, err := exec.LookPath("say"); err == nil {
		return p, nil, true
	}
	if p, err := exec.LookPath("espeak"); err == nil {
		return p, []string{"-s", "140"}, true
	}
	if p, err := exec.LookPath("spd-say"); err == nil {
		return p, nil, true
	}
	return "", nil, false
}

// Speak pronounces text using the first available system TTS binary. It
// returns ErrNoTTS if none is installed.
func Speak(ctx context.Context, text string) error {
	bin, prefix, ok := Detect()
	if !ok {
		return ErrNoTTS
	}
	args := append(append([]string{}, prefix...), text)
	return exec.CommandContext(ctx, bin, args...).Run()
}
