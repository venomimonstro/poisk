//go:build integration

package mail

import (
	"context"
	"testing"
	"time"
)

func TestLeaseRechecksActiveSuppressionBeforeMTA(t *testing.T){
	pool:=mailDB(t);repo:=Repository{DB:pool};ctx:=context.Background();seed:=seedOutboundDelivery(t,repo);now:=time.Now().UTC()
	hash,err:=deliveryAddressHash(seed.Address);if err!=nil{t.Fatal(err)}
	if _,err=pool.Exec(ctx,`INSERT INTO mail_delivery_suppressions(sender_mailbox_id,address_sha256,reason,hard_bounce_count,first_bounced_at,last_bounced_at,suppressed_until,cleared_at,updated_at)
VALUES($1,$2,'HARD_BOUNCE',2,$3,$3,$4,NULL,$3)
ON CONFLICT(sender_mailbox_id,address_sha256) DO UPDATE SET reason='HARD_BOUNCE',hard_bounce_count=2,first_bounced_at=$3,last_bounced_at=$3,suppressed_until=$4,cleared_at=NULL,updated_at=$3`,seed.SenderMailboxID,hash,now,now.Add(time.Hour));err!=nil{t.Fatal(err)}
	items,err:=repo.LeaseOutbound(ctx,"suppression-race",1,time.Minute,now);if err!=nil{t.Fatal(err)};if len(items)!=0{t.Fatalf("suppressed delivery reached worker: %+v",items)}
	var status,code string;if err=pool.QueryRow(ctx,`SELECT status,COALESCE(last_error_code,'') FROM mail_outbound_deliveries WHERE delivery_id=$1`,seed.ID).Scan(&status,&code);err!=nil{t.Fatal(err)}
	if status!="DEAD"||code!="RECIPIENT_SUPPRESSED"{t.Fatalf("status=%s code=%s",status,code)}
}

func TestMaintainDeliverabilityReconcilesStaleSubmittedWithoutAutoResend(t *testing.T){
	pool:=mailDB(t);repo:=Repository{DB:pool};ctx:=context.Background();seed:=seedOutboundDelivery(t,repo);base:=time.Now().UTC().Add(-25*time.Hour)
	items,err:=repo.LeaseOutbound(ctx,"stale-submitted",1,time.Minute,base);if err!=nil||len(items)!=1{t.Fatalf("lease=%+v err=%v",items,err)}
	if err=repo.MarkOutboundSubmitted(ctx,seed.ID,"stale-submitted","remote-stale",base);err!=nil{t.Fatal(err)}
	result,err:=repo.MaintainDeliverability(ctx,base.Add(25*time.Hour));if err!=nil{t.Fatal(err)};if result.StaleSubmitted!=1{t.Fatalf("stale submitted=%d",result.StaleSubmitted)}
	var status,code string;if err=pool.QueryRow(ctx,`SELECT status,COALESCE(last_error_code,'') FROM mail_outbound_deliveries WHERE delivery_id=$1`,seed.ID).Scan(&status,&code);err!=nil{t.Fatal(err)}
	if status!="DEAD"||code!="SUBMITTED_STALE"{t.Fatalf("status=%s code=%s",status,code)}
	leased,err:=repo.LeaseOutbound(ctx,"stale-submitted",1,time.Minute,time.Now().UTC());if err!=nil{t.Fatal(err)};if len(leased)!=0{t.Fatalf("stale submitted was automatically resent: %+v",leased)}
}

func TestDNSReadinessSnapshotsDetectOnlyStateChange(t *testing.T){
	pool:=mailDB(t);repo:=Repository{DB:pool};ctx:=context.Background();domain:="dns-drift.example.test";selector:="poisk1";base:=time.Now().UTC()
	ready:=DNSReadiness{Domain:domain,Selector:selector,MX:true,SPF:true,DMARC:true,DKIM:true,Ready:true}
	first,err:=repo.RecordDNSReadiness(ctx,ready,base);if err!=nil{t.Fatal(err)};if first.Drift{t.Fatal("first snapshot must not be drift")}
	second,err:=repo.RecordDNSReadiness(ctx,ready,base.Add(time.Minute));if err!=nil{t.Fatal(err)};if second.Drift{t.Fatal("identical snapshot must not drift")}
	changed:=ready;changed.DKIM=false;changed.Ready=false;changed.Reasons=[]string{"dkim_missing_or_mismatch"}
	third,err:=repo.RecordDNSReadiness(ctx,changed,base.Add(2*time.Minute));if err!=nil{t.Fatal(err)};if !third.Drift{t.Fatal("changed readiness must set drift")}
	latest,err:=repo.LatestDNSReadiness(ctx,domain,selector);if err!=nil{t.Fatal(err)};if latest==nil||latest.ID!=third.ID||latest.Ready||!latest.Drift{t.Fatalf("latest=%+v",latest)}
}
