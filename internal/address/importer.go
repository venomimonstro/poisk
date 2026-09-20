package address

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/venomimonstro/poisk/internal/address/gar"
)

type ImportStore interface {
	Batch(context.Context,int64)(Batch,error)
	Stage(context.Context,int64,string,int64,gar.Normalized)(int64,error)
	Reject(context.Context,int64,string,int64,gar.Kind,int,map[string]string,string,string)(int64,error)
	AdvanceCheckpoint(context.Context,int64,string,int64) error
}

type Importer struct { Store ImportStore; CheckpointEvery int64 }

type FileSpec struct {
	Name string
	RegionCode int
	Kind gar.Kind
}

func (i Importer) ImportFile(ctx context.Context,batchID int64,spec FileSpec,reader io.Reader)error{
	if i.Store==nil||reader==nil||batchID<=0{return gar.ErrInvalidRecord}
	spec.Name=strings.TrimSpace(spec.Name);if spec.Name==""||len(spec.Name)>512{return gar.ErrInvalidRecord}
	batch,err:=i.Store.Batch(ctx,batchID);if err!=nil{return err}
	if batch.Status!="STAGING"{return ErrBatchConflict}
	resumeRow:=int64(0)
	if batch.CheckpointFile==spec.Name{resumeRow=batch.CheckpointRow}
	checkpointEvery:=i.CheckpointEvery;if checkpointEvery<=0{checkpointEvery=1000}
	lastSeen:=resumeRow
	decoder:=gar.Decoder{RegionCode:spec.RegionCode,Kind:spec.Kind}
	err=decoder.StreamRows(reader,func(row gar.Row)error{
		if row.Number<=resumeRow{return nil}
		if err:=ctx.Err();err!=nil{return err}
		lastSeen=row.Number
		if row.Err!=nil{
			if _,err:=i.Store.Reject(ctx,batchID,spec.Name,row.Number,spec.Kind,spec.RegionCode,row.Raw,"INVALID_ROW",row.Err.Error());err!=nil{return err}
		}else if row.Normalized!=nil{
			if _,err:=i.Store.Stage(ctx,batchID,spec.Name,row.Number,*row.Normalized);err!=nil{return err}
		}else{return errors.New("GAR decoder returned empty row")}
		if row.Number%checkpointEvery==0{return i.Store.AdvanceCheckpoint(ctx,batchID,spec.Name,row.Number)}
		return nil
	})
	if err!=nil{return err}
	return i.Store.AdvanceCheckpoint(ctx,batchID,spec.Name,lastSeen)
}
