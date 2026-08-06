package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveServerRole(t *testing.T) {
	originalDefault := DefaultServerRole
	originalLocked := LockedServerRole
	t.Cleanup(func() {
		DefaultServerRole = originalDefault
		LockedServerRole = originalLocked
	})

	t.Run("environment overrides build default", func(t *testing.T) {
		DefaultServerRole = string(serverRoleControl)
		t.Setenv("SERVER_ROLE", "relay")
		role, err := resolveServerRole()
		require.NoError(t, err)
		assert.Equal(t, serverRoleRelay, role)
	})

	t.Run("build default is used when environment is empty", func(t *testing.T) {
		DefaultServerRole = string(serverRoleControl)
		t.Setenv("SERVER_ROLE", "")
		role, err := resolveServerRole()
		require.NoError(t, err)
		assert.Equal(t, serverRoleControl, role)
	})

	t.Run("invalid role is rejected", func(t *testing.T) {
		LockedServerRole = ""
		t.Setenv("SERVER_ROLE", "unknown")
		_, err := resolveServerRole()
		require.Error(t, err)
	})

	t.Run("locked relay executable rejects control override", func(t *testing.T) {
		LockedServerRole = string(serverRoleRelay)
		t.Setenv("SERVER_ROLE", "control")
		_, err := resolveServerRole()
		require.Error(t, err)
	})
}
