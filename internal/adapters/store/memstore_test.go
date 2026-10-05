package store_test

import (
	"testing"

	"dockvista/internal/adapters/store"
	"dockvista/internal/core/domain"
)

func TestMemStore_GetByNameAndUniquePrefix(t *testing.T) {
	s := store.New()
	s.Replace([]domain.Container{
		{ID: "abcdef1234567890", Name: "api"},
		{ID: "1234567890abcdef", Name: "db"},
	})

	if c, ok := s.Get("api"); !ok || c.ID != "abcdef1234567890" {
		t.Fatalf("name lookup: ok=%v container=%+v", ok, c)
	}
	if c, ok := s.Get("abcdef123456"); !ok || c.Name != "api" {
		t.Fatalf("prefix lookup: ok=%v container=%+v", ok, c)
	}
	if _, ok := s.Get("abc"); ok {
		t.Fatal("short prefix should not resolve")
	}
	if _, ok := s.Get("missing"); ok {
		t.Fatal("unknown name should miss")
	}
}
