package permissions

import (
	"testing"

	"github.com/ElioNeto/devon/internal/config"
)

type mockTool struct {
	name string
	perm PermissionLevel
}

func (m *mockTool) Name() string                { return m.name }
func (m *mockTool) Permission() PermissionLevel { return m.perm }

func TestPermissionLevel_String(t *testing.T) {
	tests := []struct {
		p    PermissionLevel
		want string
	}{
		{PermRead, "read"},
		{PermWrite, "write"},
		{PermExecute, "execute"},
		{PermissionLevel(99), "unknown"},
	}
	for _, tc := range tests {
		if got := tc.p.String(); got != tc.want {
			t.Errorf("PermissionLevel(%d).String() = %q, want %q", tc.p, got, tc.want)
		}
	}
}

func TestChecker_Requires_ModeYolo(t *testing.T) {
	c := &Checker{Mode: config.ModeYolo}
	tool := &mockTool{name: "bash", perm: PermExecute}

	_, confirm := c.Requires(tool)
	if confirm {
		t.Error("expected no confirmation in yolo mode")
	}
}

func TestChecker_Requires_ModeSafe(t *testing.T) {
	c := &Checker{Mode: config.ModeSafe}
	tool := &mockTool{name: "read_file", perm: PermRead}

	blocked, confirm := c.Requires(tool)
	if blocked {
		t.Error("expected not blocked")
	}
	if !confirm {
		t.Error("expected confirmation in safe mode even for read")
	}
}

func TestChecker_Requires_ModeAuto_Read(t *testing.T) {
	c := &Checker{Mode: config.ModeAuto}
	tool := &mockTool{name: "read_file", perm: PermRead}

	blocked, confirm := c.Requires(tool)
	if blocked {
		t.Error("expected not blocked")
	}
	if confirm {
		t.Error("expected no confirmation for read in auto mode")
	}
}

func TestChecker_Requires_ModeAuto_Write(t *testing.T) {
	c := &Checker{Mode: config.ModeAuto}
	tool := &mockTool{name: "write_file", perm: PermWrite}

	_, confirm := c.Requires(tool)
	if !confirm {
		t.Error("expected confirmation for write in auto mode")
	}
}

func TestChecker_Requires_ModeAuto_Execute(t *testing.T) {
	c := &Checker{Mode: config.ModeAuto}
	tool := &mockTool{name: "bash", perm: PermExecute}

	_, confirm := c.Requires(tool)
	if !confirm {
		t.Error("expected confirmation for execute in auto mode")
	}
}

func TestChecker_Requires_Blocklist(t *testing.T) {
	c := &Checker{
		Mode:      config.ModeYolo,
		Blocklist: []string{"rm -rf /"},
	}
	tool := &mockTool{name: "rm -rf /", perm: PermExecute}

	blocked, _ := c.Requires(tool)
	if !blocked {
		t.Error("expected blocked")
	}
}

func TestChecker_Requires_SessionApproved(t *testing.T) {
	c := &Checker{
		Mode:    config.ModeSafe,
		Session: map[string]bool{"bash": true},
	}
	tool := &mockTool{name: "bash", perm: PermExecute}

	_, confirm := c.Requires(tool)
	if confirm {
		t.Error("session-approved tool should not require confirmation")
	}
}

func TestChecker_Requires_UnknownMode(t *testing.T) {
	c := &Checker{Mode: config.Mode(99)}
	tool := &mockTool{name: "read", perm: PermRead}

	_, confirm := c.Requires(tool)
	if !confirm {
		t.Error("unknown mode should default to requiring confirmation")
	}
}

func TestChecker_Approve_NilSession(t *testing.T) {
	c := &Checker{Mode: config.ModeAuto}
	c.Approve("bash")

	if c.Session == nil {
		t.Fatal("Session should be initialized after Approve")
	}
	if !c.Session["bash"] {
		t.Error("bash should be approved")
	}
}

func TestChecker_Approve_ExistingSession(t *testing.T) {
	c := &Checker{
		Mode:    config.ModeAuto,
		Session: map[string]bool{},
	}
	c.Approve("write_file")

	if !c.Session["write_file"] {
		t.Error("write_file should be approved")
	}
}
