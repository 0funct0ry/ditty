package store

import "testing"

func TestCreateUser_DuplicateRejected(t *testing.T) {
	s := openTemp(t)
	if _, err := s.CreateUser("alice", "hunter22", RoleViewer); err != nil {
		t.Fatalf("first CreateUser: %v", err)
	}
	if _, err := s.CreateUser("alice", "different", RoleOperator); err != ErrUserExists {
		t.Fatalf("second CreateUser error = %v, want ErrUserExists", err)
	}
}

func TestCreateUser_InvalidRoleRejected(t *testing.T) {
	s := openTemp(t)
	if _, err := s.CreateUser("alice", "hunter22", "admin"); err == nil {
		t.Fatal("expected an error for an unknown role")
	}
}

func TestGetUserByUsername_CaseInsensitive(t *testing.T) {
	s := openTemp(t)
	if _, err := s.CreateUser("Alice", "hunter22", RoleViewer); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	u, found, err := s.GetUserByUsername("alice")
	if err != nil || !found {
		t.Fatalf("GetUserByUsername: %v found=%v", err, found)
	}
	if u.Username != "Alice" {
		t.Fatalf("Username = %q, want Alice", u.Username)
	}
}

func TestGetUserByUsername_NotFound(t *testing.T) {
	s := openTemp(t)
	_, found, err := s.GetUserByUsername("nobody")
	if err != nil {
		t.Fatalf("GetUserByUsername: %v", err)
	}
	if found {
		t.Fatal("expected found=false for a nonexistent user")
	}
}

func TestListUsers_OrderedByUsername(t *testing.T) {
	s := openTemp(t)
	for _, name := range []string{"carol", "alice", "bob"} {
		if _, err := s.CreateUser(name, "hunter22", RoleViewer); err != nil {
			t.Fatalf("CreateUser(%s): %v", name, err)
		}
	}
	users, err := s.ListUsers()
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	want := []string{"alice", "bob", "carol"}
	if len(users) != len(want) {
		t.Fatalf("len(users) = %d, want %d", len(users), len(want))
	}
	for i, name := range want {
		if users[i].Username != name {
			t.Fatalf("users[%d].Username = %q, want %q", i, users[i].Username, name)
		}
	}
}

func TestVerifyPassword(t *testing.T) {
	s := openTemp(t)
	if _, err := s.CreateUser("alice", "hunter22", RoleOperator); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	if _, ok, err := s.VerifyPassword("alice", "wrong"); err != nil || ok {
		t.Fatalf("wrong password: ok=%v err=%v, want ok=false", ok, err)
	}
	u, ok, err := s.VerifyPassword("alice", "hunter22")
	if err != nil || !ok {
		t.Fatalf("correct password: ok=%v err=%v, want ok=true", ok, err)
	}
	if u.Role != RoleOperator {
		t.Fatalf("Role = %q, want %q", u.Role, RoleOperator)
	}
}

func TestVerifyPassword_DisabledUserRejected(t *testing.T) {
	s := openTemp(t)
	if _, err := s.CreateUser("alice", "hunter22", RoleOperator); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if err := s.SetDisabled("alice", true); err != nil {
		t.Fatalf("SetDisabled: %v", err)
	}
	if _, ok, err := s.VerifyPassword("alice", "hunter22"); err != nil || ok {
		t.Fatalf("disabled user: ok=%v err=%v, want ok=false", ok, err)
	}
}

func TestSetPassword(t *testing.T) {
	s := openTemp(t)
	if _, err := s.CreateUser("alice", "hunter22", RoleViewer); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if err := s.SetPassword("alice", "newpass99"); err != nil {
		t.Fatalf("SetPassword: %v", err)
	}
	if _, ok, err := s.VerifyPassword("alice", "hunter22"); err != nil || ok {
		t.Fatalf("old password should no longer work: ok=%v err=%v", ok, err)
	}
	if _, ok, err := s.VerifyPassword("alice", "newpass99"); err != nil || !ok {
		t.Fatalf("new password should work: ok=%v err=%v", ok, err)
	}
}

func TestSetPassword_UnknownUser(t *testing.T) {
	s := openTemp(t)
	if err := s.SetPassword("nobody", "x"); err != ErrUserNotFound {
		t.Fatalf("err = %v, want ErrUserNotFound", err)
	}
}

func TestSetDisabled_UnknownUser(t *testing.T) {
	s := openTemp(t)
	if err := s.SetDisabled("nobody", true); err != ErrUserNotFound {
		t.Fatalf("err = %v, want ErrUserNotFound", err)
	}
}
