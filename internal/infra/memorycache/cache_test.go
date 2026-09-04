package memorycache

import (
	"testing"
	"time"
)

func TestCache_SetGet(t *testing.T) {
	c := New()
	c.Set("k", "v", 0)
	if v, ok := c.Get("k"); !ok || v != "v" {
		t.Errorf("Get = (%q,%v), want (v,true)", v, ok)
	}
	if _, ok := c.Get("nope"); ok {
		t.Error("missing key should return false")
	}
}

func TestCache_TTLExpiry(t *testing.T) {
	c := New()
	c.Set("k", "v", 30*time.Millisecond)
	if _, ok := c.Get("k"); !ok {
		t.Fatal("expected present before expiry")
	}
	time.Sleep(40 * time.Millisecond)
	if _, ok := c.Get("k"); ok {
		t.Fatal("expected expired")
	}
}

func TestCache_Overwrite(t *testing.T) {
	c := New()
	c.Set("k", "v1", 0)
	c.Set("k", "v2", 0)
	if v, _ := c.Get("k"); v != "v2" {
		t.Errorf("Get = %q, want v2", v)
	}
}
