//go:build integration

package reviews

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func reviewDB(t *testing.T)*pgxpool.Pool{t.Helper();dsn:=os.Getenv("TEST_DATABASE_URL");if dsn==""{t.Fatal("TEST_DATABASE_URL is required")};pool,err:=pgxpool.New(context.Background(),dsn);if err!=nil{t.Fatal(err)};if err=pool.Ping(context.Background());err!=nil{pool.Close();t.Fatal(err)};t.Cleanup(pool.Close);return pool}
func createConsumer(t *testing.T,pool *pgxpool.Pool,prefix string)int64{t.Helper();var id int64;email:=fmt.Sprintf("%s-%d@example.test",prefix,time.Now().UnixNano());if err:=pool.QueryRow(context.Background(),`INSERT INTO consumer_users(email,password_hash,email_verified_at) VALUES($1,$2,now()) RETURNING user_id`,email,"integration-password-hash-000000").Scan(&id);err!=nil{t.Fatal(err)};return id}
func createUnverifiedConsumer(t *testing.T,pool *pgxpool.Pool,prefix string)int64{t.Helper();var id int64;email:=fmt.Sprintf("%s-%d@example.test",prefix,time.Now().UnixNano());if err:=pool.QueryRow(context.Background(),`INSERT INTO consumer_users(email,password_hash) VALUES($1,$2) RETURNING user_id`,email,"integration-password-hash-000000").Scan(&id);err!=nil{t.Fatal(err)};return id}
func createPlace(t *testing.T,pool *pgxpool.Pool,prefix string)int64{t.Helper();var id int64;name:=fmt.Sprintf("%s %d",prefix,time.Now().UnixNano());if err:=pool.QueryRow(context.Background(),`INSERT INTO organizations(name,normalized_name,status,quality_score,source_count) VALUES($1,lower($1),'ACTIVE',80,1) RETURNING place_id`,name).Scan(&id);err!=nil{t.Fatal(err)};return id}

func TestReviewRevisionAndVisibleRatingLifecycle(t *testing.T){
	pool:=reviewDB(t);repo:=Repository{DB:pool};ctx:=context.Background();userID:=createConsumer(t,pool,"reviewer");placeID:=createPlace(t,pool,"Review place")
	first,err:=repo.Upsert(ctx,userID,placeID,5,"Отличное место и хороший сервис.");if err!=nil{t.Fatal(err)};if first.Version!=1{t.Fatalf("version=%d",first.Version)}
	stats,err:=repo.Stats(ctx,placeID);if err!=nil{t.Fatal(err)};if stats.Count!=1||stats.Average!=5{t.Fatalf("stats=%+v",stats)}
	second,err:=repo.Upsert(ctx,userID,placeID,3,"После повторного визита впечатление изменилось.");if err!=nil{t.Fatal(err)};if second.ID!=first.ID||second.Version!=2{t.Fatalf("second=%+v first=%+v",second,first)}
	var revisions int;if err:=pool.QueryRow(ctx,`SELECT count(*) FROM organization_review_revisions WHERE review_id=$1`,first.ID).Scan(&revisions);err!=nil{t.Fatal(err)};if revisions!=2{t.Fatalf("revisions=%d",revisions)}
	stats,err=repo.Stats(ctx,placeID);if err!=nil{t.Fatal(err)};if stats.Count!=1||stats.Average!=3{t.Fatalf("stats after edit=%+v",stats)}
	if err:=repo.SoftDelete(ctx,userID,first.ID);err!=nil{t.Fatal(err)};stats,err=repo.Stats(ctx,placeID);if err!=nil{t.Fatal(err)};if stats.Count!=0||stats.Average!=0{t.Fatalf("deleted review still rated: %+v",stats)}
	if err:=pool.QueryRow(ctx,`SELECT count(*) FROM organization_review_revisions WHERE review_id=$1`,first.ID).Scan(&revisions);err!=nil{t.Fatal(err)};if revisions!=3{t.Fatalf("revisions after delete=%d",revisions)}
	if _,err:=pool.Exec(ctx,`DELETE FROM organization_reviews WHERE review_id=$1`,first.ID);err==nil{t.Fatal("hard delete unexpectedly succeeded")}
}

