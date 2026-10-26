package connection

import (
	"context"
	"testing"
)

// hosts CRUD 往返：Save → Get → Update → List → Delete。
func TestHostCRUD(t *testing.T) {
	s := testConnServiceWithDB(t)
	ctx := context.Background()

	id, err := s.SaveHost(ctx, Host{
		Name: "prod", Host: "10.0.0.1", Port: "2222", User: "deploy",
		Auth: 1, KeyPath: `C:\keys\id_ed25519`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if id == 0 {
		t.Fatal("want new id")
	}

	h, err := s.GetHost(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if h.Name != "prod" || h.Host != "10.0.0.1" || h.Port != "2222" || h.Auth != 1 {
		t.Fatalf("unexpected host: %+v", h)
	}

	h.Name = "prod-2"
	h.Auth = 0
	if _, err := s.SaveHost(ctx, h); err != nil {
		t.Fatal(err)
	}
	h2, _ := s.GetHost(ctx, id)
	if h2.Name != "prod-2" || h2.Auth != 0 {
		t.Fatalf("update not applied: %+v", h2)
	}

	list, err := s.ListHosts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("want 1 host, got %d", len(list))
	}

	if err := s.DeleteHost(ctx, id); err != nil {
		t.Fatal(err)
	}
	list, _ = s.ListHosts(ctx)
	if len(list) != 0 {
		t.Fatalf("want 0 hosts after delete, got %d", len(list))
	}
}

// 校验：host/user 必填；未知 auth code 拒绝；默认值。
func TestHostValidation(t *testing.T) {
	s := testConnServiceWithDB(t)
	ctx := context.Background()

	if _, err := s.SaveHost(ctx, Host{Host: "", User: "u"}); err == nil {
		t.Fatal("empty host must be rejected")
	}
	if _, err := s.SaveHost(ctx, Host{Host: "h", User: ""}); err == nil {
		t.Fatal("empty user must be rejected")
	}
	if _, err := s.SaveHost(ctx, Host{Host: "h", User: "u", Auth: 9}); err == nil {
		t.Fatal("unknown auth code must be rejected")
	}
	id, err := s.SaveHost(ctx, Host{Host: "srv", User: "u"})
	if err != nil {
		t.Fatal(err)
	}
	h, _ := s.GetHost(ctx, id)
	if h.Port != "22" || h.Name != "srv" {
		t.Fatalf("defaults not applied: %+v", h)
	}
}
