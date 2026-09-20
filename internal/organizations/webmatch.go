package organizations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5"
)

type WebMatchStats struct{Organizations int `json:"organizations"`;WebsiteLinks int `json:"website_links"`;SchemaLinks int `json:"schema_links"`}
type schemaSignal struct{Name string;URL string;Phone string;Type string}

// MatchWebBatch creates provenance links from canonical organizations to the web corpus.
// It only accepts deterministic host equality plus bounded Schema.org confirmations.
func (r *Repository) MatchWebBatch(ctx context.Context,afterPlaceID int64,limit int)(WebMatchStats,int64,error){
	if r==nil||r.db==nil{return WebMatchStats{},afterPlaceID,errors.New("organization repository is not initialized")}
	if limit<=0||limit>1000{limit=200}
	rows,err:=r.db.Query(ctx,`SELECT place_id,normalized_name,COALESCE(phone,''),COALESCE(website,'') FROM organizations
WHERE place_id>$1 AND status IN ('ACTIVE','REVIEW') AND website IS NOT NULL AND website<>'' ORDER BY place_id LIMIT $2`,afterPlaceID,limit)
	if err!=nil{return WebMatchStats{},afterPlaceID,err};defer rows.Close()
	type org struct{id int64;name,phone,website string};items:=make([]org,0,limit)
	for rows.Next(){var v org;if err:=rows.Scan(&v.id,&v.name,&v.phone,&v.website);err!=nil{return WebMatchStats{},afterPlaceID,err};items=append(items,v)}
	if err:=rows.Err();err!=nil{return WebMatchStats{},afterPlaceID,err}
	stats:=WebMatchStats{};cursor:=afterPlaceID
	for _,item:=range items{
		if err:=ctx.Err();err!=nil{return stats,cursor,err};cursor=item.id;stats.Organizations++
		host,err:=websiteHost(item.website);if err!=nil{continue}
		var domainID int64
		err=r.db.QueryRow(ctx,`SELECT domain_id FROM domains WHERE host=$1`,host).Scan(&domainID)
		if errors.Is(err,pgx.ErrNoRows){continue};if err!=nil{return stats,cursor,err}
		if err:=r.upsertWebLink(ctx,item.id,domainID,nil,"WEBSITE_HOST",100,map[string]any{"host":host});err!=nil{return stats,cursor,err};stats.WebsiteLinks++
		docs,err:=r.schemaDocuments(ctx,domainID,50);if err!=nil{return stats,cursor,err}
		for _,doc:=range docs{
			for _,signal:=range parseSchemaSignals(doc.data){
				if signal.Phone!=""&&item.phone!=""&&normalizeSchemaPhone(signal.Phone)==item.phone{
					urlID:=doc.urlID;if err:=r.upsertWebLink(ctx,item.id,domainID,&urlID,"SCHEMA_PHONE",95,map[string]any{"type":signal.Type,"name":signal.Name});err!=nil{return stats,cursor,err};stats.SchemaLinks++;continue
				}
				if signal.URL!=""&&normalizeIdentityText(signal.Name)==item.name{
					schemaHost,err:=websiteHost(signal.URL);if err==nil&&schemaHost==host{urlID:=doc.urlID;if err:=r.upsertWebLink(ctx,item.id,domainID,&urlID,"SCHEMA_URL",90,map[string]any{"type":signal.Type,"name":signal.Name});err!=nil{return stats,cursor,err};stats.SchemaLinks++}
				}
			}
		}
	}
	return stats,cursor,nil
}

type schemaDoc struct{urlID int64;data []byte}
func (r *Repository) schemaDocuments(ctx context.Context,domainID int64,limit int)([]schemaDoc,error){
	rows,err:=r.db.Query(ctx,`SELECT u.url_id,dc.structured_data FROM urls u JOIN document_content dc ON dc.url_id=u.url_id AND dc.version=u.version
WHERE u.domain_id=$1 AND u.index_status<>'DELETED' AND dc.robots_noindex=FALSE AND jsonb_array_length(dc.structured_data)>0 ORDER BY u.url_id LIMIT $2`,domainID,limit)
	if err!=nil{return nil,err};defer rows.Close();out:=make([]schemaDoc,0,limit);for rows.Next(){var d schemaDoc;if err:=rows.Scan(&d.urlID,&d.data);err!=nil{return nil,err};out=append(out,d)};return out,rows.Err()
}
func (r *Repository) upsertWebLink(ctx context.Context,placeID,domainID int64,urlID *int64,matchType string,confidence int,evidence map[string]any)error{
	raw,err:=json.Marshal(evidence);if err!=nil{return err};_,err=r.db.Exec(ctx,`INSERT INTO organization_web_links(place_id,domain_id,source_url_id,match_type,confidence,evidence)
VALUES($1,$2,$3,$4,$5,$6::jsonb) ON CONFLICT(place_id,domain_id,match_type) DO UPDATE SET source_url_id=COALESCE(EXCLUDED.source_url_id,organization_web_links.source_url_id),confidence=GREATEST(organization_web_links.confidence,EXCLUDED.confidence),evidence=EXCLUDED.evidence,last_seen_at=now()`,placeID,domainID,urlID,matchType,confidence,string(raw));return err
}
func websiteHost(raw string)(string,error){u,err:=url.Parse(strings.TrimSpace(raw));if err!=nil{return "",err};host:=strings.ToLower(strings.TrimSuffix(u.Hostname(),"."));if host==""{return "",errors.New("website host is empty")};return host,nil}
func normalizeSchemaPhone(raw string)string{var b strings.Builder;for _,r:=range raw{if r>='0'&&r<='9'{b.WriteRune(r)}};value:=b.String();if len(value)==11&&strings.HasPrefix(value,"8"){value="7"+value[1:]};if value==""{return ""};return "+"+value}
func parseSchemaSignals(raw []byte)[]schemaSignal{
	var encoded []json.RawMessage;if err:=json.Unmarshal(raw,&encoded);err!=nil{return nil};out:=make([]schemaSignal,0,8)
	for _,item:=range encoded{var text string;if err:=json.Unmarshal(item,&text);err==nil{collectSchemaJSON([]byte(text),&out);continue};collectSchemaJSON(item,&out)}
	return out
}
func collectSchemaJSON(raw []byte,out *[]schemaSignal){var value any;if err:=json.Unmarshal(raw,&value);err!=nil{return};walkSchema(value,out,0)}
func walkSchema(value any,out *[]schemaSignal,depth int){if depth>8||len(*out)>=64{return};switch v:=value.(type){case []any:for _,item:=range v{walkSchema(item,out,depth+1)};case map[string]any:s:=schemaSignal{Type:stringValue(v["@type"]),Name:stringValue(v["name"]),URL:stringValue(v["url"]),Phone:stringValue(v["telephone"])};if s.Type!=""&&(s.Name!=""||s.URL!=""||s.Phone!=""){*out=append(*out,s)};if graph,ok:=v["@graph"];ok{walkSchema(graph,out,depth+1)}}}
func stringValue(value any)string{switch v:=value.(type){case string:return strings.TrimSpace(v);case []any:if len(v)>0{return stringValue(v[0])}};return ""}

var _ = fmt.Sprintf
