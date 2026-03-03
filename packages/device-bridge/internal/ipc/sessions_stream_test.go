package ipc

import (
	"context"
	"errors"
	"testing"

	"github.com/anthropics/yepanywhere/device-bridge/internal/device"
)

type streamTestDevice struct {
	startErr         error
	startCalls       int
	stopCalls        int
	startOpts        device.StreamOptions
	source           *device.NalSource
	newSourceOnStart bool
}

func (d *streamTestDevice) GetFrame(context.Context, int) (*device.Frame, error) { return nil, nil }
func (d *streamTestDevice) SendTouch(context.Context, []device.TouchPoint) error { return nil }
func (d *streamTestDevice) SendKey(context.Context, string) error                { return nil }
func (d *streamTestDevice) ScreenSize() (int32, int32)                           { return 1080, 2400 }
func (d *streamTestDevice) Close() error                                         { return nil }

func (d *streamTestDevice) StartStream(_ context.Context, opts device.StreamOptions) (*device.NalSource, error) {
	d.startCalls++
	d.startOpts = opts
	if d.startErr != nil {
		return nil, d.startErr
	}
	if d.source == nil || d.newSourceOnStart {
		d.source = device.NewNalSource()
	}
	return d.source, nil
}

func (d *streamTestDevice) StopStream(context.Context) error {
	d.stopCalls++
	return nil
}
func (d *streamTestDevice) SetStreamBitrate(context.Context, int) error { return nil }
func (d *streamTestDevice) RequestStreamKeyframe(context.Context) error { return nil }

type nonStreamTestDevice struct{}

func (d *nonStreamTestDevice) GetFrame(context.Context, int) (*device.Frame, error) { return nil, nil }
func (d *nonStreamTestDevice) SendTouch(context.Context, []device.TouchPoint) error { return nil }
func (d *nonStreamTestDevice) SendKey(context.Context, string) error                { return nil }
func (d *nonStreamTestDevice) ScreenSize() (int32, int32)                           { return 1080, 2400 }
func (d *nonStreamTestDevice) Close() error                                         { return nil }

func TestMaybeStartAndroidStreamStartsWhenSupported(t *testing.T) {
	dev := &streamTestDevice{source: device.NewNalSource()}

	nalSource, streamCap, err := maybeStartAndroidStream(dev, "android", 1280, 720, 30)
	if err != nil {
		t.Fatalf("maybeStartAndroidStream returned error: %v", err)
	}
	if nalSource == nil {
		t.Fatal("expected NAL source when stream is supported")
	}
	if streamCap == nil {
		t.Fatal("expected StreamCapable handle")
	}
	if dev.startCalls != 1 {
		t.Fatalf("expected StartStream to be called once, got %d", dev.startCalls)
	}
	if dev.startOpts.Width != 1280 || dev.startOpts.Height != 720 || dev.startOpts.FPS != 30 {
		t.Fatalf("unexpected StartStream opts: %+v", dev.startOpts)
	}
	if dev.startOpts.BitrateBps != 2_000_000 {
		t.Fatalf("unexpected bitrate: got %d", dev.startOpts.BitrateBps)
	}
}

func TestMaybeStartAndroidStreamFallsBackWhenUnsupported(t *testing.T) {
	dev := &nonStreamTestDevice{}

	nalSource, streamCap, err := maybeStartAndroidStream(dev, "android", 720, 1280, 30)
	if err != nil {
		t.Fatalf("expected nil error for unsupported stream path, got %v", err)
	}
	if nalSource != nil || streamCap != nil {
		t.Fatalf("expected nil stream results for unsupported path: nal=%v streamCap=%v", nalSource, streamCap)
	}
}

func TestMaybeStartAndroidStreamReturnsErrorForFallback(t *testing.T) {
	dev := &streamTestDevice{startErr: errors.New("legacy server")}

	nalSource, streamCap, err := maybeStartAndroidStream(dev, "android", 720, 1280, 30)
	if err == nil {
		t.Fatal("expected start error")
	}
	if nalSource != nil || streamCap != nil {
		t.Fatalf("expected nil results on stream_start failure: nal=%v streamCap=%v", nalSource, streamCap)
	}
	if dev.startCalls != 1 {
		t.Fatalf("expected StartStream to be called once, got %d", dev.startCalls)
	}
}

