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
	t.Run("CombinedClient with Norman", func(t *testing.T) {
		n := &norman.Client{}
		cc := &CombinedClient{Norman: n}
		got, err := ValidateNormanClient(cc)
		if err != nil || got != n {
			t.Fatal("expected CombinedClient.Norman to be returned")
		}
	})

	t.Run("direct Norman client", func(t *testing.T) {
		n := &norman.Client{}
		got, err := ValidateNormanClient(n)
		if err != nil || got != n {
			t.Fatal("expected direct Norman client to be returned")
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		_, err := ValidateNormanClient(42)
		if err == nil {
			t.Fatal("expected error for wrong type")
		}
	})
}
