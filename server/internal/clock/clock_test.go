package clock

import (
	"testing"
	"time"
)

func TestSystemReturnsCurrentUTCTime(t *testing.T) {
	t.Parallel()

	before := time.Now().UTC()
	got := (System{}).Now()
	after := time.Now().UTC()

	if got.Location() != time.UTC {
		t.Errorf("location = %v, want UTC", got.Location())
	}
	if got.Before(before) || got.After(after) {
		t.Errorf("Now() = %v, want between %v and %v", got, before, after)
	}
}

func TestFixedReturnsConfiguredInstantInUTC(t *testing.T) {
	t.Parallel()

	location := time.FixedZone("test", -7*60*60)
	configured := time.Date(2026, time.July, 29, 18, 30, 45, 123456789, location)
	var clock Clock = Fixed{Time: configured}

	got := clock.Now()

	if !got.Equal(configured) {
		t.Errorf("Now() = %v, want instant %v", got, configured)
	}
	if got.Location() != time.UTC {
		t.Errorf("location = %v, want UTC", got.Location())
	}
}