func TestFreshUnverifiedAccountStartsPending(t *testing.T){
	pool:=reviewDB(t);repo:=Repository{DB:pool};ctx:=context.Background();userID:=createUnverifiedConsumer(t,pool,"fresh-unverified");placeID:=createPlace(t,pool,"Trust gate place")
	review,err:=repo.Upsert(ctx,userID,placeID,5,"Новый аккаунт оставляет отзыв на модерацию.");if err!=nil{t.Fatal(err)};if review.Status!="PENDING"{t.Fatalf("status=%s",review.Status)}
	stats,err:=repo.Stats(ctx,placeID);if err!=nil{t.Fatal(err)};if stats.Count!=0||stats.Average!=0{t.Fatalf("pending fresh account affects rating: %+v",stats)}
	var reason string;if err:=pool.QueryRow(ctx,`SELECT change_reason FROM organization_reviews WHERE review_id=$1`,review.ID).Scan(&reason);err!=nil{t.Fatal(err)};if reason!="USER_CREATE_TRUST_REVIEW"{t.Fatalf("reason=%s",reason)}
}

func TestModeratedReviewCannotSelfRestoreVisible(t *testing.T){
	pool:=reviewDB(t);repo:=Repository{DB:pool};ctx:=context.Background();userID:=createConsumer(t,pool,"moderated");placeID:=createPlace(t,pool,"Moderated place")
	review,err:=repo.Upsert(ctx,userID,placeID,4,"Первоначальный пользовательский отзыв.");if err!=nil{t.Fatal(err)}
	if _,err:=pool.Exec(ctx,`UPDATE organization_reviews SET status='HIDDEN',change_actor_type='SYSTEM',change_actor_id=NULL,change_reason='ABUSE_FILTER' WHERE review_id=$1`,review.ID);err!=nil{t.Fatal(err)}
	edited,err:=repo.Upsert(ctx,userID,placeID,5,"Исправленный текст после модерации отзыва.");if err!=nil{t.Fatal(err)};if edited.Status!="PENDING"{t.Fatalf("status=%s",edited.Status)}
	stats,err:=repo.Stats(ctx,placeID);if err!=nil{t.Fatal(err)};if stats.Count!=0{t.Fatalf("pending review affects rating: %+v",stats)}
}

func TestOwnerReplyRequiresActiveClaimForSameConsumer(t *testing.T){
	pool:=reviewDB(t);repo:=Repository{DB:pool};ctx:=context.Background();reviewer:=createConsumer(t,pool,"reply-reviewer");owner:=createConsumer(t,pool,"reply-owner");other:=createConsumer(t,pool,"reply-other");placeID:=createPlace(t,pool,"Claimed place")
	review,err:=repo.Upsert(ctx,reviewer,placeID,5,"Отзыв для проверки ответа владельца.");if err!=nil{t.Fatal(err)}
	var domainID int64;host:=fmt.Sprintf("claim-%d.example.test",time.Now().UnixNano());if err:=pool.QueryRow(ctx,`INSERT INTO domains(host) VALUES($1) RETURNING domain_id`,host).Scan(&domainID);err!=nil{t.Fatal(err)}
	var wmID int64;if err:=pool.QueryRow(ctx,`INSERT INTO webmaster_users(email,password_hash,status,consumer_user_id) SELECT email,password_hash,'ACTIVE',user_id FROM consumer_users WHERE user_id=$1 RETURNING user_id`,owner).Scan(&wmID);err!=nil{t.Fatal(err)}
	var siteID int64;if err:=pool.QueryRow(ctx,`INSERT INTO webmaster_sites(user_id,domain_id,origin,host,status,verified_at,verification_method) VALUES($1,$2,$3,$4,'VERIFIED',now(),'DNS_TXT') RETURNING site_id`,wmID,domainID,"https://"+host,host).Scan(&siteID);err!=nil{t.Fatal(err)}
	var claimID int64;if err:=pool.QueryRow(ctx,`INSERT INTO organization_claims(place_id,user_id,site_id,proof_type,proof_host,status) VALUES($1,$2,$3,'VERIFIED_WEBSITE_HOST',$4,'ACTIVE') RETURNING claim_id`,placeID,wmID,siteID,host).Scan(&claimID);err!=nil{t.Fatal(err)}
	if _,err:=repo.Upsert(ctx,owner,placeID,5,"Владелец не должен ставить рейтинг своей компании.");!errors.Is(err,ErrForbidden){t.Fatalf("claimed owner self-review err=%v",err)}
	if _,err:=repo.Reply(ctx,other,review.ID,"Чужой пользователь не должен отвечать.");!errors.Is(err,ErrForbidden){t.Fatalf("other reply err=%v",err)}
	reply,err:=repo.Reply(ctx,owner,review.ID,"Спасибо за ваш отзыв!");if err!=nil{t.Fatal(err)};if reply.Body==""{t.Fatal("empty owner reply")}
	if _,err:=pool.Exec(ctx,`UPDATE organization_claims SET status='REVOKED',revoked_at=now(),updated_at=now() WHERE claim_id=$1`,claimID);err!=nil{t.Fatal(err)}
	if _,err:=repo.Reply(ctx,owner,review.ID,"Ответ после отзыва прав владения.");!errors.Is(err,ErrForbidden){t.Fatalf("revoked claim reply err=%v",err)}
}

