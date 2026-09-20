package manticore

const (
	WebIndex                    = "web_documents"
	WebSchemaVersion            = 2
	OrganizationsIndex          = "organizations"
	OrganizationsSchemaVersion  = 2
	AddressesIndex              = "addresses"
	AddressesSchemaVersion      = 1
)

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
    authority_score float,
    fetched_at timestamp
) morphology='stem_enru' min_word_len='2'`

const OrganizationsSchemaSQL = `CREATE TABLE IF NOT EXISTS organizations (
    name text indexed stored,
    address text indexed stored,
    city_key string attribute indexed,
    category_key string attribute indexed,
    phone string attribute,
    website string attribute indexed,
    normalized_name string attribute indexed,
    normalized_address string attribute indexed,
    latitude float,
    longitude float,
    has_location bool,
    quality_score float,
    source_count uint,
    entity_version bigint,
    status string attribute indexed
) morphology='stem_enru' min_word_len='2'`

const AddressesSchemaSQL = `CREATE TABLE IF NOT EXISTS addresses (
    display_name text indexed stored,
    full_address text indexed stored,
    normalized_name string attribute indexed,
    region_code uint,
    object_kind string attribute indexed,
    level uint,
    parent_address_id bigint,
    latitude float,
    longitude float,
    has_location bool,
    entity_version bigint,
    status string attribute indexed
) morphology='stem_enru' min_word_len='1'`

type Document struct {
	ID             int64
	EntityVersion  int64
	Title          string
	Description    string
	Body           string
	URL            string
	Host           string
	Lang           string
	ContentHash    string
	QualityScore   float64
	SpamScore      float64
	AuthorityScore float64
	FetchedAtUnix  int64
}

type OrganizationDocument struct {
	ID                int64
	EntityVersion     int64
	Name              string
	Address           string
	CityKey           string
	CategoryKey       string
	Phone             string
	Website           string
	NormalizedName    string
	NormalizedAddress string
	Latitude          float64
	Longitude         float64
	HasLocation       bool
	QualityScore      float64
	SourceCount       int
	Status            string
}

type AddressDocument struct {
	ID              int64
	EntityVersion   int64
	DisplayName     string
	FullAddress     string
	NormalizedName  string
	RegionCode      int
	ObjectKind      string
	Level           int
	ParentAddressID int64
	Latitude        float64
	Longitude       float64
	HasLocation     bool
	Status          string
}
