package pantry

import "testing"

func TestOwnerCanPromoteExistingMember(t *testing.T) {
	server, store := newTestServer(t)
	owner := registerHousehold(t, server, "promote_owner", "甲家庭")
	otherOwner := registerHousehold(t, server, "other_owner", "乙家庭")
	join := func(username string) (*testClient, string) {
		t.Helper()
		invite := decodeBody[map[string]string](t, owner.request("POST", "/api/v1/household-invites", `{}`))
		member := &testClient{t: t, server: server}
		assertStatus(t, member.request("POST", "/api/v1/household-invites/"+invite["token"]+"/accept", `{"username":"`+username+`","password":"member-pass","display_name":"家人"}`), 200)
		me := decodeBody[map[string]any](t, member.request("GET", "/api/v1/me", ""))
		if me["role"] != "member" {
			t.Fatal("invitation should still create a member")
		}
		return member, me["id"].(string)
	}
	member, id := join("promote_member")
	_, secondID := join("second_member")
	_, disabledID := join("disabled_member")
	if _, err := store.DB.Exec(`UPDATE users SET status='disabled' WHERE id=?`, disabledID); err != nil {
		t.Fatal(err)
	}
	path := "/api/v1/members/" + id + "/promote-owner"
	anonymous := &testClient{t: t, server: server}
	assertStatus(t, anonymous.request("POST", path, ""), 401)
	assertStatus(t, member.request("POST", path, ""), 403)
	assertStatus(t, member.request("POST", "/api/v1/members/"+secondID+"/promote-owner", ""), 403)
	assertStatus(t, otherOwner.request("POST", path, ""), 404)
	assertStatus(t, owner.request("POST", "/api/v1/members/missing/promote-owner", ""), 404)
	assertStatus(t, owner.request("POST", "/api/v1/members/"+disabledID+"/promote-owner", ""), 404)
	admin := &testClient{t: t, server: server}
	assertStatus(t, admin.request("POST", "/api/v1/auth/login", `{"username":"admin","password":"admin-secret-123"}`), 200)
	assertStatus(t, admin.request("POST", path, ""), 403)
	if me := decodeBody[map[string]any](t, member.request("GET", "/api/v1/me", "")); me["role"] != "member" {
		t.Fatal("unauthorized requests changed role")
	}
	assertStatus(t, owner.request("POST", path, ""), 200)
	assertStatus(t, owner.request("POST", path, ""), 200) // Retry is idempotent.
	me := decodeBody[map[string]any](t, member.request("GET", "/api/v1/me", ""))
	if me["role"] != "owner" {
		t.Fatalf("role=%v", me["role"])
	}
	// Existing session gains actual owner permissions, not just a changed label.
	assertStatus(t, member.request("POST", "/api/v1/household-invites", `{}`), 201)
	var role string
	if err := store.DB.QueryRow(`SELECT role FROM users WHERE id=?`, secondID).Scan(&role); err != nil {
		t.Fatal(err)
	}
	if role != "member" {
		t.Fatal("unrelated members must not be automatically promoted")
	}
	assertStatus(t, member.request("POST", "/api/v1/members/"+secondID+"/promote-owner", ""), 200)
	assertStatus(t, owner.request("DELETE", "/api/v1/members/"+id, ""), 400)
}
