package security

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBasicGrant_Authenticate(t *testing.T) {
	grant, err := NewBasicGrant("alice:s3cret")
	if err != nil {
		t.Fatalf("NewBasicGrant: %v", err)
	}

	ok := httptest.NewRequest(http.MethodGet, "/", nil)
	ok.SetBasicAuth("alice", "s3cret")
	if _, admitted := grant.Authenticate(ok); !admitted {
		t.Fatal("Authenticate(correct credentials) = false, want true")
	}

	badPass := httptest.NewRequest(http.MethodGet, "/", nil)
	badPass.SetBasicAuth("alice", "wrong")
	if _, admitted := grant.Authenticate(badPass); admitted {
		t.Fatal("Authenticate(wrong password) = true, want false")
	}

	badUser := httptest.NewRequest(http.MethodGet, "/", nil)
	badUser.SetBasicAuth("mallory", "s3cret")
	if _, admitted := grant.Authenticate(badUser); admitted {
		t.Fatal("Authenticate(wrong username) = true, want false")
	}

	none := httptest.NewRequest(http.MethodGet, "/", nil)
	if _, admitted := grant.Authenticate(none); admitted {
		t.Fatal("Authenticate(no Authorization header) = true, want false")
	}
}

func TestNewBasicGrant_RequiresColon(t *testing.T) {
	if _, err := NewBasicGrant("nocolon"); err == nil {
		t.Fatal("NewBasicGrant(no colon) = nil error, want error")
	}
	if _, err := NewBasicGrant(":pass"); err == nil {
		t.Fatal("NewBasicGrant(empty user) = nil error, want error")
	}
}
