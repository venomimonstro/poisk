package agency

import "context"

type Capacity struct {
	Members int64 `json:"members"`
	Sites int64 `json:"sites"`
}

func (r *Repository) Capacity(ctx context.Context,agencyID int64)(Capacity,error){
	if r==nil||r.db==nil||agencyID<=0{return Capacity{},ErrInvalid}
	var out Capacity
	err:=r.db.QueryRow(ctx,`SELECT
  (SELECT count(*) FROM agency_members WHERE agency_id=$1 AND status='ACTIVE'),
  (SELECT count(*) FROM agency_site_access WHERE agency_id=$1 AND status='ACTIVE')`).Scan(&out.Members,&out.Sites)
	return out,err
}

func (r *Repository) HasActiveSite(ctx context.Context,agencyID,siteID int64)(bool,error){
	if r==nil||r.db==nil||agencyID<=0||siteID<=0{return false,ErrInvalid}
	var exists bool
	err:=r.db.QueryRow(ctx,`SELECT EXISTS(SELECT 1 FROM agency_site_access WHERE agency_id=$1 AND site_id=$2 AND status='ACTIVE')`,agencyID,siteID).Scan(&exists)
	return exists,err
}
