package widget

import (
	"context"
	"time"
)

type UsageDay struct {
	Day time.Time `json:"day"`
	Requests int64 `json:"requests"`
	Results int64 `json:"results"`
	ZeroResults int64 `json:"zero_results"`
}

func (r *Repository) Usage(ctx context.Context,userID,siteID int64,days int)([]UsageDay,error){
	if r==nil||r.db==nil||userID<=0||siteID<=0{return nil,ErrInvalid};if days<=0{days=30};if days>90{days=90}
	var allowed bool
	if err:=r.db.QueryRow(ctx,`SELECT EXISTS(SELECT 1 FROM site_search_widgets w JOIN webmaster_sites s ON s.site_id=w.site_id WHERE w.site_id=$1 AND s.user_id=$2)`,siteID,userID).Scan(&allowed);err!=nil{return nil,err};if !allowed{return nil,ErrForbidden}
	rows,err:=r.db.Query(ctx,`SELECT u.day,u.requests,u.results,u.zero_results FROM site_search_usage_daily u JOIN site_search_widgets w ON w.widget_id=u.widget_id WHERE w.site_id=$1 AND u.day>=CURRENT_DATE-$2::int ORDER BY u.day DESC`,siteID,days);if err!=nil{return nil,err};defer rows.Close();out:=[]UsageDay{};for rows.Next(){var v UsageDay;if err:=rows.Scan(&v.Day,&v.Requests,&v.Results,&v.ZeroResults);err!=nil{return nil,err};out=append(out,v)};if err:=rows.Err();err!=nil{return nil,err};return out,nil
}
