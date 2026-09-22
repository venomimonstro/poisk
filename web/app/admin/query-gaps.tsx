"use client";

import { useCallback, useEffect, useState } from "react";

type Gap = {
  gap_id: number;
  representative_query?: string;
  state: string;
  demand_score: number;
  coverage_score: number;
  quality_score: number;
  freshness_score: number;
  spam_score: number;
  gap_score: number;
  independent_buckets: number;
  feedback_count: number;
  last_seen_at: string;
};
type Preview = {
  preview_token: string;
  expires_at: string;
  representative_query?: string;
  before: { gap_id: number; state: string };
  after: { gap_id: number; state: string };
  reason: string;
};

const card: React.CSSProperties = { border: "1px solid #e2e2e2", borderRadius: 12, padding: 16, background: "#fff" };
const button: React.CSSProperties = { padding: "8px 12px", border: 0, borderRadius: 8, cursor: "pointer", fontWeight: 700 };

async function readJSON<T>(url: string, init?: RequestInit): Promise<T> {
  const response = await fetch(url, { ...init, cache: "no-store", credentials: "same-origin" });
  const data = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(typeof data?.error === "string" ? data.error : `http_${response.status}`);
  return data as T;
}

export default function QueryGapsPanel({ csrf, operator }: { csrf: string; operator: boolean }) {
  const [state, setState] = useState("OPEN");
  const [rows, setRows] = useState<Gap[]>([]);
  const [error, setError] = useState("");
  const [preview, setPreview] = useState<Preview | null>(null);
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    try {
      const result = await readJSON<{ results: Gap[] }>(`/api/admin/query-gaps?state=${encodeURIComponent(state)}&limit=50`);
      setRows(result.results || []); setError("");
    } catch (e) { setError(e instanceof Error ? e.message : "query_gaps_unavailable"); }
  }, [state]);

  useEffect(() => { void load(); }, [load]);

  async function makePreview(gap: Gap) {
    if (!operator || busy) return;
    const action = gap.state === "SUPPRESSED" ? "REOPEN" : "SUPPRESS";
    const reason = window.prompt(action === "SUPPRESS" ? "Причина подавления" : "Причина возврата", "operator review")?.trim();
    if (!reason) return;
    setBusy(true); setError("");
    try {
      setPreview(await readJSON<Preview>("/api/admin/query-gaps/preview", {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf },
        body: JSON.stringify({ gap_id: gap.gap_id, action, reason }),
      }));
    } catch (e) { setError(e instanceof Error ? e.message : "preview_failed"); }
    finally { setBusy(false); }
  }

  async function apply() {
    if (!preview || busy) return;
    setBusy(true); setError("");
    try {
      await readJSON("/api/admin/query-gaps/apply", {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf },
        body: JSON.stringify({ preview_token: preview.preview_token }),
      });
      setPreview(null); await load();
    } catch (e) { setError(e instanceof Error ? e.message : "apply_failed"); }
    finally { setBusy(false); }
  }

  return <section style={{ ...card, display: "grid", gap: 12 }}>
    <div style={{ display: "flex", justifyContent: "space-between", gap: 12, alignItems: "center", flexWrap: "wrap" }}>
      <div><strong>Query Gaps</strong><div style={{ color: "#666", marginTop: 4 }}>Demand/Coverage/Quality/Freshness/Spam diagnostics</div></div>
      <div style={{ display: "flex", gap: 8 }}>
        <select value={state} onChange={e => setState(e.target.value)} style={{ padding: "8px 10px", borderRadius: 8, border: "1px solid #ccc" }}>
          <option value="OPEN">OPEN</option><option value="WATCH">WATCH</option><option value="SUPPRESSED">SUPPRESSED</option><option value="RESOLVED">RESOLVED</option><option value="">ALL</option>
        </select>
        <button style={button} onClick={() => void load()}>Обновить</button>
      </div>
    </div>
    {error && <div role="alert" style={{ color: "#991b1b" }}>{error}</div>}
    <div style={{ display: "grid", gap: 8 }}>
      {rows.map(gap => <div key={gap.gap_id} style={{ border: "1px solid #eee", borderRadius: 10, padding: 12, display: "grid", gap: 6 }}>
        <div style={{ display: "flex", justifyContent: "space-between", gap: 12, alignItems: "center" }}><strong>{gap.representative_query || `Gap #${gap.gap_id}`}</strong><span>{gap.state} · gap {gap.gap_score}/100</span></div>
        <div style={{ color: "#555", fontSize: 13 }}>demand {gap.demand_score} · coverage {gap.coverage_score} · quality {gap.quality_score} · freshness {gap.freshness_score} · spam {gap.spam_score} · buckets {gap.independent_buckets} · feedback {gap.feedback_count}</div>
        <div style={{ color: "#777", fontSize: 12 }}>last seen {new Date(gap.last_seen_at).toLocaleString()}</div>
        {operator && gap.state !== "RESOLVED" && <div><button style={{ ...button, background: gap.state === "SUPPRESSED" ? "#f3f4f6" : "#fff1f2", color: gap.state === "SUPPRESSED" ? "#111827" : "#991b1b" }} disabled={busy} onClick={() => void makePreview(gap)}>{gap.state === "SUPPRESSED" ? "Reopen" : "Suppress"}</button></div>}
      </div>)}
      {!rows.length && <div style={{ color: "#666" }}>Нет записей для выбранного состояния.</div>}
    </div>
    {preview && <div style={{ border: "1px solid #f0b4b4", borderRadius: 10, padding: 12, background: "#fffafa" }}>
      <strong>Подтвердите Query Gap mutation</strong>
      <div>{preview.representative_query || `Gap #${preview.before.gap_id}`}: {preview.before.state} → {preview.after.state}</div>
      <div>Причина: {preview.reason}</div>
      <div>Preview истекает: {new Date(preview.expires_at).toLocaleString()}</div>
      <div style={{ display: "flex", gap: 8, marginTop: 10 }}><button style={{ ...button, background: "#8b0000", color: "white" }} disabled={busy} onClick={() => void apply()}>Применить</button><button style={button} onClick={() => setPreview(null)}>Отмена</button></div>
    </div>}
  </section>;
}
