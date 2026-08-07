package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidatePrivateListenerConfigRejectsWildcardControlBind(t *testing.T) {
	for _, address := range []string{":8027", "0.0.0.0:8027", "[::]:8027"} {
		err := validatePrivateListenerConfig(&ServerConfig{
			PublicListenAddr:  "0.0.0.0:8080",
			ControlListenAddr: address,
			ControlHosts:      []string{"100.97.17.1"},
			OperatorPeerIPs:   []string{"100.97.17.25"},
		})
		require.ErrorContains(t, err, "EXAPI_CONTROL_LISTEN_ADDR", address)
	}
}

func TestValidatePrivateListenerConfigRejectsEquivalentListenerAddresses(t *testing.T) {
	err := validatePrivateListenerConfig(&ServerConfig{
		PublicListenAddr:  ":8080",
		ControlListenAddr: "0.0.0.0:8080",
		ControlHosts:      []string{"100.97.17.1"},
		OperatorPeerIPs:   []string{"100.97.17.25"},
	})
	require.ErrorContains(t, err, "must not bind to an unspecified address")

	require.True(t, listenAddressesEqual(":8080", "0.0.0.0:8080"))
	require.True(t, listenAddressesEqual("[::]:8080", "0.0.0.0:8080"))
	require.False(t, listenAddressesEqual("127.0.0.1:8080", "127.0.0.1:8027"))
}

func TestValidatePrivateListenerConfigAcceptsExplicitControlBind(t *testing.T) {
	require.NoError(t, validatePrivateListenerConfig(&ServerConfig{
		PublicListenAddr:  "0.0.0.0:8080",
		ControlListenAddr: "100.97.17.1:8027",
		ControlHosts:      []string{"100.97.17.1"},
		OperatorPeerIPs:   []string{"100.97.17.25"},
	}))
}
