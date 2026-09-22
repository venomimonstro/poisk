package httpapi

import (
	"context"
	"net/http"
)

type authKey struct{}

func withAuth(ctx context.Context,a authed)context.Context{return context.WithValue(ctx,authKey{},a)}
func authFrom(r *http.Request)(authed,bool){a,ok:=r.Context().Value(authKey{}).(authed);return a,ok}
