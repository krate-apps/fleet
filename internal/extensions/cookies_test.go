package extensions

import "testing"

func TestCookieStoreHeaderForURL(t *testing.T) {
	dir := t.TempDir()
	store := &CookieStore{DataDir: dir}
	if err := store.Set([]HostCookieEntry{
		{Host: "Tracker.Example", Cookies: "uid=1; pass=abc"},
		{Host: "empty.example", Cookies: ""},
	}); err != nil {
		t.Fatal(err)
	}
	got, err := store.HeaderForURL("https://tracker.example/feed.xml")
	if err != nil {
		t.Fatal(err)
	}
	if got != "pass=abc; uid=1" {
		t.Fatalf("got %q", got)
	}
	if err := store.Upsert("tracker.example", "uid=2; session=xyz"); err != nil {
		t.Fatal(err)
	}
	got, err = store.HeaderForURL("https://tracker.example/")
	if err != nil {
		t.Fatal(err)
	}
	if got != "pass=abc; session=xyz; uid=2" {
		t.Fatalf("upsert got %q", got)
	}
	miss, err := store.HeaderForURL("https://other.example/")
	if err != nil || miss != "" {
		t.Fatalf("miss=%q err=%v", miss, err)
	}
	list, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Host != "tracker.example" {
		t.Fatalf("%#v", list)
	}
	if err := store.Set([]HostCookieEntry{{Host: "", Cookies: "a=b"}}); err == nil {
		t.Fatal("expected host required")
	}
}
