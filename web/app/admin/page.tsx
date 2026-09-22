"use client";

import { FormEvent, useCallback, useEffect, useState } from "react";
import QueryGapsPanel from "./query-gaps";

type AdminMe = { admin_id: number; email: string; role: string; expires_at: string };
type Queue = { ready: number; leased: number; retry: number; dead: number };
type Ops = {
  crawl: Queue;
  outbox: Array<Queue & { entity_type: string }>;
  imports: { organizations_active: number; addresses_active: number; webmaster_sitemaps_active: number };
  runtime: { goroutines: number; heap_alloc_bytes: number; heap_sys_bytes: number };
  resources: { state: string; disk_used_pct: number; memory_used_pct: number; updated_at: string };
};
type Owner = {
  users: { total: number; active: number; locked: number; disabled: number };
  webmaster: { sites: number; verified: number; pending: number; suspended: number };
  directory: { organizations: number; addresses: number };
  capacity?: { snapshot_id: number; measured_documents: number; measured_at: string; bottlenecks: Array<{ code?: string; severity?: string }> };
};
type Diagnostics = {
  crawler: { domains: number; urls: number; ready: number; leased: number; retry: number; dead: number; success_24h: number; retry_24h: number; dead_24h: number; blocked_24h: number };
  index: { ready: number; leased: number; retry: number; dead: number; processed_24h: number };
  demand: { watch: number; open: number; resolved: number; suppressed: number; active_feedback: number };
  webmaster: { users: number; sites: number; pending_verifications: number; queued_sitemaps: number; queued_url_requests: number };
  billing: { accounts: number; active_subscriptions: number; past_due_subscriptions: number; open_invoices: number; failed_invoices: number; unprocessed_payment_events: number };
};
type Domain = { domain_id: number; host: string; status: string; policy: string; trust_level: number; quality_score: number; demand_score: number };
type Preview = { preview_token: string; expires_at: string; host: string; before: { domain_id: number; status: string; policy: string }; after: { domain_id: number; status: string; policy: string } };

const card: React.CSSProperties = { border: "1px solid #e2e2e2", borderRadius: 12, padding: 16, background: "#fff" };
const input: React.CSSProperties = { width: "100%", boxSizing: "border-box", padding: "10px 12px", border: "1px solid #ccc", borderRadius: 8, font: "inherit" };
const button: React.CSSProperties = { padding: "10px 14px", border: 0, borderRadius: 8, cursor: "pointer", fontWeight: 700 };

async function readJSON<T>(url: string, init?: RequestInit): Promise<T> {
  const response = await fetch(url, { ...init, cache: "no-store", credentials: "same-origin" });
  const data = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(typeof data?.error === "string" ? data.error : `http_${response.status}`);
  return data as T;
}

