//go:build linux || (darwin && cgo)

package usbip

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestConsumeImportLeaseAcceptsCurrentGeneration(t *testing.T) {
	t.Parallel()

	const busid = "1-1"
	server := &ServerService{
		controlSeq: 7,
		leasesByBusID: map[string]serverImportLease{
			busid: {
				ID:          55,
				BusID:       busid,
				ClientNonce: 99,
				Generation:  7,
				Expires:     time.Now().Add(importLeaseTTL),
			},
		},
	}

	ok := server.consumeImportLease(ImportExtRequest{
		BusID:       busid,
		LeaseID:     55,
		ClientNonce: 99,
	})
	require.True(t, ok)
	require.Empty(t, server.leasesByBusID)
}

func TestConsumeImportLeaseRejectsStaleGeneration(t *testing.T) {
	t.Parallel()

	const busid = "1-1"
	server := &ServerService{
		controlSeq: 8,
		leasesByBusID: map[string]serverImportLease{
			busid: {
				ID:          55,
				BusID:       busid,
				ClientNonce: 99,
				Generation:  7,
				Expires:     time.Now().Add(importLeaseTTL),
			},
		},
	}

	ok := server.consumeImportLease(ImportExtRequest{
		BusID:       busid,
		LeaseID:     55,
		ClientNonce: 99,
	})
	require.False(t, ok)
	require.Empty(t, server.leasesByBusID)
}

func TestConsumeImportLeaseRejectsExpiredLease(t *testing.T) {
	t.Parallel()

	const busid = "1-1"
	server := &ServerService{
		controlSeq: 7,
		leasesByBusID: map[string]serverImportLease{
			busid: {
				ID:          55,
				BusID:       busid,
				ClientNonce: 99,
				Generation:  7,
				Expires:     time.Now().Add(-time.Second),
			},
		},
	}

	ok := server.consumeImportLease(ImportExtRequest{
		BusID:       busid,
		LeaseID:     55,
		ClientNonce: 99,
	})
	require.False(t, ok)
	require.Empty(t, server.leasesByBusID)
}
