//go:build integration

package reviews

import (
	"context"
	"errors"
	"testing"
)

func TestReviewMutationIDORIsRejected(t *testing.T){
	pool:=reviewDB(t);repo:=Repository{DB:pool};ctx:=context.Background();owner:=createConsumer(t,pool,"idor-owner");other:=createConsumer(t,pool,"idor-other");placeID:=createPlace(t,pool,"IDOR place")
	review,err:=repo.Upsert(ctx,owner,placeID,4,"Отзыв принадлежит только своему автору.");if err!=nil{t.Fatal(err)}
	if err:=repo.SoftDelete(ctx,other,review.ID);!errors.Is(err,ErrNotFound){t.Fatalf("foreign delete err=%v",err)}
	items,err:=repo.ListPublic(ctx,placeID,20,0);if err!=nil{t.Fatal(err)};if len(items)!=1||items[0].ID!=review.ID{t.Fatalf("foreign mutation changed review: %+v",items)}
}

func TestUnverifiedAccountEditCannotBypassPendingGate(t *testing.T){
	pool:=reviewDB(t);repo:=Repository{DB:pool};ctx:=context.Background();userID:=createUnverifiedConsumer(t,pool,"unverified-edit");placeID:=createPlace(t,pool,"Unverified edit place")
	first,err:=repo.Upsert(ctx,userID,placeID,2,"Первый отзыв нового аккаунта ожидает проверки.");if err!=nil{t.Fatal(err)};if first.Status!="PENDING"{t.Fatalf("first status=%s",first.Status)}
	second,err:=repo.Upsert(ctx,userID,placeID,5,"Редактирование не должно обходить trust moderation.");if err!=nil{t.Fatal(err)};if second.Status!="PENDING"{t.Fatalf("edited status=%s",second.Status)}
	stats,err:=repo.Stats(ctx,placeID);if err!=nil{t.Fatal(err)};if stats.Count!=0{t.Fatalf("unverified pending review affects rating: %+v",stats)}
}
