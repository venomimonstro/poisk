package datahub

import (
	"context"
	"time"
)

type SitemapStats struct{
	Published int64 `json:"published"`
	ShardSize int `json:"shard_size"`
	Shards int `json:"shards"`
}

func (r *Repository) SitemapStats(ctx context.Context,shardSize int)(SitemapStats,error){
	if r==nil||r.db==nil||shardSize<100||shardSize>5000{return SitemapStats{},ErrInvalid}
	var count int64
	if err:=r.db.QueryRow(ctx,`SELECT count(*) FROM datahub_pages WHERE state='PUBLISHED'`).Scan(&count);err!=nil{return SitemapStats{},err}
	shards:=0;if count>0{shards=int((count+int64(shardSize)-1)/int64(shardSize))}
	return SitemapStats{Published:count,ShardSize:shardSize,Shards:shards},nil
}

func (r *Repository) SitemapShard(ctx context.Context,shard,shardSize int)([]SitemapItem,error){
	if r==nil||r.db==nil||shard<0||shard>100000||shardSize<100||shardSize>5000{return nil,ErrInvalid}
	offset:=int64(shard)*int64(shardSize)
	rows,err:=r.db.Query(ctx,`SELECT page_id,canonical_path,updated_at
FROM datahub_pages
WHERE state='PUBLISHED'
ORDER BY page_id
LIMIT $1 OFFSET $2`,shardSize,offset);if err!=nil{return nil,err};defer rows.Close()
	out:=make([]SitemapItem,0,shardSize)
	for rows.Next(){var x SitemapItem;if err:=rows.Scan(&x.PageID,&x.Path,&x.UpdatedAt);err!=nil{return nil,err};x.UpdatedAt=x.UpdatedAt.UTC().Truncate(time.Second);out=append(out,x)}
	return out,rows.Err()
}
