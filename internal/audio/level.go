package audio

import (
	"math"
	"time"
)

const (
	levelThrottleMs = 25

	// Auto-range: recentMax decay per emit (~25 emits/s)
	// 0.993^25 ≈ 0.84 per second, halves in ~4s
	levelDecay      = 0.993
	levelNoiseFloor = 0.01 // absolute noise floor (below = silence)
	levelSmoothing  = 0.4  // EMA alpha (higher = more responsive)
)

// LevelProcessor computes normalized audio level [0,1] for visualization.
// Auto-scales to microphone sensitivity by tracking recent peak maximum.
type LevelProcessor struct {
	recentMax float64
	smoothed  float64
	peakSince float64 // max peak between emits
	lastEmit  time.Time
	LevelChan chan float64
}

func NewLevelProcessor() *LevelProcessor {
	return &LevelProcessor{
		recentMax: 0.01, // reasonable starting estimate
		LevelChan: make(chan float64, 16),
	}
}

// Process takes a PCM16 mono buffer and emits normalized level to LevelChan.
func (lp *LevelProcessor) Process(pcm []byte) {
	if len(pcm) < 2 {
		return
	}

	// Peak detection - accumulate max between emits
	sampleCount := len(pcm) / 2
	var maxSample float64
	for i := 0; i < sampleCount; i++ {
		s := int16(pcm[2*i]) | int16(pcm[2*i+1])<<8
		v := math.Abs(float64(s))
		if v > maxSample {
			maxSample = v
		}
	}
	peak := maxSample / 32768.0

	if peak > lp.peakSince {
		lp.peakSince = peak
	}

	// Throttle - only emit at interval
	now := time.Now()
	if now.Sub(lp.lastEmit) < levelThrottleMs*time.Millisecond {
		return
	}
	lp.lastEmit = now

	// Use accumulated peak and reset
	peak = lp.peakSince
	lp.peakSince = 0

	// Auto-range: track recent maximum
	if peak > lp.recentMax {
		lp.recentMax = peak // instant attack
	} else {
		lp.recentMax *= levelDecay // slow release
	}
	if lp.recentMax < levelNoiseFloor {
		lp.recentMax = levelNoiseFloor
	}

	// Normalize to 0-1
	level := 0.0
	if peak > levelNoiseFloor {
		level = peak / lp.recentMax
		if level > 1.0 {
			level = 1.0
		}
	}

	// EMA smoothing
	lp.smoothed = levelSmoothing*level + (1-levelSmoothing)*lp.smoothed

	select {
	case lp.LevelChan <- lp.smoothed:
	default:
	}
}
