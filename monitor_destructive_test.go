//go:build windows && destructive

// Destructive tests restore the monitor to factory defaults and wipe all
// user configuration. They are excluded from normal runs on purpose and
// require the `destructive` build tag.
package ddcci_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/niluan304/ddcci"
	"github.com/niluan304/ddcci/vcp"
)

func TestPhysicalMonitor_RestoreMonitorFactoryDefaults(t *testing.T) {
	t.Run("With HighLevel Api", func(t *testing.T) {
		m := ddcci.PhysicalMonitor0()
		err := m.RestoreFactoryDefaults()
		require.NoError(t, err)
	})

	time.Sleep(time.Second)

	t.Run("With SetVCPFeature", func(t *testing.T) {
		m := ddcci.PhysicalMonitor0()
		err := m.SetVCPFeature(vcp.RestoreFactoryDefaults.Value(0))
		require.NoError(t, err)
	})
}
