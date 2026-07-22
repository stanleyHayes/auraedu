package servercmd

import (
	"context"
	"errors"
	"testing"
)

type fakePinger struct {
	err         error
	sawDeadline bool
}

func (p *fakePinger) Ping(ctx context.Context) error {
	_, p.sawDeadline = ctx.Deadline()
	return p.err
}

func TestReadinessCheckUsesDeadlineAndPropagatesFailure(t *testing.T) {
	want := errors.New("postgres unavailable")
	pinger := &fakePinger{err: want}
	if got := readinessCheck(pinger)(); !errors.Is(got, want) {
		t.Fatalf("readiness error=%v, want %v", got, want)
	}
	if !pinger.sawDeadline {
		t.Fatal("database readiness ping must have a deadline")
	}
}