func TestClampBitrateDown(t *testing.T) {
	if got := clampBitrateDown(2_000_000, 500_000); got != 1_500_000 {
		t.Fatalf("expected 1,500,000, got %d", got)
	}
	if got := clampBitrateDown(550_000, 500_000); got != 500_000 {
		t.Fatalf("expected clamp to min 500,000, got %d", got)
	}
	if got := clampBitrateDown(500_000, 500_000); got != 500_000 {
		t.Fatalf("expected min to stay unchanged, got %d", got)
	}
}

func TestClampBitrateUp(t *testing.T) {
	if got := clampBitrateUp(1_000_000, 2_000_000); got != 1_250_000 {
		t.Fatalf("expected 1,250,000, got %d", got)
	}
	if got := clampBitrateUp(1_900_000, 2_000_000); got != 2_000_000 {
		t.Fatalf("expected clamp to max 2,000,000, got %d", got)
	}
	if got := clampBitrateUp(2_000_000, 2_000_000); got != 2_000_000 {
		t.Fatalf("expected max to stay unchanged, got %d", got)
	}
}

func TestBuildAdaptiveProfilesMonotonic(t *testing.T) {
	profiles := buildAdaptiveProfiles(1280, 720, 30)
	if len(profiles) < 2 {
		t.Fatalf("expected multiple profiles, got %d", len(profiles))
	}
	if profiles[0].Width != 1280 || profiles[0].Height != 720 || profiles[0].FPS != 30 {
		t.Fatalf("unexpected base profile: %+v", profiles[0])
	}
	for i := 1; i < len(profiles); i++ {
		prev := profiles[i-1]
		cur := profiles[i]
		if cur.Width > prev.Width || cur.Height > prev.Height || cur.FPS > prev.FPS {
			t.Fatalf("profiles must not increase at step %d: prev=%+v cur=%+v", i, prev, cur)
		}
		if cur.Width%2 != 0 || cur.Height%2 != 0 {
			t.Fatalf("dimensions must be even: %+v", cur)
		}
		if cur.BitrateBps < 500_000 {
			t.Fatalf("bitrate floor violated: %+v", cur)
		}
	}
}

func TestRestartNALStreamUpdatesSessionProfile(t *testing.T) {
	dev := &streamTestDevice{newSourceOnStart: true}
	initialSource := device.NewNalSource()
	dev.source = initialSource
	sess := &streamSession{
		sessionID: "s1",
		nalSource: initialSource,
		streamCap: dev,
		targetW:   1280,
		targetH:   720,
		maxFPS:    30,
	}
	sm := &SessionManager{}

	nextOpts := device.StreamOptions{
		Width:      960,
		Height:     540,
		FPS:        24,
		BitrateBps: 1_200_000,
	}

	newSource, err := sm.restartNALStream(sess, nextOpts)
	if err != nil {
		t.Fatalf("restartNALStream: %v", err)
	}
	if newSource == nil {
		t.Fatal("expected new source")
	}
	if newSource == initialSource {
		t.Fatal("expected source replacement on restart")
	}
	if sess.nalSource != newSource {
		t.Fatal("session source was not updated")
	}
	if dev.stopCalls != 1 || dev.startCalls != 1 {
		t.Fatalf("expected 1 stop/1 start call, got stop=%d start=%d", dev.stopCalls, dev.startCalls)
	}
	if dev.startOpts != nextOpts {
		t.Fatalf("unexpected restart opts: %+v", dev.startOpts)
	}
	if sess.targetW != nextOpts.Width || sess.targetH != nextOpts.Height || sess.maxFPS != nextOpts.FPS {
		t.Fatalf("session profile not updated: target=%dx%d fps=%d", sess.targetW, sess.targetH, sess.maxFPS)
	}
}
