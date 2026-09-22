package mail

import (
	"context"
	"errors"
	"strings"
	"time"
)

type GatewayWorker struct {
	Repo Repository
	MTA MTAClient
	BlobRoot string
	WorkerID string
	BatchSize int
	Lease time.Duration
	Poll time.Duration
}

func (r Repository) MarkOutboundDead(ctx context.Context,deliveryID int64,worker,code,detail string,now time.Time)error{
	if r.DB==nil||deliveryID<=0||strings.TrimSpace(worker)==""{return ErrInvalid};if now.IsZero(){now=time.Now().UTC()};code=strings.TrimSpace(code);detail=strings.TrimSpace(detail);if len(code)>80||len(detail)>1000{return ErrInvalid}
	tx,err:=r.DB.Begin(ctx);if err!=nil{return err};defer func(){_=tx.Rollback(ctx)}();var attempt int
	if err=tx.QueryRow(ctx,`SELECT attempts FROM mail_outbound_deliveries WHERE delivery_id=$1 AND status='LEASED' AND lease_owner=$2 FOR UPDATE`,deliveryID,worker).Scan(&attempt);err!=nil{return ErrConflict}
	if _,err=tx.Exec(ctx,`UPDATE mail_outbound_deliveries SET status='DEAD',lease_owner=NULL,lease_until=NULL,last_error_code=NULLIF($3,''),last_error_detail=NULLIF($4,''),updated_at=$5 WHERE delivery_id=$1 AND lease_owner=$2`,deliveryID,worker,code,detail,now);err!=nil{return err}
	if _,err=tx.Exec(ctx,`INSERT INTO mail_outbound_events(delivery_id,action,attempt,details) VALUES($1,'DEAD',$2,jsonb_build_object('code',NULLIF($3,'')))`,deliveryID,attempt,code);err!=nil{return err}
	if _,err=tx.Exec(ctx,`INSERT INTO mail_gateway_events(direction,event_type,delivery_id,code,details) VALUES('OUTBOUND','REJECT',$1,NULLIF($2,''),jsonb_build_object('detail',NULLIF($3,'')))`,deliveryID,code,detail);err!=nil{return err};return tx.Commit(ctx)
}

func (w GatewayWorker) Run(ctx context.Context)error{
	if w.Repo.DB==nil||strings.TrimSpace(w.WorkerID)==""||strings.TrimSpace(w.BlobRoot)==""{return ErrInvalid};if w.BatchSize<=0{w.BatchSize=16};if w.BatchSize>100{w.BatchSize=100};if w.Lease<=0{w.Lease=45*time.Second};if w.Poll<=0{w.Poll=time.Second}
	ticker:=time.NewTicker(w.Poll);defer ticker.Stop()
	for{
		if err:=w.Repo.RecoverExpiredOutboundLeases(ctx,time.Now().UTC());err!=nil&&ctx.Err()==nil{return err}
		items,err:=w.Repo.LeaseOutbound(ctx,w.WorkerID,w.BatchSize,w.Lease,time.Now().UTC());if err!=nil{if ctx.Err()!=nil{return ctx.Err()};return err}
		for _,delivery:=range items{if err:=w.process(ctx,delivery);err!=nil&&ctx.Err()!=nil{return ctx.Err()}}
		select{case<-ctx.Done():return ctx.Err();case<-ticker.C:}
	}
}

func (w GatewayWorker) process(ctx context.Context,d OutboundDelivery)error{
	envelope,err:=w.Repo.LoadOutboundEnvelope(ctx,d,w.BlobRoot);if err!=nil{
		code:="ENVELOPE_INVALID";detail:=err.Error();if errors.Is(err,ErrRateLimited){code="MESSAGE_TOO_LARGE"};return w.Repo.MarkOutboundDead(ctx,d.ID,w.WorkerID,code,detail,time.Now().UTC())
	}
	remoteID,err:=w.MTA.Submit(ctx,envelope);if err==nil{return w.Repo.MarkOutboundSubmitted(ctx,d.ID,w.WorkerID,remoteID,time.Now().UTC())}
	var mtaErr *MTAError;if errors.As(err,&mtaErr){if mtaErr.Permanent{return w.Repo.MarkOutboundDead(ctx,d.ID,w.WorkerID,mtaErr.Code,mtaErr.Detail,time.Now().UTC())};return w.Repo.RetryOutbound(ctx,d.ID,w.WorkerID,mtaErr.Code,mtaErr.Detail,time.Now().UTC())}
	return w.Repo.RetryOutbound(ctx,d.ID,w.WorkerID,"MTA_ERROR",err.Error(),time.Now().UTC())
}
