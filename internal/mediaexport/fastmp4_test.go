package mediaexport

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestBuildConcatArgs(t *testing.T) {
	mp4WithTag := strings.Join(buildConcatArgs("list.txt", "out.mp4", false, false, true), " ")
	if !strings.Contains(mp4WithTag, "-tag:v hvc1") || !strings.Contains(mp4WithTag, "-c copy") {
		t.Fatalf("unexpected mp4WithTag args: %s", mp4WithTag)
	}
	mp4NoTagNoAud := strings.Join(buildConcatArgs("list.txt", "out.mp4", false, true, false), " ")
	if strings.Contains(mp4NoTagNoAud, "hvc1") {
		t.Fatalf("expected hvc1 omitted, got: %s", mp4NoTagNoAud)
	}
	if !strings.Contains(mp4NoTagNoAud, "-map 0:v:0 -c:v copy -an") {
		t.Fatalf("expected -an video copy args, got: %s", mp4NoTagNoAud)
	}
	mkv := strings.Join(buildConcatArgs("list.txt", "out.mkv", true, true, true), " ")
	if !strings.Contains(mkv, "-f matroska") || strings.Contains(mkv, "hvc1") {
		t.Fatalf("unexpected mkv args: %s", mkv)
	}
}

func TestRunConcatFallbacksHVC1Order(t *testing.T) {
	var attempts [][2]bool
	const h264Stderr = "Tag hvc1 incompatible with output codec id '27' (avc1)"
	_, err := runConcatFallbacks(context.Background(), false, h264Stderr, errors.New("exit 1"), func(dropAudio, useHVC1Tag bool) (string, error) {
		attempts = append(attempts, [2]bool{dropAudio, useHVC1Tag})
		if !dropAudio && !useHVC1Tag {
			return "", nil
		}
		return "fail", errors.New("fail")
	})
	if err != nil {
		t.Fatalf("expected H.264 fallback (false,false) to succeed, got %v", err)
	}
	if len(attempts) != 1 || attempts[0] != [2]bool{false, false} {
		t.Fatalf("expected immediate retry with dropAudio=false, useHVC1Tag=false; got %v", attempts)
	}
}

func TestBuildFragmentedMP4Args(t *testing.T) {
	a := strings.Join(BuildFragmentedMP4Args("rtsp://cam/1", 30, false, true), " ")
	if !strings.Contains(a, "-t 30") || !strings.Contains(a, "-tag:v hvc1") || strings.Contains(a, "-map 0:v:0") {
		t.Fatalf("unexpected tiandy-shaped args: %s", a)
	}
	b := strings.Join(BuildFragmentedMP4Args("rtsp://cam/1", 0, true, false), " ")
	if strings.Contains(b, "hvc1") || strings.Contains(b, " -t ") || !strings.Contains(b, "-map 0:v:0") {
		t.Fatalf("unexpected h264-fallback args: %s", b)
	}
}
