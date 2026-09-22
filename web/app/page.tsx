"use client";

import { FormEvent, useRef, useState } from "react";

type SearchResult = {
  id: number;
  score: number;
  title: string;
  url: string;
  host: string;
  lang?: string;
  snippet: string;
};

type SearchResponse = {
  query: string;
  normalized: string;
  used_query: string;
  total: number;
  took_ms: number;
  results: SearchResult[];
  cached: boolean;
};

type AnswerSource = { id: number; title: string; url: string; host: string };
type AnswerClaim = { text: string; source_ids: number[] };
type AnswerResponse = {
  available: boolean;
  answer?: string;
  claims?: AnswerClaim[];
  sources?: AnswerSource[];
  confidence: number;
  fallback_reason?: string;
};

const apiBase = (process.env.NEXT_PUBLIC_API_BASE_URL || "").replace(/\/$/, "");

export default function HomePage() {
  const [query, setQuery] = useState("");
  const [data, setData] = useState<SearchResponse | null>(null);
  const [answer, setAnswer] = useState<AnswerResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [answerLoading, setAnswerLoading] = useState(false);
  const [error, setError] = useState("");
  const requestEpoch = useRef(0);

  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const q = query.trim();
    if (!q || loading) return;
    const epoch = ++requestEpoch.current;
    setLoading(true);
    setAnswerLoading(false);
    setAnswer(null);
    setError("");
    try {
      const controller = new AbortController();
      const timer = window.setTimeout(() => controller.abort(), 5000);
      try {
        const response = await fetch(`${apiBase}/api/search?q=${encodeURIComponent(q)}&limit=10`, { signal: controller.signal, headers: { Accept: "application/json" } });
        const payload = await response.json().catch(() => null);
        if (!response.ok) throw new Error(payload?.error || "search_failed");
        if (epoch !== requestEpoch.current) return;
        const searchPayload = payload as SearchResponse;
        setData(searchPayload);
        if (searchPayload.results.length > 0) void loadAnswer(q, epoch);
      } finally { window.clearTimeout(timer); }
    } catch (err) {
      if (epoch !== requestEpoch.current) return;
      setData(null); setAnswer(null);
      setError(err instanceof DOMException && err.name === "AbortError" ? "Поиск занял слишком много времени. Попробуйте ещё раз." : "Не удалось выполнить поиск. Попробуйте ещё раз.");
    } finally { if (epoch === requestEpoch.current) setLoading(false); }
  }

  async function loadAnswer(q: string, epoch: number) {
    if (epoch !== requestEpoch.current) return;
    setAnswerLoading(true);
    const controller = new AbortController();
    const timer = window.setTimeout(() => controller.abort(), 4500);
    try {
      const response = await fetch(`${apiBase}/api/answer?q=${encodeURIComponent(q)}`, { signal: controller.signal, headers: { Accept: "application/json" } });
      if (!response.ok || epoch !== requestEpoch.current) return;
      const payload = (await response.json()) as AnswerResponse;
      if (epoch === requestEpoch.current) setAnswer(payload.available ? payload : null);
    } catch { if (epoch === requestEpoch.current) setAnswer(null); }
    finally { window.clearTimeout(timer); if (epoch === requestEpoch.current) setAnswerLoading(false); }
  }

  const hasResults = Boolean(data?.results?.length);
  return (
    <main className={hasResults || data || error ? "shell shellResults" : "shell"}>
      <section className={hasResults || data || error ? "hero heroCompact" : "hero"}>
        <div className="brand">ПОИСК</div>
        {!hasResults && !data && !error && <><h1>Поиск, который не заставляет искать ответ.</h1><p>Быстрый независимый поиск с ответами только по найденным источникам.</p></>}
        <form className="search" onSubmit={onSubmit}>
          <input aria-label="Поисковый запрос" placeholder="Найдите или спросите что угодно" value={query} onChange={(event) => setQuery(event.target.value)} maxLength={256} autoComplete="off" />
          <button type="submit" disabled={loading || query.trim().length === 0}>{loading ? "Ищем…" : "Найти"}</button>
        </form>
        <nav className="productLinks" aria-label="Сервисы Поиска">
          <a href="/map">Карты</a>
          <a href="/webmaster">Вебмастер</a>
          <a href="/data">Данные</a>
        </nav>
      </section>
      {error && <div className="searchState searchError" role="alert">{error}</div>}
      {data && (
        <section className="serp" aria-live="polite">
          {answerLoading && <div className="answerLoading">Проверяем источники для краткого ответа…</div>}
          {answer?.available && <AnswerCard answer={answer} />}
          <div className="serpMeta">
            {data.results.length > 0 ? `Найдено: ${data.total}` : "Ничего не найдено"}
            {data.used_query !== data.normalized && <span> · использован вариант «{data.used_query}»</span>}
            <span> · {data.took_ms} мс{data.cached ? " · из кэша" : ""}</span>
          </div>
          {data.results.length === 0 ? <div className="searchState">Попробуйте изменить формулировку запроса или проверить раскладку клавиатуры.</div> : (
            <ol className="resultsList">
              {data.results.map((result) => (
                <li className="resultCard" key={`${result.id}-${result.url}`}>
                  <div className="resultHost">{result.host}</div>
                  <a className="resultTitle" href={result.url} rel="noopener noreferrer" onClick={() => trackClick(result.url)}>{result.title || result.url}</a>
                  <div className="resultUrl">{result.url}</div>
                  {result.snippet && <p className="resultSnippet">{renderSnippet(result.snippet)}</p>}
                </li>
              ))}
            </ol>
          )}
        </section>
      )}
    </main>
  );
}

function AnswerCard({ answer }: { answer: AnswerResponse }) {
  const sources = new Map((answer.sources || []).map((source) => [source.id, source]));
  const claims = answer.claims || [];
  if (claims.length === 0) return null;
  return (
    <section className="answerCard" aria-label="Ответ по найденным источникам">
      <div className="answerEyebrow">Ответ по источникам</div>
      <div className="answerClaims">
        {claims.map((claim, index) => (
          <p className={index === 0 ? "answerLead" : "answerClaim"} key={`${index}-${claim.text}`}>
            {claim.text}{" "}<span className="answerCitations">
              {claim.source_ids.map((sourceID) => {
                const source = sources.get(sourceID);
                return source ? <a key={sourceID} href={source.url} rel="noopener noreferrer" title={source.title || source.host} onClick={() => trackClick(source.url)}>[{sourceID}]</a> : null;
              })}
            </span>
          </p>
        ))}
      </div>
      <div className="answerSources">
        {(answer.sources || []).map((source) => <a href={source.url} rel="noopener noreferrer" key={source.id} onClick={() => trackClick(source.url)}>[{source.id}] {source.host}</a>)}
      </div>
    </section>
  );
}

function trackClick(url: string) {
  const body = JSON.stringify({ url });
  try {
    if (navigator.sendBeacon) {
      navigator.sendBeacon(`${apiBase}/api/click`, new Blob([body], { type: "application/json" }));
      return;
    }
    void fetch(`${apiBase}/api/click`, { method: "POST", headers: { "Content-Type": "application/json" }, body, keepalive: true });
  } catch { /* analytics must never block navigation */ }
}

function renderSnippet(snippet: string) {
  const parts = snippet.split(/(<mark>|<\/mark>)/gi);
  let marked = false;
  return parts.map((part, index) => {
    if (part.toLowerCase() === "<mark>") { marked = true; return null; }
    if (part.toLowerCase() === "</mark>") { marked = false; return null; }
    return marked ? <mark key={index}>{part}</mark> : <span key={index}>{part}</span>;
  });
}
