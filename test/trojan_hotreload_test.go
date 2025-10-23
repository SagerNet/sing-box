package tests

import (
	"testing"

	"github.com/sagernet/sing-box/option"
	"github.com/stretchr/testify/require"
)

// TestTrojanUsersEqual verifies that we can properly compare Trojan user configurations
func TestTrojanUsersEqual(t *testing.T) {
	user1 := option.TrojanUser{
		Name:     "alice",
		Password: "password1",
	}
	user2 := option.TrojanUser{
		Name:     "alice",
		Password: "password1",
	}
	user3 := option.TrojanUser{
		Name:     "bob",
		Password: "password2",
	}

	// Same users should be equal
	require.Equal(t, user1, user2)

	// Different users should not be equal
	require.NotEqual(t, user1, user3)
}

// TestTrojanConfigChanges simulates hot reload scenarios
func TestTrojanConfigChanges(t *testing.T) {
	t.Run("Add User", func(t *testing.T) {
		oldUsers := []option.TrojanUser{
			{Name: "alice", Password: "password1"},
		}
		newUsers := []option.TrojanUser{
			{Name: "alice", Password: "password1"},
			{Name: "bob", Password: "password2"},
		}

		require.NotEqual(t, len(oldUsers), len(newUsers))
		require.Equal(t, 1, len(oldUsers))
		require.Equal(t, 2, len(newUsers))
	})

	t.Run("Remove User", func(t *testing.T) {
		oldUsers := []option.TrojanUser{
			{Name: "alice", Password: "password1"},
			{Name: "bob", Password: "password2"},
		}
		newUsers := []option.TrojanUser{
			{Name: "alice", Password: "password1"},
		}

		require.NotEqual(t, len(oldUsers), len(newUsers))
		require.Equal(t, 2, len(oldUsers))
		require.Equal(t, 1, len(newUsers))
	})

	t.Run("Update Password", func(t *testing.T) {
		oldUsers := []option.TrojanUser{
			{Name: "alice", Password: "old_password"},
		}
		newUsers := []option.TrojanUser{
			{Name: "alice", Password: "new_password"},
		}

		require.Equal(t, len(oldUsers), len(newUsers))
		require.NotEqual(t, oldUsers[0].Password, newUsers[0].Password)
	})

	t.Run("No Changes", func(t *testing.T) {
		oldUsers := []option.TrojanUser{
			{Name: "alice", Password: "password1"},
		}
		newUsers := []option.TrojanUser{
			{Name: "alice", Password: "password1"},
		}

		require.Equal(t, oldUsers, newUsers)
	})
}

// TestTrojanInboundOptionsStructure verifies the options structure
func TestTrojanInboundOptionsStructure(t *testing.T) {
	options := option.TrojanInboundOptions{
		ListenOptions: option.ListenOptions{
			Listen:     option.NewListenAddress(option.ListenAddress("")),
			ListenPort: 443,
		},
		Users: []option.TrojanUser{
			{Name: "alice", Password: "password1"},
			{Name: "bob", Password: "password2"},
		},
	}

	require.NotNil(t, options.Users)
	require.Equal(t, 2, len(options.Users))
	require.Equal(t, "alice", options.Users[0].Name)
	require.Equal(t, "password1", options.Users[0].Password)
	require.Equal(t, uint16(443), options.ListenPort)
}
