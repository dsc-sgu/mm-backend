package attempt

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseSubmitOption(t *testing.T) {
	require.Equal(t, "task-a", parseSubmitOption([]string{"trace=1", " submit=task-a "}))
	require.Empty(t, parseSubmitOption([]string{"trace=1"}))
}
