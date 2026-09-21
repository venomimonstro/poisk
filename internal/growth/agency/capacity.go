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
