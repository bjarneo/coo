package model

import "testing"

func TestAppendKeepsScrollAnchored(t *testing.T) {
	b := &Buffer{}
	for i := 0; i < 20; i++ {
		b.Append(Line{Text: "msg"})
	}
	b.ScrollOff = 5 // user scrolled up 5 rendered rows

	b.Append(Line{Text: "new line arrives"})

	if b.ScrollOff != 6 {
		t.Fatalf("expected ScrollOff to bump by 1 (anchor preserved); got %d", b.ScrollOff)
	}
}

func TestAppendAtBottomDoesNotInflateScroll(t *testing.T) {
	b := &Buffer{}
	b.Append(Line{Text: "first"})
	if b.ScrollOff != 0 {
		t.Fatalf("ScrollOff should stay 0 when at bottom; got %d", b.ScrollOff)
	}
}

func TestAppendRingCapPreservesScroll(t *testing.T) {
	// Fill past the cap and verify ScrollOff doesn't go negative or unbounded.
	b := &Buffer{}
	for i := 0; i < bufferCap+50; i++ {
		b.Append(Line{Text: "x"})
	}
	if len(b.Lines) != bufferCap {
		t.Fatalf("expected ring cap %d, got %d", bufferCap, len(b.Lines))
	}
	b.ScrollOff = 10
	for i := 0; i < 5; i++ {
		b.Append(Line{Text: "y"})
	}
	if b.ScrollOff < 0 {
		t.Fatalf("ScrollOff went negative: %d", b.ScrollOff)
	}
}

func TestClampScroll(t *testing.T) {
	b := &Buffer{}
	for i := 0; i < 5; i++ {
		b.Append(Line{Text: "x"})
	}
	b.ScrollOff = 999
	b.ClampScroll()
	if b.ScrollOff != len(b.Lines) {
		t.Fatalf("ClampScroll didn't cap to %d, got %d", len(b.Lines), b.ScrollOff)
	}
	b.ScrollOff = -3
	b.ClampScroll()
	if b.ScrollOff != 0 {
		t.Fatalf("negative not clamped to 0: %d", b.ScrollOff)
	}
}

func TestBufferIsChannelIsPM(t *testing.T) {
	cases := []struct {
		name              string
		isServer          bool
		wantChan, wantPM  bool
	}{
		{"#go", false, true, false},
		{"&local", false, true, false},
		{"alice", false, false, true},
		{"*server*", true, false, false},
	}
	for _, tc := range cases {
		b := &Buffer{Name: tc.name, IsServer: tc.isServer}
		if got := b.IsChannel(); got != tc.wantChan {
			t.Errorf("%q.IsChannel() = %v, want %v", tc.name, got, tc.wantChan)
		}
		if got := b.IsPM(); got != tc.wantPM {
			t.Errorf("%q.IsPM() = %v, want %v", tc.name, got, tc.wantPM)
		}
	}
}
