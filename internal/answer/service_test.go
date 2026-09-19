package answer

import (
	"strings"
	"testing"

	searchsvc "github.com/venomimonstro/poisk/internal/search"
)

func TestBuildRequiresIndependentHosts(t *testing.T) {
	results := []searchsvc.Result{
		{ID: 1, Host: "a.test", URL: "https://a.test/1", Title: "One", Snippet: "Индексация сайта помогает поисковой системе находить и сохранять страницы."},
		{ID: 2, Host: "a.test", URL: "https://a.test/2", Title: "Two", Snippet: "Индексация сайта начинается после обнаружения страницы поисковым роботом."},
	}
	got := Build("как работает индексация сайта", results, DefaultMinConfidence)
	if got.Available { t.Fatalf("answer must be unavailable: %+v", got) }
	if got.FallbackReason != "insufficient_independent_sources" { t.Fatalf("fallback=%q", got.FallbackReason) }
}

func TestBuildReturnsCitationIntegrity(t *testing.T) {
	results := []searchsvc.Result{
		{ID: 1, Host: "a.test", URL: "https://a.test/indexing", Title: "A", Snippet: "Индексация сайта работает после обхода страницы поисковым роботом и сохранения содержимого в поисковом индексе."},
		{ID: 2, Host: "b.test", URL: "https://b.test/indexing", Title: "B", Snippet: "Поисковый робот обходит страницу, а поисковая система добавляет доступное содержимое страницы в индекс сайта."},
		{ID: 3, Host: "c.test", URL: "https://c.test/indexing", Title: "C", Snippet: "Индексация страницы позволяет поисковой системе использовать найденное содержимое при поиске."},
	}
	got := Build("как работает индексация сайта", results, 0.50)
	if !got.Available { t.Fatalf("expected answer: %+v", got) }
	if got.Confidence < 0 || got.Confidence > 1 { t.Fatalf("confidence=%f", got.Confidence) }
	if len(got.Sources) < 2 || len(got.Claims) < 2 { t.Fatalf("response=%+v", got) }

	sourceIDs := make(map[int]struct{}, len(got.Sources))
	for _, source := range got.Sources {
		if source.URL == "" { t.Fatalf("source without URL: %+v", source) }
		sourceIDs[source.ID] = struct{}{}
	}
	for _, claim := range got.Claims {
		if claim.Text == "" || len(claim.SourceIDs) == 0 { t.Fatalf("invalid claim: %+v", claim) }
		for _, id := range claim.SourceIDs {
			if _, ok := sourceIDs[id]; !ok { t.Fatalf("claim references missing source id=%d", id) }
		}
	}
}

func TestBuildLowConfidenceFallsBack(t *testing.T) {
	results := []searchsvc.Result{
		{Host: "a.test", URL: "https://a.test/1", Snippet: "Хостинг используется для размещения файлов сайта и доступен пользователям через сеть."},
		{Host: "b.test", URL: "https://b.test/1", Snippet: "Сервер хранит файлы и обрабатывает запросы пользователей к интернет ресурсу."},
	}
	got := Build("почему поисковая индексация использует robots sitemap canonical", results, 0.80)
	if got.Available { t.Fatalf("unexpected answer: %+v", got) }
	if got.FallbackReason != "low_confidence" && got.FallbackReason != "insufficient_independent_sources" {
		t.Fatalf("fallback=%q", got.FallbackReason)
	}
}

func TestEvidenceIsHTMLFreeAndBounded(t *testing.T) {
	raw := "[[важный]] <b>текст</b> " + strings.Repeat("слово ", 200)
	clean := cleanEvidence(raw)
	if strings.Contains(clean, "<") || strings.Contains(clean, ">") || strings.Contains(clean, "[[") {
		t.Fatalf("unsafe evidence=%q", clean)
	}
	if len([]rune(clean)) > maxEvidenceRunes { t.Fatalf("evidence length=%d", len([]rune(clean))) }
}
