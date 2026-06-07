package toolset

import (
	"testing"

	"github.com/futuretea/rancher-mcp-server/pkg/client/norman"
	"github.com/futuretea/rancher-mcp-server/pkg/client/steve"
)

func TestValidateSteveClient(t *testing.T) {
	t.Run("CombinedClient with Steve", func(t *testing.T) {
		s := &steve.Client{}
		cc := &CombinedClient{Steve: s}
		got, err := ValidateSteveClient(cc)
		if err != nil || got != s {
			t.Fatal("expected CombinedClient.Steve to be returned")
		}
	})

	t.Run("CombinedClient with nil Steve", func(t *testing.T) {
		cc := &CombinedClient{}
		_, err := ValidateSteveClient(cc)
		if err == nil {
			t.Fatal("expected error for nil Steve in CombinedClient")
		}
	})

	t.Run("direct Steve client", func(t *testing.T) {
		s := &steve.Client{}
		got, err := ValidateSteveClient(s)
		if err != nil || got != s {
			t.Fatal("expected direct Steve client to be returned")
		}
	})

	t.Run("nil client", func(t *testing.T) {
		_, err := ValidateSteveClient(nil)
		if err == nil {
			t.Fatal("expected error for nil client")
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		_, err := ValidateSteveClient("not-a-client")
		if err == nil {
			t.Fatal("expected error for wrong type")
		}
	})
}

func TestValidateNormanClient(t *testing.T) {
	t.Run("CombinedClient with unusable Norman", func(t *testing.T) {
		cc := &CombinedClient{Norman: &norman.Client{}}
		_, err := ValidateNormanClient(cc)
		if err == nil {
			t.Fatal("expected error for unusable Norman client")
		}
	})

	t.Run("direct unusable Norman client", func(t *testing.T) {
		_, err := ValidateNormanClient(&norman.Client{})
		if err == nil {
			t.Fatal("expected error for unusable Norman client")
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		_, err := ValidateNormanClient(42)
		if err == nil {
			t.Fatal("expected error for wrong type")
		}
	})
}
