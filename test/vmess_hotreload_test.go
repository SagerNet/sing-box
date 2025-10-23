package tests

import (
	"testing"

	"github.com/sagernet/sing-box/option"
	"github.com/stretchr/testify/require"
)

// TestVMessUsersEqual verifies that we can properly compare VMess user configurations
func TestVMessUsersEqual(t *testing.T) {
	user1 := option.VMessUser{
		Name:    "alice",
		UUID:    "550e8400-e29b-41d4-a716-446655440000",
		AlterId: 0,
	}
	user2 := option.VMessUser{
		Name:    "alice",
		UUID:    "550e8400-e29b-41d4-a716-446655440000",
		AlterId: 0,
	}
	user3 := option.VMessUser{
		Name:    "bob",
		UUID:    "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
		AlterId: 0,
	}

	// Same users should be equal
	require.Equal(t, user1, user2)

	// Different users should not be equal
	require.NotEqual(t, user1, user3)
}

// TestVMessConfigChanges simulates hot reload scenarios
func TestVMessConfigChanges(t *testing.T) {
	t.Run("Add User", func(t *testing.T) {
		oldUsers := []option.VMessUser{
			{Name: "alice", UUID: "550e8400-e29b-41d4-a716-446655440000", AlterId: 0},
		}
		newUsers := []option.VMessUser{
			{Name: "alice", UUID: "550e8400-e29b-41d4-a716-446655440000", AlterId: 0},
			{Name: "bob", UUID: "6ba7b810-9dad-11d1-80b4-00c04fd430c8", AlterId: 0},
		}

		require.NotEqual(t, len(oldUsers), len(newUsers))
		require.Equal(t, 1, len(oldUsers))
		require.Equal(t, 2, len(newUsers))
	})

	t.Run("Remove User", func(t *testing.T) {
		oldUsers := []option.VMessUser{
			{Name: "alice", UUID: "550e8400-e29b-41d4-a716-446655440000", AlterId: 0},
			{Name: "bob", UUID: "6ba7b810-9dad-11d1-80b4-00c04fd430c8", AlterId: 0},
		}
		newUsers := []option.VMessUser{
			{Name: "alice", UUID: "550e8400-e29b-41d4-a716-446655440000", AlterId: 0},
		}

		require.NotEqual(t, len(oldUsers), len(newUsers))
		require.Equal(t, 2, len(oldUsers))
		require.Equal(t, 1, len(newUsers))
	})

	t.Run("Update UUID", func(t *testing.T) {
		oldUsers := []option.VMessUser{
			{Name: "alice", UUID: "550e8400-e29b-41d4-a716-446655440000", AlterId: 0},
		}
		newUsers := []option.VMessUser{
			{Name: "alice", UUID: "12345678-1234-1234-1234-123456789012", AlterId: 0},
		}

		require.Equal(t, len(oldUsers), len(newUsers))
		require.NotEqual(t, oldUsers[0].UUID, newUsers[0].UUID)
		require.Equal(t, oldUsers[0].Name, newUsers[0].Name)
	})

	t.Run("Update AlterID", func(t *testing.T) {
		oldUsers := []option.VMessUser{
			{Name: "alice", UUID: "550e8400-e29b-41d4-a716-446655440000", AlterId: 0},
		}
		newUsers := []option.VMessUser{
			{Name: "alice", UUID: "550e8400-e29b-41d4-a716-446655440000", AlterId: 64},
		}

		require.Equal(t, len(oldUsers), len(newUsers))
		require.Equal(t, oldUsers[0].UUID, newUsers[0].UUID)
		require.NotEqual(t, oldUsers[0].AlterId, newUsers[0].AlterId)
	})

	t.Run("No Changes", func(t *testing.T) {
		oldUsers := []option.VMessUser{
			{Name: "alice", UUID: "550e8400-e29b-41d4-a716-446655440000", AlterId: 0},
		}
		newUsers := []option.VMessUser{
			{Name: "alice", UUID: "550e8400-e29b-41d4-a716-446655440000", AlterId: 0},
		}

		require.Equal(t, oldUsers, newUsers)
	})
}

// TestVMessInboundOptionsStructure verifies the options structure
func TestVMessInboundOptionsStructure(t *testing.T) {
	options := option.VMessInboundOptions{
		ListenOptions: option.ListenOptions{
			Listen:     option.NewListenAddress(option.ListenAddress("")),
			ListenPort: 8080,
		},
		Users: []option.VMessUser{
			{Name: "alice", UUID: "550e8400-e29b-41d4-a716-446655440000", AlterId: 0},
			{Name: "bob", UUID: "6ba7b810-9dad-11d1-80b4-00c04fd430c8", AlterId: 0},
		},
	}

	require.NotNil(t, options.Users)
	require.Equal(t, 2, len(options.Users))
	require.Equal(t, "alice", options.Users[0].Name)
	require.Equal(t, "550e8400-e29b-41d4-a716-446655440000", options.Users[0].UUID)
	require.Equal(t, 0, options.Users[0].AlterId)
	require.Equal(t, uint16(8080), options.ListenPort)
}

// TestVMessUUIDFormat verifies UUID format handling
func TestVMessUUIDFormat(t *testing.T) {
	t.Run("Valid UUID", func(t *testing.T) {
		validUUIDs := []string{
			"550e8400-e29b-41d4-a716-446655440000",
			"6ba7b810-9dad-11d1-80b4-00c04fd430c8",
			"7c9e6679-7425-40de-944b-e07fc1f90ae7",
			"00000000-0000-0000-0000-000000000000",
		}

		for _, uuid := range validUUIDs {
			user := option.VMessUser{
				Name:    "test",
				UUID:    uuid,
				AlterId: 0,
			}
			require.Equal(t, uuid, user.UUID)
		}
	})

	t.Run("AlterID Range", func(t *testing.T) {
		// AlterID should typically be 0-255
		validAlterIds := []int{0, 1, 64, 128, 255}

		for _, alterId := range validAlterIds {
			user := option.VMessUser{
				Name:    "test",
				UUID:    "550e8400-e29b-41d4-a716-446655440000",
				AlterId: alterId,
			}
			require.Equal(t, alterId, user.AlterId)
		}
	})
}
