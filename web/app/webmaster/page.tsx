"use client";

import { FormEvent, useEffect, useMemo, useState } from "react";

type User = { user_id: number; email: string; status: string };
type Site = { site_id: number; domain_id: number; origin: string; host: string; status: string; verified_at?: string; verification_method?: string };
type Metrics = { impressions: number; clicks: number; answer_citations: number; ctr: number };
type Challenge = { token: string; instruction: string; verification: { method: string; expires_at: string } };

type ApiError = { error?: string };

async function api<T>(url: string, init?: RequestInit): Promise<T> {
  const response = await fetch(url, { ...init, cache: "no-store", credentials: "same-origin" });
  const text = await response.text();
  let data: unknown = {};
  if (text) {
    try { data = JSON.parse(text); } catch { data = {}; }
  }
  if (!response.ok) {
    const code = (data as ApiError).error || `http_${response.status}`;
    throw new Error(code);
  }
  return data as T;
}

function messageFor(code: string) {
  const messages: Record<string, string> = {
    invalid_credentials: "Неверный email или пароль",
    invalid_request: "Проверьте введённые данные",
    conflict: "Такой аккаунт или сайт уже существует",
    unauthorized: "Нужно войти в аккаунт",
    csrf_failed: "Сессия обновилась. Повторите действие",
    site_not_verified: "Сначала подтвердите владение сайтом",
    invalid_site_id: "Некорректный сайт",
    site_limit_reached: "Достигнут лимит сайтов",
    usage_limit_reached: "Достигнут лимит операций",
    webmaster_backend_error: "Сервис вебмастера временно недоступен",
    account_backend_error: "Сервис аккаунта временно недоступен",
  };
  return messages[code] || "Не удалось выполнить действие";
}

