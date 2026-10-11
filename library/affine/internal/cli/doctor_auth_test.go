package cli

import "testing"

func TestClassifyCurrentUserProbe(t *testing.T) {
	cases := map[string]string{
		`{"data":{"currentUser":null}}`:                                  "invalid",
		`{"data":{"currentUser":{"id":"u1","email":"a@b.c"}}}`:           "valid",
		`{"errors":[{"message":"AUTHENTICATION_REQUIRED"}],"data":null}`: "invalid",
		`<html>`: "error",
	}
	for body, prefix := range cases {
		got := classifyCurrentUserProbe([]byte(body))
		if len(got) < len(prefix) || got[:len(prefix)] != prefix {
			t.Errorf("%s -> %q, want prefix %q", body, got, prefix)
		}
	}
	if doctorCredentialsErr(map[string]any{"credentials": classifyCurrentUserProbe([]byte(`{"data":{"currentUser":null}}`))}) == nil {
		t.Error("anonymous probe must fail doctor")
	}
	if doctorCredentialsErr(map[string]any{"credentials": "valid (currentUser a@b.c)"}) != nil {
		t.Error("valid probe must pass doctor")
	}
}
