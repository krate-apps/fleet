package storage

import (
	"testing"

	"github.com/krate-apps/fleet/internal/gateways"
)

func TestDefaultGatewayClient(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	a, err := store.UpsertGatewayClient(GatewayClient{
		ID: "gw_a", Kind: gateways.KindDeluge, Name: "A", URL: "http://a", Enabled: true,
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	b, err := store.UpsertGatewayClient(GatewayClient{
		ID: "gw_b", Kind: gateways.KindQBittorrent, Name: "B", URL: "http://b", Enabled: true,
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if a.IsDefault || b.IsDefault {
		t.Fatalf("unexpected default %+v %+v", a, b)
	}
	if _, ok, err := store.GetDefaultGatewayClient(); err != nil || ok {
		t.Fatalf("want no default, ok=%v err=%v", ok, err)
	}
	if err := store.SetDefaultGatewayClient("gw_b"); err != nil {
		t.Fatal(err)
	}
	def, ok, err := store.GetDefaultGatewayClient()
	if err != nil || !ok || def.ID != "gw_b" {
		t.Fatalf("default=%+v ok=%v err=%v", def, ok, err)
	}
	if err := store.SetDefaultGatewayClient("gw_a"); err != nil {
		t.Fatal(err)
	}
	list, err := store.ListGatewayClients()
	if err != nil {
		t.Fatal(err)
	}
	var defaults int
	for _, c := range list {
		if c.IsDefault {
			defaults++
			if c.ID != "gw_a" {
				t.Fatalf("default id=%s", c.ID)
			}
		}
	}
	if defaults != 1 {
		t.Fatalf("defaults=%d", defaults)
	}
}
