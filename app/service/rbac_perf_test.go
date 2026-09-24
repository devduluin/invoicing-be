package service

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"duluin_invoice/app/sso"
)

type slowRoleSSO struct {
	*fakeSSO
	calls int32
}

func (s *slowRoleSSO) GetRolePermissions(_ context.Context, roleID, _, _ string) (sso.RolePermissions, error) {
	atomic.AddInt32(&s.calls, 1)
	time.Sleep(150 * time.Millisecond)
	return sso.RolePermissions{RoleID: roleID, RoleName: "Owner", Permissions: []string{"invoice-mitra-list"}}, nil
}

// Regression for "every permission-gated API takes ~20s": with Redis down the RBAC cache was a
// no-op, so every concurrent request fetched the role's permissions from SSO on its own.
func TestResolveRolePermissions_CoalescedAndCachedWithoutRedis(t *testing.T) {
	stub := &slowRoleSSO{fakeSSO: newFakeSSO()}
	svc := &MembershipService{sso: stub, cache: NewRedisRBACCache(nil)} // nil client = Redis down

	fire := func(n int) {
		var wg sync.WaitGroup
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				perms, name, err := svc.resolveRolePermissions(context.Background(), "role-1", "co-1", "tok")
				if err != nil || name != "Owner" || len(perms) != 1 {
					t.Errorf("bad result: %v %q %v", perms, name, err)
				}
			}()
		}
		wg.Wait()
	}
	fire(8)
	if got := atomic.LoadInt32(&stub.calls); got != 1 {
		t.Fatalf("8 concurrent lookups made %d SSO calls, want 1", got)
	}
	fire(8)
	if got := atomic.LoadInt32(&stub.calls); got != 1 {
		t.Fatalf("cached lookups hit SSO again: total %d, want 1", got)
	}
}

func TestLocalRBACCache_ExpiryAndInvalidation(t *testing.T) {
	c := NewRedisRBACCache(nil)
	ctx := context.Background()
	c.SetJSON(ctx, "rbac:a", map[string]string{"k": "v"}, time.Hour)
	c.SetJSON(ctx, "rbac:b", 1, 20*time.Millisecond)
	var m map[string]string
	if !c.GetJSON(ctx, "rbac:a", &m) || m["k"] != "v" {
		t.Fatal("expected local hit")
	}
	time.Sleep(40 * time.Millisecond)
	var n int
	if c.GetJSON(ctx, "rbac:b", &n) {
		t.Fatal("expired entry must miss")
	}
	c.DelByPattern(ctx, "rbac:*")
	if c.GetJSON(ctx, "rbac:a", &m) {
		t.Fatal("pattern delete must invalidate the local cache too")
	}
}
