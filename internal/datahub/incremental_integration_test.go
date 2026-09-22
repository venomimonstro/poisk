//go:build integration

package datahub

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestOrganizationMaterializerResumesFromSavedCursor(t *testing.T){
	dsn:=os.Getenv("TEST_DATABASE_URL");if dsn==""{t.Fatal("TEST_DATABASE_URL is required")}
	ctx:=context.Background();pool,err:=pgxpool.New(ctx,dsn);if err!=nil{t.Fatal(err)};defer pool.Close()
	if _,err=pool.Exec(ctx,`TRUNCATE TABLE datahub_publication_events,datahub_page_versions,datahub_pages,organization_source_links,organizations,organization_sources RESTART IDENTITY CASCADE`);err!=nil{t.Fatal(err)}
	if _,err=pool.Exec(ctx,`INSERT INTO datahub_build_state(job_key,cursor_text) VALUES('ORGANIZATIONS','') ON CONFLICT(job_key) DO UPDATE SET cursor_text=''`);err!=nil{t.Fatal(err)}
	if _,err=pool.Exec(ctx,`INSERT INTO organization_sources(source_key,display_name,trust_level) VALUES('test','Test',100)`);err!=nil{t.Fatal(err)}
	rows,err:=pool.Query(ctx,`INSERT INTO organizations(status,name,normalized_name,city_key,category_key,address,normalized_address,quality_score,source_count)
VALUES('ACTIVE','One','one','moscow','clinic','Address 1','address 1',80,1),('ACTIVE','Two','two','moscow','clinic','Address 2','address 2',90,1)
RETURNING place_id`);if err!=nil{t.Fatal(err)}
	var ids []int64;for rows.Next(){var id int64;if err:=rows.Scan(&id);err!=nil{t.Fatal(err)};ids=append(ids,id)};rows.Close();if len(ids)!=2{t.Fatalf("ids=%v",ids)}
	for _,id:=range ids{if _,err=pool.Exec(ctx,`INSERT INTO organization_source_links(source_key,source_record_id,place_id,source_payload_hash) VALUES('test',$1,$2,repeat('a',64))`,string(rune('a'+id)),id);err!=nil{t.Fatal(err)}}

	repo:=NewRepository(pool);now:=time.Now().UTC()
	first,err:=repo.RebuildOrganizationBatch(ctx,1,now);if err!=nil{t.Fatal(err)}
	if first.Scanned!=1||first.NextCursor==""{t.Fatalf("first=%+v",first)}
	var firstCursor string;if err:=pool.QueryRow(ctx,`SELECT cursor_text FROM datahub_build_state WHERE job_key='ORGANIZATIONS'`).Scan(&firstCursor);err!=nil{t.Fatal(err)}
	if firstCursor!=first.NextCursor{t.Fatalf("cursor=%q stats=%q",firstCursor,first.NextCursor)}

	second,err:=repo.RebuildOrganizationBatch(ctx,1,now.Add(time.Second));if err!=nil{t.Fatal(err)}
	if second.Scanned!=1||second.NextCursor==first.NextCursor{t.Fatalf("second=%+v",second)}
	var pages int;if err:=pool.QueryRow(ctx,`SELECT count(*) FROM datahub_pages WHERE page_type='ORGANIZATION'`).Scan(&pages);err!=nil{t.Fatal(err)}
	if pages!=2{t.Fatalf("pages=%d",pages)}

	complete,err:=repo.RebuildOrganizationBatch(ctx,1,now.Add(2*time.Second));if err!=nil{t.Fatal(err)}
	if !complete.Completed||complete.Scanned!=0{t.Fatalf("complete=%+v",complete)}
	var reset string;if err:=pool.QueryRow(ctx,`SELECT cursor_text FROM datahub_build_state WHERE job_key='ORGANIZATIONS'`).Scan(&reset);err!=nil{t.Fatal(err)}
	if reset!=""{t.Fatalf("reset cursor=%q",reset)}
}
