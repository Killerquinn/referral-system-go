package service

import (
	"errors"
	"testing"

	sharederrors "github.com/killerquinn/referral-system-go/internal/shared/shared-errors.go"
)

func TestValidateSettingsEffectGroups(t *testing.T) {
	cases := []struct {
		name    string
		effects []string
		ok      bool
	}{
		{"empty", nil, true},
		{"one of each group", []string{"snow", "smolder_name", "cursor_wind"}, true},
		{"three backgrounds (old pages)", []string{"snow", "rain", "particles"}, true},
		{"four backgrounds", []string{"snow", "rain", "particles", "stars"}, false},
		{"two name effects", []string{"glow_name", "glitch_name"}, false},
		{"all extras", []string{"typewriter_bio", "tilt_card", "cursor_wind"}, true},
		{"unknown", []string{"lasers"}, false},
		{"duplicate", []string{"smoke", "smoke"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := defaultSettings()
			s.Effects = c.effects
			err := validateSettings(&s)
			if c.ok && err != nil {
				t.Fatalf("want ok, got %v", err)
			}
			if !c.ok && !errors.Is(err, sharederrors.ErrInvalidPageSettings) {
				t.Fatalf("want ErrInvalidPageSettings, got %v", err)
			}
		})
	}
}
