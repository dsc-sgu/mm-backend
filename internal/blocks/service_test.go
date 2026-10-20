package blocks

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateBlockData(t *testing.T) {
	tests := []struct {
		name  string
		data  string
		valid bool
	}{
		{"object with version", `{"version":1,"text":"a"}`, true},
		{"no version", `{"text":"a"}`, false},
		{"fractional version", `{"version":1.5}`, false},
		{"string version", `{"version":"1"}`, false},
		{"not an object", `[1]`, false},
		{"null", `null`, false},
		{"empty", ``, false},
		{
			"too large",
			`{"version":1,"text":"` + strings.Repeat("a", maxBlockDataSize) + `"}`,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBlockData(json.RawMessage(tt.data))
			if tt.valid {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, ErrInvalidBlockData)
			}
		})
	}
}
