package admin

import (
	"errors"
	"testing"
)

func TestRequireRoleReadOnlyAndOperatorBoundaries(t *testing.T) {
	svc := Service{}
	cases := []struct {
		name string
		role string
		allowed []string
		wantForbidden bool
	}{
		{name:"viewer read",role:"VIEWER",allowed:[]string{"VIEWER","ANALYST","OPERATOR"}},
		{name:"viewer mutate",role:"VIEWER",allowed:[]string{"OPERATOR"},wantForbidden:true},
		{name:"analyst read",role:"ANALYST",allowed:[]string{"VIEWER","ANALYST","OPERATOR"}},
		{name:"analyst mutate",role:"ANALYST",allowed:[]string{"OPERATOR"},wantForbidden:true},
		{name:"operator mutate",role:"OPERATOR",allowed:[]string{"OPERATOR"}},
		{name:"superadmin inherits operator",role:"SUPERADMIN",allowed:[]string{"OPERATOR"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := svc.RequireRole(Session{Role:tc.role}, tc.allowed...)
			if tc.wantForbidden {
				if !errors.Is(err, ErrForbidden) { t.Fatalf("err=%v", err) }
				return
			}
			if err != nil { t.Fatalf("err=%v", err) }
		})
	}
}
