package httpapi

import (
	"context"
	"net/http"

	"github.com/venomimonstro/poisk/internal/identity"
)

type contextKey string
const authKey contextKey="mail_auth"
type authed struct{User identity.User;Session identity.Session}
func withAuth(ctx context.Context,a authed)context.Context{return context.WithValue(ctx,authKey,a)}
func authFrom(r *http.Request)(authed,bool){a,ok:=r.Context().Value(authKey).(authed);return a,ok}
