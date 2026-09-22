package httpapi

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/venomimonstro/poisk/internal/identity"
	"github.com/venomimonstro/poisk/internal/webmaster"
)

const consumerSessionCookie = "poisk_session"

type portalContextKey string
const portalSessionKey portalContextKey = "consumer-session"

type PortalHandler struct {
	Base     Handler
	Identity *identity.Service
}

func (h PortalHandler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(h.RequireConsumer)
	r.Get("/sites", h.Base.ListSites)
	r.Get("/usage", h.Usage)
	r.With(h.RequireCSRF).Post("/sites", h.Base.AddSite)
	r.With(h.RequireCSRF).Post("/sites/{siteID}/verification", h.Base.BeginVerification)
	r.With(h.RequireCSRF).Post("/sites/{siteID}/verify", h.Base.CompleteVerification)
	r.With(h.RequireCSRF).Post("/sites/{siteID}/sitemaps", h.Base.SubmitSitemap)
	r.With(h.RequireCSRF).Post("/sites/{siteID}/urls", h.Base.SubmitURL)
	r.Get("/sites/{siteID}/url-status", h.Base.URLStatus)
	r.Get("/sites/{siteID}/metrics", h.Base.Metrics)
	return r
}

func (h PortalHandler) RequireConsumer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.Identity == nil || h.Identity.Repo == nil || h.Base.Service == nil {
			writeError(w, http.StatusServiceUnavailable, "webmaster_unavailable")
			return
		}
		cookie, err := r.Cookie(consumerSessionCookie)
		if err != nil || strings.TrimSpace(cookie.Value) == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		consumer, session, err := h.Identity.Authenticate(r.Context(), cookie.Value)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		webmasterID, err := h.Identity.EnsureWebmasterProfile(r.Context(), consumer)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "webmaster_profile_unavailable")
			return
		}
		ctx := context.WithValue(r.Context(), userContextKey, webmaster.User{ID: webmasterID, Email: consumer.Email, Status: "ACTIVE"})
		ctx = context.WithValue(ctx, portalSessionKey, session)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h PortalHandler) RequireCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, ok := r.Context().Value(portalSessionKey).(identity.Session)
		if !ok || !identity.VerifyCSRF(session, r.Header.Get("X-CSRF-Token")) {
			writeError(w, http.StatusForbidden, "csrf_failed")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h PortalHandler) Usage(w http.ResponseWriter,r *http.Request){
	user,ok:=currentUser(r);if !ok{writeError(w,http.StatusUnauthorized,"unauthorized");return}
	if h.Base.Billing==nil{writeJSON(w,http.StatusOK,map[string]any{"product":"WEBMASTER_PRO","paid":false,"plan_code":"FREE","usage":map[string]int64{},"limits":map[string]int64{"sites":3,"sitemaps_month":10,"url_requests_month":100}});return}
	accountID,err:=h.Base.Billing.AccountIDForUser(r.Context(),user.ID);if err!=nil{writeBillingError(w,err);return}
	usage,err:=h.Base.Billing.Usage(r.Context(),user.ID,accountID);if err!=nil{writeBillingError(w,err);return}
	now:=time.Now().UTC()
	sites,err:=h.Base.Billing.EffectiveQuota(r.Context(),accountID,"WEBMASTER_PRO","sites",3,now);if err!=nil{writeBillingError(w,err);return}
	sitemaps,err:=h.Base.Billing.EffectiveQuota(r.Context(),accountID,"WEBMASTER_PRO","sitemaps_month",10,now);if err!=nil{writeBillingError(w,err);return}
	urls,err:=h.Base.Billing.EffectiveQuota(r.Context(),accountID,"WEBMASTER_PRO","url_requests_month",100,now);if err!=nil{writeBillingError(w,err);return}
	plan:="FREE";paid:=sites.Paid||sitemaps.Paid||urls.Paid;if paid{if sites.PlanCode!=""{plan=sites.PlanCode}else if sitemaps.PlanCode!=""{plan=sitemaps.PlanCode}else if urls.PlanCode!=""{plan=urls.PlanCode}}
	writeJSON(w,http.StatusOK,map[string]any{"product":"WEBMASTER_PRO","paid":paid,"plan_code":plan,"usage":usage,"limits":map[string]int64{"sites":sites.Limit,"sitemaps_month":sitemaps.Limit,"url_requests_month":urls.Limit},"period_end":urls.PeriodEnd})
}
