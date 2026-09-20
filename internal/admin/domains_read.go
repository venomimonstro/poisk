package admin

import (
	"context"
	"errors"
	"strings"
)

type DomainRow struct {
	ID int64 `json:"domain_id"`
	Host string `json:"host"`
	Status string `json:"status"`
	Policy string `json:"policy"`
	TrustLevel int `json:"trust_level"`
	QualityScore float64 `json:"quality_score"`
	DemandScore float64 `json:"demand_score"`
}

func (s Service) ListDomains(ctx context.Context,session Session,query string,limit int)([]DomainRow,error){
	if s.Store==nil||s.Store.db==nil{return nil,errors.New("admin service is not initialized")}
	if err:=s.RequireRole(session,"OPERATOR","ANALYST","SUPPORT");err!=nil{return nil,err}
	query=strings.ToLower(strings.TrimSpace(query));if len(query)>255{return nil,ErrInvalidCredential};if limit<=0{limit=20};if limit>50{limit=50}
	rows,err:=s.Store.db.Query(ctx,`SELECT domain_id,host,status,policy,trust_level,quality_score,demand_score FROM domains
WHERE $1='' OR lower(host) LIKE '%'||$1||'%' ORDER BY CASE WHEN lower(host)=$1 THEN 0 ELSE 1 END,host LIMIT $2`,query,limit);if err!=nil{return nil,err};defer rows.Close()
	out:=make([]DomainRow,0,limit);for rows.Next(){var row DomainRow;if err:=rows.Scan(&row.ID,&row.Host,&row.Status,&row.Policy,&row.TrustLevel,&row.QualityScore,&row.DemandScore);err!=nil{return nil,err};out=append(out,row)};if err:=rows.Err();err!=nil{return nil,err};return out,nil
}
