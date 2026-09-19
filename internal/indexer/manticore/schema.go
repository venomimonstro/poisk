package manticore

const (
	WebIndex         = "web_documents"
	WebSchemaVersion = 1
)

// WebSchemaSQL is intentionally explicit. Schema changes require a version bump
// and an explicit rebuild instead of silent runtime mutation.
const WebSchemaSQL = `CREATE TABLE IF NOT EXISTS web_documents (
    title text indexed stored,
    description text indexed stored,
    body text indexed stored,
    url string attribute indexed,
    host string attribute indexed,
    lang string attribute,
    content_hash string attribute,
    entity_version bigint,
    quality_score float,
    spam_score float,
    fetched_at timestamp
) morphology='stem_enru' min_word_len='2'`

type Document struct {
	ID            int64
	EntityVersion int64
	Title         string
	Description   string
	Body          string
	URL           string
	Host          string
	Lang          string
	ContentHash   string
	QualityScore  float64
	SpamScore     float64
	FetchedAtUnix int64
}
