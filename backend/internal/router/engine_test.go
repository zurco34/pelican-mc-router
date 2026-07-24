package router

import (
	"errors"
	"testing"
)

func TestParseEngine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		want    Engine
		wantErr bool
	}{
		{
			name:  "mc-router",
			value: "mc-router",
			want:  EngineMCRouter,
		},
		{
			name:  "Infrared",
			value: "infrared",
			want:  EngineInfrared,
		},
		{
			name:  "surrounding whitespace",
			value: "  mc-router  ",
			want:  EngineMCRouter,
		},
		{
			name:    "empty",
			value:   "",
			wantErr: true,
		},
		{
			name:    "unsupported",
			value:   "velocity",
			wantErr: true,
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseEngine(test.value)

			if test.wantErr {
				if !errors.Is(err, ErrUnsupportedEngine) {
					t.Fatalf(
						"ParseEngine(%q) error = %v, want error %v",
						test.value,
						err,
						ErrUnsupportedEngine,
					)
				}

				return
			}

			if err != nil {
				t.Fatalf(
					"ParseEngine(%q) error = %v",
					test.value,
					err,
				)
			}

			if got != test.want {
				t.Fatalf(
					"ParseEngine(%q) = %q, want %q",
					test.value,
					got,
					test.want,
				)
			}
		})
	}
}