export default function WebmasterPage() {
  const [user, setUser] = useState<User | null>(null);
  const [csrf, setCsrf] = useState("");
  const [sites, setSites] = useState<Site[]>([]);
  const [selected, setSelected] = useState<number | null>(null);
  const [mode, setMode] = useState<"login" | "register">("login");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [origin, setOrigin] = useState("");
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState("");
  const [error, setError] = useState("");
  const [challenge, setChallenge] = useState<Challenge | null>(null);
  const [verifyMethod, setVerifyMethod] = useState("DNS_TXT");
  const [metrics, setMetrics] = useState<Metrics | null>(null);
  const [sitemap, setSitemap] = useState("");
  const [urlToSubmit, setUrlToSubmit] = useState("");
  const [urlOperation, setUrlOperation] = useState("SUBMIT");
  const [urlStatus, setUrlStatus] = useState<Record<string, unknown> | null>(null);

  const currentSite = useMemo(() => sites.find((site) => site.site_id === selected) || null, [sites, selected]);

  async function refreshCsrf() {
    const result = await api<{ csrf_token: string }>("/api/account/csrf");
    setCsrf(result.csrf_token);
    return result.csrf_token;
  }

  async function loadSites() {
    const result = await api<{ sites: Site[] }>("/api/portal/webmaster/sites");
    setSites(result.sites || []);
    setSelected((previous) => previous && result.sites?.some((site) => site.site_id === previous) ? previous : result.sites?.[0]?.site_id || null);
  }

  async function bootstrap() {
    try {
      const me = await api<{ user: User }>("/api/account/me");
      setUser(me.user);
      await refreshCsrf();
      await loadSites();
    } catch {
      setUser(null);
      setCsrf("");
      setSites([]);
    }
  }

  useEffect(() => { void bootstrap(); }, []);

  function clearMessages() { setError(""); setNotice(""); }

  async function authSubmit(event: FormEvent) {
    event.preventDefault();
    clearMessages();
    setBusy(true);
    try {
      const result = await api<{ user: User; csrf_token: string }>(`/api/account/${mode}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email, password }),
      });
      setUser(result.user);
      setCsrf(result.csrf_token);
      setPassword("");
      await loadSites();
    } catch (e) { setError(messageFor((e as Error).message)); }
    finally { setBusy(false); }
  }

  async function mutation<T>(url: string, body: unknown): Promise<T> {
    let token = csrf;
    if (!token) token = await refreshCsrf();
    try {
      return await api<T>(url, { method: "POST", headers: { "Content-Type": "application/json", "X-CSRF-Token": token }, body: JSON.stringify(body) });
    } catch (e) {
      if ((e as Error).message === "csrf_failed") {
        token = await refreshCsrf();
        return api<T>(url, { method: "POST", headers: { "Content-Type": "application/json", "X-CSRF-Token": token }, body: JSON.stringify(body) });
      }
      throw e;
    }
  }

  async function addSite(event: FormEvent) {
    event.preventDefault(); clearMessages(); setBusy(true);
    try {
      const site = await mutation<Site>("/api/portal/webmaster/sites", { origin });
      setOrigin(""); setNotice("Сайт добавлен"); await loadSites(); setSelected(site.site_id);
    } catch (e) { setError(messageFor((e as Error).message)); }
    finally { setBusy(false); }
  }

  async function beginVerification() {
    if (!currentSite) return; clearMessages(); setBusy(true);
    try {
      const result = await mutation<Challenge>(`/api/portal/webmaster/sites/${currentSite.site_id}/verification`, { method: verifyMethod });
      setChallenge(result); setNotice("Код подтверждения создан. Установите его на сайте и нажмите «Проверить».");
    } catch (e) { setError(messageFor((e as Error).message)); }
    finally { setBusy(false); }
  }

  async function verifySite() {
    if (!currentSite || !challenge) return; clearMessages(); setBusy(true);
    try {
      await mutation(`/api/portal/webmaster/sites/${currentSite.site_id}/verify`, { method: challenge.verification.method, token: challenge.token });
      setChallenge(null); setNotice("Владение сайтом подтверждено"); await loadSites();
    } catch (e) { setError(messageFor((e as Error).message)); }
    finally { setBusy(false); }
  }

  async function loadMetrics() {
    if (!currentSite) return; clearMessages();
    try {
      const to = new Date(); const from = new Date(); from.setDate(from.getDate() - 30);
      const fmt = (date: Date) => date.toISOString().slice(0, 10);
      setMetrics(await api<Metrics>(`/api/portal/webmaster/sites/${currentSite.site_id}/metrics?from=${fmt(from)}&to=${fmt(to)}`));
    } catch (e) { setError(messageFor((e as Error).message)); }
  }

  async function submitSitemap(event: FormEvent) {
    event.preventDefault(); if (!currentSite) return; clearMessages(); setBusy(true);
    try { await mutation(`/api/portal/webmaster/sites/${currentSite.site_id}/sitemaps`, { url: sitemap }); setSitemap(""); setNotice("Sitemap принят в обработку"); }
    catch (e) { setError(messageFor((e as Error).message)); }
    finally { setBusy(false); }
  }

  async function submitUrl(event: FormEvent) {
    event.preventDefault(); if (!currentSite) return; clearMessages(); setBusy(true);
    try { await mutation(`/api/portal/webmaster/sites/${currentSite.site_id}/urls`, { url: urlToSubmit, operation: urlOperation }); setNotice("URL принят в обработку"); }
    catch (e) { setError(messageFor((e as Error).message)); }
    finally { setBusy(false); }
  }

  async function checkUrl() {
    if (!currentSite || !urlToSubmit) return; clearMessages();
    try { setUrlStatus(await api<Record<string, unknown>>(`/api/portal/webmaster/sites/${currentSite.site_id}/url-status?url=${encodeURIComponent(urlToSubmit)}`)); }
    catch (e) { setError(messageFor((e as Error).message)); }
  }

  async function logout() {
    clearMessages(); setBusy(true);
    try {
      const token = csrf || await refreshCsrf();
      await api("/api/account/logout", { method: "POST", headers: { "X-CSRF-Token": token } });
      setUser(null); setSites([]); setCsrf(""); setSelected(null); setMetrics(null); setChallenge(null);
    } catch (e) { setError(messageFor((e as Error).message)); }
    finally { setBusy(false); }
  }

  if (!user) {
    return <main className="wmAuthPage">
      <a className="wmBrand" href="/">Поиск</a>
      <section className="wmAuthCard">
        <h1>Вебмастер</h1>
        <p>Добавляйте сайты, подтверждайте владение и контролируйте индексирование.</p>
        <div className="wmTabs">
          <button className={mode === "login" ? "active" : ""} onClick={() => setMode("login")}>Войти</button>
          <button className={mode === "register" ? "active" : ""} onClick={() => setMode("register")}>Создать аккаунт</button>
        </div>
        <form onSubmit={authSubmit} className="wmStack">
          <input type="email" autoComplete="email" placeholder="Email" value={email} onChange={(e) => setEmail(e.target.value)} required />
          <input type="password" autoComplete={mode === "login" ? "current-password" : "new-password"} placeholder="Пароль, минимум 12 символов" value={password} onChange={(e) => setPassword(e.target.value)} minLength={12} required />
          <button className="wmPrimary" disabled={busy}>{busy ? "Подождите…" : mode === "login" ? "Войти" : "Создать аккаунт"}</button>
        </form>
        {error && <div className="wmError">{error}</div>}
      </section>
    </main>;
  }

  return <main className="wmPage">
    <header className="wmHeader">
      <a className="wmBrand" href="/">Поиск</a>
      <nav><a href="/">Поиск</a><a href="/map">Карты</a><span>{user.email}</span><button onClick={logout} disabled={busy}>Выйти</button></nav>
    </header>
    <div className="wmLayout">
      <aside className="wmSidebar">
        <h2>Мои сайты</h2>
        <form onSubmit={addSite} className="wmStack compact">
          <input placeholder="https://example.ru" value={origin} onChange={(e) => setOrigin(e.target.value)} required />
          <button className="wmPrimary" disabled={busy}>Добавить сайт</button>
        </form>
        <div className="wmSiteList">
          {sites.map((site) => <button key={site.site_id} className={site.site_id === selected ? "selected" : ""} onClick={() => { setSelected(site.site_id); setChallenge(null); setMetrics(null); setUrlStatus(null); }}>
            <strong>{site.host}</strong><span>{site.status === "VERIFIED" ? "Подтверждён" : "Нужно подтвердить"}</span>
          </button>)}
          {!sites.length && <p className="wmMuted">Добавьте первый сайт.</p>}
        </div>
      </aside>
      <section className="wmContent">
        {error && <div className="wmError">{error}</div>}
        {notice && <div className="wmNotice">{notice}</div>}
        {!currentSite ? <div className="wmEmpty"><h1>Добавьте сайт</h1><p>После добавления подтвердите владение и отправьте sitemap.</p></div> : <>
          <div className="wmTitleRow"><div><h1>{currentSite.host}</h1><p>{currentSite.origin}</p></div><span className={`wmStatus ${currentSite.status.toLowerCase()}`}>{currentSite.status === "VERIFIED" ? "Подтверждён" : currentSite.status}</span></div>
          {currentSite.status !== "VERIFIED" && <section className="wmPanel">
            <h2>1. Подтвердите владение</h2>
            <p>Выберите способ, установите проверочный код и запустите проверку.</p>
            <div className="wmInline">
              <select value={verifyMethod} onChange={(e) => { setVerifyMethod(e.target.value); setChallenge(null); }}><option value="DNS_TXT">DNS TXT</option><option value="HTML_FILE">HTML-файл</option><option value="META_TAG">Meta-тег</option></select>
              <button className="wmSecondary" onClick={beginVerification} disabled={busy}>Получить код</button>
            </div>
            {challenge && <div className="wmChallenge"><code>{challenge.instruction}</code><button className="wmPrimary" onClick={verifySite} disabled={busy}>Проверить владение</button></div>}
          </section>}
          <section className="wmPanel">
            <div className="wmPanelHead"><div><h2>Статистика поиска</h2><p>Показы, клики и упоминания в ответах за последние 30 дней.</p></div><button className="wmSecondary" onClick={loadMetrics}>Обновить</button></div>
            {metrics ? <div className="wmMetrics"><div><span>Показы</span><strong>{metrics.impressions.toLocaleString("ru-RU")}</strong></div><div><span>Клики</span><strong>{metrics.clicks.toLocaleString("ru-RU")}</strong></div><div><span>CTR</span><strong>{(metrics.ctr * 100).toFixed(1)}%</strong></div><div><span>Ответы</span><strong>{metrics.answer_citations.toLocaleString("ru-RU")}</strong></div></div> : <p className="wmMuted">Нажмите «Обновить», чтобы загрузить данные.</p>}
          </section>
          <section className="wmPanel">
            <h2>Индексирование</h2>
            <div className="wmGrid2">
              <form onSubmit={submitSitemap} className="wmStack"><h3>Sitemap</h3><input placeholder={`${currentSite.origin}/sitemap.xml`} value={sitemap} onChange={(e) => setSitemap(e.target.value)} required /><button className="wmPrimary" disabled={busy || currentSite.status !== "VERIFIED"}>Отправить sitemap</button></form>
              <form onSubmit={submitUrl} className="wmStack"><h3>Отдельный URL</h3><input placeholder={`${currentSite.origin}/page`} value={urlToSubmit} onChange={(e) => setUrlToSubmit(e.target.value)} required /><select value={urlOperation} onChange={(e) => setUrlOperation(e.target.value)}><option value="SUBMIT">Добавить</option><option value="REINDEX">Переобойти</option><option value="DELETE">Удалить из индекса</option></select><div className="wmInline"><button className="wmPrimary" disabled={busy || currentSite.status !== "VERIFIED"}>Отправить</button><button type="button" className="wmSecondary" onClick={checkUrl}>Проверить статус</button></div></form>
            </div>
            {urlStatus && <pre className="wmResult">{JSON.stringify(urlStatus, null, 2)}</pre>}
          </section>
        </>}
      </section>
    </div>
  </main>;
}