export default function AdminPage() {
  const [me, setMe] = useState<AdminMe | null>(null);
  const [csrf, setCSRF] = useState("");
  const [ops, setOps] = useState<Ops | null>(null);
  const [owner, setOwner] = useState<Owner | null>(null);
  const [diagnostics, setDiagnostics] = useState<Diagnostics | null>(null);
  const [error, setError] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [factor, setFactor] = useState("");
  const [query, setQuery] = useState("");
  const [domains, setDomains] = useState<Domain[]>([]);
  const [selected, setSelected] = useState<Domain | null>(null);
  const [status, setStatus] = useState("ACTIVE");
  const [policy, setPolicy] = useState("ALLOW");
  const [preview, setPreview] = useState<Preview | null>(null);

  const refresh = useCallback(async () => {
    const current = await readJSON<AdminMe>("/api/admin/me");
    const token = await readJSON<{ csrf_token: string }>("/api/admin/csrf");
    const [snapshot, diag] = await Promise.all([readJSON<Ops>("/api/admin/status"), readJSON<Diagnostics>("/api/admin/diagnostics")]);
    const ownerSnapshot = current.role === "SUPERADMIN" ? await readJSON<Owner>("/api/admin/owner") : null;
    setMe(current); setCSRF(token.csrf_token); setOps(snapshot); setDiagnostics(diag); setOwner(ownerSnapshot); setError("");
  }, []);

  useEffect(() => { void refresh().catch(() => setMe(null)); }, [refresh]);

  async function login(event: FormEvent) {
    event.preventDefault(); setError("");
    try {
      const result = await readJSON<{ csrf_token: string; admin: { id: number; email: string; role: string }; expires_at: string }>("/api/admin/login", {
        method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ email, password, second_factor: factor }),
      });
      setCSRF(result.csrf_token); setMe({ admin_id: result.admin.id, email: result.admin.email, role: result.admin.role, expires_at: result.expires_at });
      setPassword(""); setFactor("");
      const [snapshot, diag] = await Promise.all([readJSON<Ops>("/api/admin/status"), readJSON<Diagnostics>("/api/admin/diagnostics")]);
      setOps(snapshot); setDiagnostics(diag);
      setOwner(result.admin.role === "SUPERADMIN" ? await readJSON<Owner>("/api/admin/owner") : null);
    } catch (e) { setError(e instanceof Error ? e.message : "login_failed"); }
  }

  async function logout() {
    try { await readJSON("/api/admin/logout", { method: "POST", headers: { "X-CSRF-Token": csrf } }); } catch { /* cookie is cleared on success only */ }
    setMe(null); setCSRF(""); setOps(null); setOwner(null); setDiagnostics(null); setDomains([]); setPreview(null);
  }

  async function searchDomains() {
    try {
      const result = await readJSON<{ results: Domain[] }>(`/api/admin/domains?q=${encodeURIComponent(query)}&limit=25`);
      setDomains(result.results); setError("");
    } catch (e) { setError(e instanceof Error ? e.message : "domains_unavailable"); }
  }

  async function makePreview() {
    if (!selected) return;
    try {
      const result = await readJSON<Preview>("/api/admin/domains/preview", {
        method: "POST", headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf }, body: JSON.stringify({ domain_id: selected.domain_id, status, policy }),
      });
      setPreview(result); setError("");
    } catch (e) { setError(e instanceof Error ? e.message : "preview_failed"); }
  }

  async function applyPreview() {
    if (!preview) return;
    try {
      await readJSON("/api/admin/domains/apply", {
        method: "POST", headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf }, body: JSON.stringify({ preview_token: preview.preview_token }),
      });
      setPreview(null); setSelected(null); await searchDomains(); await refresh();
    } catch (e) { setError(e instanceof Error ? e.message : "apply_failed"); }
  }

  if (!me) return <main style={{ maxWidth: 440, margin: "80px auto", padding: 24 }}>
    <h1>Poisk Admin</h1><p>Доступ только для созданных оператором администраторов с 2FA.</p>
    <form onSubmit={login} style={{ ...card, display: "grid", gap: 12 }}>
      <input style={input} type="email" autoComplete="username" placeholder="Email" value={email} onChange={e => setEmail(e.target.value)} required />
      <input style={input} type="password" autoComplete="current-password" placeholder="Пароль" value={password} onChange={e => setPassword(e.target.value)} required />
      <input style={input} inputMode="numeric" autoComplete="one-time-code" placeholder="TOTP или recovery code" value={factor} onChange={e => setFactor(e.target.value)} required />
      <button style={{ ...button, background: "#111", color: "#fff" }} type="submit">Войти</button>
      {error && <div role="alert">{error}</div>}
    </form>
  </main>;

  const operator = me.role === "OPERATOR" || me.role === "SUPERADMIN";
  return <main style={{ maxWidth: 1180, margin: "32px auto", padding: 24, display: "grid", gap: 18 }}>
    <header style={{ display: "flex", justifyContent: "space-between", gap: 16, alignItems: "center" }}>
      <div><h1 style={{ margin: 0 }}>Poisk Admin</h1><small>{me.email} · {me.role}</small></div>
      <div style={{ display: "flex", gap: 8 }}><button style={button} onClick={() => void refresh()}>Обновить</button><button style={{ ...button, background: "#111", color: "white" }} onClick={() => void logout()}>Выйти</button></div>
    </header>
    {error && <div role="alert" style={{ ...card, borderColor: "#b00" }}>{error}</div>}
    {owner && <section style={{ ...card, display: "grid", gap: 14 }}>
      <div><strong>Owner overview</strong><div style={{ color: "#666", marginTop: 4 }}>Read-only коммерческий и продуктовый срез</div></div>
      <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(190px,1fr))", gap: 10 }}>
        <Metric label="Пользователи" value={owner.users.total} note={`active ${owner.users.active} · locked ${owner.users.locked} · disabled ${owner.users.disabled}`} />
        <Metric label="Webmaster sites" value={owner.webmaster.sites} note={`verified ${owner.webmaster.verified} · pending ${owner.webmaster.pending}`} />
        <Metric label="Организации" value={owner.directory.organizations} note="Активные карточки каталога" />
        <Metric label="Адреса" value={owner.directory.addresses} note="Активный адресный индекс" />
      </div>
      {owner.capacity ? <div style={{ borderTop: "1px solid #eee", paddingTop: 12 }}><strong>Последний Capacity snapshot #{owner.capacity.snapshot_id}</strong><div>Документов: {owner.capacity.measured_documents.toLocaleString("ru-RU")} · {new Date(owner.capacity.measured_at).toLocaleString()}</div><div>Проблемы: {owner.capacity.bottlenecks?.length ? owner.capacity.bottlenecks.map(b => `${b.code || "unknown"}${b.severity ? ` (${b.severity})` : ""}`).join(", ") : "не зафиксированы"}</div></div> : <div style={{ borderTop: "1px solid #eee", paddingTop: 12 }}>Capacity benchmark ещё не зафиксирован.</div>}
    </section>}
    {ops && <section style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(220px,1fr))", gap: 12 }}>
      <div style={card}><strong>Resources</strong><div>State: {ops.resources.state}</div><div>Disk: {ops.resources.disk_used_pct.toFixed(1)}%</div><div>Memory: {ops.resources.memory_used_pct.toFixed(1)}%</div></div>
      <div style={card}><strong>Crawler</strong><div>Ready {ops.crawl.ready} · Leased {ops.crawl.leased}</div><div>Retry {ops.crawl.retry} · Dead {ops.crawl.dead}</div></div>
      <div style={card}><strong>Imports</strong><div>Organizations {ops.imports.organizations_active}</div><div>Addresses {ops.imports.addresses_active}</div><div>Sitemaps {ops.imports.webmaster_sitemaps_active}</div></div>
      <div style={card}><strong>Runtime</strong><div>Goroutines {ops.runtime.goroutines}</div><div>Heap {(ops.runtime.heap_alloc_bytes / 1048576).toFixed(1)} MB</div></div>
    </section>}
    {diagnostics && <section style={{ ...card, display: "grid", gap: 14 }}>
      <div><strong>Subsystem diagnostics</strong><div style={{ color: "#666", marginTop: 4 }}>Read-only operational state</div></div>
      <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(210px,1fr))", gap: 12 }}>
        <div style={card}><strong>Crawler</strong><div>{diagnostics.crawler.domains} domains · {diagnostics.crawler.urls} URLs</div><div>queue R {diagnostics.crawler.ready} / L {diagnostics.crawler.leased} / retry {diagnostics.crawler.retry} / dead {diagnostics.crawler.dead}</div><div>24h success {diagnostics.crawler.success_24h} · dead {diagnostics.crawler.dead_24h} · blocked {diagnostics.crawler.blocked_24h}</div></div>
        <div style={card}><strong>Index</strong><div>ready {diagnostics.index.ready} · leased {diagnostics.index.leased}</div><div>retry {diagnostics.index.retry} · dead {diagnostics.index.dead}</div><div>processed 24h {diagnostics.index.processed_24h}</div></div>
        <div style={card}><strong>Demand</strong><div>open {diagnostics.demand.open} · watch {diagnostics.demand.watch}</div><div>resolved {diagnostics.demand.resolved} · suppressed {diagnostics.demand.suppressed}</div><div>active feedback {diagnostics.demand.active_feedback}</div></div>
        <div style={card}><strong>Webmaster</strong><div>{diagnostics.webmaster.users} users · {diagnostics.webmaster.sites} sites</div><div>verification pending {diagnostics.webmaster.pending_verifications}</div><div>sitemaps {diagnostics.webmaster.queued_sitemaps} · URL ops {diagnostics.webmaster.queued_url_requests}</div></div>
        <div style={card}><strong>Billing</strong><div>{diagnostics.billing.accounts} active accounts · {diagnostics.billing.active_subscriptions} subscriptions</div><div>past due {diagnostics.billing.past_due_subscriptions} · open invoices {diagnostics.billing.open_invoices}</div><div>failed invoices {diagnostics.billing.failed_invoices} · unprocessed events {diagnostics.billing.unprocessed_payment_events}</div></div>
      </div>
    </section>}
    {ops?.outbox?.length ? <section style={card}><strong>Index outbox</strong>{ops.outbox.map(item => <div key={item.entity_type}>{item.entity_type}: ready {item.ready}, leased {item.leased}, retry {item.retry}, dead {item.dead}</div>)}</section> : null}
    <QueryGapsPanel csrf={csrf} operator={operator} />
    <section style={card}>
      <h2>Domains</h2>
      <div style={{ display: "flex", gap: 8 }}><input style={input} placeholder="host" value={query} onChange={e => setQuery(e.target.value)} onKeyDown={e => { if (e.key === "Enter") void searchDomains(); }} /><button style={button} onClick={() => void searchDomains()}>Найти</button></div>
      <div style={{ display: "grid", gap: 6, marginTop: 12 }}>{domains.map(domain => <button key={domain.domain_id} style={{ ...button, textAlign: "left", background: selected?.domain_id === domain.domain_id ? "#ddd" : "#f4f4f4" }} onClick={() => { setSelected(domain); setStatus(domain.status); setPolicy(domain.policy); setPreview(null); }}>{domain.host} · {domain.status}/{domain.policy}</button>)}</div>
    </section>
    {selected && operator && <section style={card}>
      <h2>Изменение политики: {selected.host}</h2>
      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr auto", gap: 8 }}>
        <select style={input} value={status} onChange={e => { setStatus(e.target.value); setPreview(null); }}><option>ACTIVE</option><option>PAUSED</option><option>DISABLED</option></select>
        <select style={input} value={policy} onChange={e => { setPolicy(e.target.value); setPreview(null); }}><option>ALLOW</option><option>LIMITED</option><option>REVIEW</option><option>BLOCK</option></select>
        <button style={button} onClick={() => void makePreview()}>Preview</button>
      </div>
      {preview && <div style={{ marginTop: 12, padding: 12, border: "1px solid #d99", borderRadius: 8 }}>
        <div><strong>До:</strong> {preview.before.status}/{preview.before.policy}</div><div><strong>После:</strong> {preview.after.status}/{preview.after.policy}</div><div>Preview истекает: {new Date(preview.expires_at).toLocaleString()}</div>
        <button style={{ ...button, marginTop: 10, background: "#8b0000", color: "white" }} onClick={() => void applyPreview()}>Подтвердить изменение</button>
      </div>}
    </section>}
  </main>;
}

function Metric({ label, value, note }: { label: string; value: number; note: string }) {
  return <div><small>{label}</small><div style={{ fontSize: 28, fontWeight: 800 }}>{value.toLocaleString("ru-RU")}</div><div>{note}</div></div>;
}
