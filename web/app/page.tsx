export default function HomePage() {
  return (
    <main className="shell">
      <section className="hero">
        <div className="brand">ПОИСК</div>
        <h1>Поиск, который не заставляет искать ответ.</h1>
        <p>Web Search + Answer Engine + GEO. Первая версия интерфейса будет подключена к Search API в Sprint 07.</p>
        <form className="search" action="#">
          <input aria-label="Поисковый запрос" placeholder="Найдите или спросите что угодно" disabled />
          <button type="button" disabled>Найти</button>
        </form>
      </section>
    </main>
  );
}
