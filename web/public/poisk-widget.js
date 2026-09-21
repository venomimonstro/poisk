(() => {
  "use strict";
  const script = document.currentScript;
  if (!script) return;
  const key = (script.dataset.key || "").trim();
  if (!/^psw_[A-Za-z0-9_-]{24,64}$/.test(key)) return;

  let apiBase;
  try { apiBase = new URL(script.src, window.location.href).origin; } catch { return; }
  const pageHost = window.location.hostname.toLowerCase().replace(/\.$/, "");
  const targetSelector = (script.dataset.target || "").trim();
  const mount = targetSelector ? document.querySelector(targetSelector) : null;
  const root = mount || document.createElement("div");
  if (!mount) script.insertAdjacentElement("afterend", root);
  root.className = "poisk-site-search";
  root.setAttribute("role", "search");

  const form = document.createElement("form");
  form.style.display = "flex";
  form.style.gap = "8px";
  form.style.width = "100%";

  const label = document.createElement("label");
  label.textContent = script.dataset.label || "Поиск по сайту";
  label.style.position = "absolute";
  label.style.width = "1px";
  label.style.height = "1px";
  label.style.overflow = "hidden";
  label.style.clipPath = "inset(50%)";

  const input = document.createElement("input");
  input.type = "search";
  input.name = "q";
  input.required = true;
  input.maxLength = 256;
  input.autocomplete = "off";
  input.placeholder = script.dataset.placeholder || "Поиск по сайту";
  input.style.flex = "1";
  input.style.minWidth = "0";
  input.style.padding = "10px 12px";
  input.style.border = "1px solid #bbb";
  input.style.borderRadius = "8px";
  input.style.font = "inherit";
  label.htmlFor = `poisk-widget-${Math.random().toString(36).slice(2)}`;
  input.id = label.htmlFor;

  const submit = document.createElement("button");
  submit.type = "submit";
  submit.textContent = script.dataset.button || "Найти";
  submit.style.padding = "10px 14px";
  submit.style.border = "0";
  submit.style.borderRadius = "8px";
  submit.style.cursor = "pointer";

  const status = document.createElement("div");
  status.setAttribute("role", "status");
  status.setAttribute("aria-live", "polite");
  status.style.marginTop = "8px";

  const results = document.createElement("ol");
  results.style.paddingLeft = "20px";
  results.style.margin = "12px 0 0";

  form.append(label, input, submit);
  root.replaceChildren(form, status, results);

  let request = null;
  form.addEventListener("submit", async (event) => {
    event.preventDefault();
    const q = input.value.trim();
    if (!q) return;
    request?.abort();
    request = new AbortController();
    submit.disabled = true;
    status.textContent = "Ищем…";
    results.replaceChildren();
    try {
      const response = await fetch(`${apiBase}/api/widget/search?key=${encodeURIComponent(key)}`, {
        method: "POST",
        mode: "cors",
        credentials: "omit",
        cache: "no-store",
        headers: { "Content-Type": "application/json", "Accept": "application/json" },
        body: JSON.stringify({ q, limit: 10 }),
        signal: request.signal,
      });
      if (!response.ok) throw new Error(`search_${response.status}`);
      const data = await response.json();
      const items = Array.isArray(data.results) ? data.results : [];
      status.textContent = items.length ? `Найдено: ${items.length}` : "Ничего не найдено";
      for (const item of items) {
        const li = document.createElement("li");
        li.style.marginBottom = "10px";
        const a = document.createElement("a");
        a.textContent = String(item.title || item.url || "Результат");
        try {
          const destination = new URL(String(item.url || ""), window.location.origin);
          const destinationHost = destination.hostname.toLowerCase().replace(/\.$/, "");
          if ((destination.protocol !== "http:" && destination.protocol !== "https:") || destinationHost !== pageHost) continue;
          a.href = destination.href;
        } catch { continue; }
        const snippet = document.createElement("div");
        snippet.textContent = String(item.snippet || "").replaceAll("[[", "").replaceAll("]]", "");
        snippet.style.fontSize = "0.92em";
        snippet.style.marginTop = "3px";
        li.append(a, snippet);
        results.append(li);
      }
    } catch (error) {
      if (error instanceof DOMException && error.name === "AbortError") return;
      status.textContent = "Поиск временно недоступен";
    } finally {
      submit.disabled = false;
    }
  });
})();