func TestClaimActivationSuppressesExistingOwnerReview(t *testing.T){
	pool:=reviewDB(t);repo:=Repository{DB:pool};ctx:=context.Background();owner:=createConsumer(t,pool,"prior-owner-review");placeID:=createPlace(t,pool,"Prior owner place")
	review,err:=repo.Upsert(ctx,owner,placeID,5,"Отзыв был создан до подтверждения владения компанией.");if err!=nil{t.Fatal(err)}
	stats,err:=repo.Stats(ctx,placeID);if err!=nil{t.Fatal(err)};if stats.Count!=1{t.Fatalf("pre-claim stats=%+v",stats)}
	var domainID int64;host:=fmt.Sprintf("prior-claim-%d.example.test",time.Now().UnixNano());if err:=pool.QueryRow(ctx,`INSERT INTO domains(host) VALUES($1) RETURNING domain_id`,host).Scan(&domainID);err!=nil{t.Fatal(err)}
	var wmID int64;if err:=pool.QueryRow(ctx,`INSERT INTO webmaster_users(email,password_hash,status,consumer_user_id) SELECT email,password_hash,'ACTIVE',user_id FROM consumer_users WHERE user_id=$1 RETURNING user_id`,owner).Scan(&wmID);err!=nil{t.Fatal(err)}
	var siteID int64;if err:=pool.QueryRow(ctx,`INSERT INTO webmaster_sites(user_id,domain_id,origin,host,status,verified_at,verification_method) VALUES($1,$2,$3,$4,'VERIFIED',now(),'DNS_TXT') RETURNING site_id`,wmID,domainID,"https://"+host,host).Scan(&siteID);err!=nil{t.Fatal(err)}
	if _,err:=pool.Exec(ctx,`INSERT INTO organization_claims(place_id,user_id,site_id,proof_type,proof_host,status) VALUES($1,$2,$3,'VERIFIED_WEBSITE_HOST',$4,'ACTIVE')`,placeID,wmID,siteID,host);err!=nil{t.Fatal(err)}
	var status,reason string;if err:=pool.QueryRow(ctx,`SELECT status,change_reason FROM organization_reviews WHERE review_id=$1`,review.ID).Scan(&status,&reason);err!=nil{t.Fatal(err)};if status!="HIDDEN"||reason!="OWNER_CLAIM_SELF_REVIEW"{t.Fatalf("status=%s reason=%s",status,reason)}
	stats,err=repo.Stats(ctx,placeID);if err!=nil{t.Fatal(err)};if stats.Count!=0||stats.Average!=0{t.Fatalf("owner self-review still affects rating: %+v",stats)}
	var revisions int;if err:=pool.QueryRow(ctx,`SELECT count(*) FROM organization_review_revisions WHERE review_id=$1`,review.ID).Scan(&revisions);err!=nil{t.Fatal(err)};if revisions!=2{t.Fatalf("revisions=%d",revisions)}
}
