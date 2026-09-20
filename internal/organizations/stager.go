package organizations

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

type StageStore interface {
	StageValid(context.Context,int64,int64,NormalizedRow)(int64,error)
	StageRejected(context.Context,int64,int64,string,[]byte,string,string)(int64,error)
	FinishStaging(context.Context,int64) error
}

type RowAdapter interface { Decode([]byte)(SourceRow,error) }

type JSONRowAdapter struct{}
func (JSONRowAdapter) Decode(raw []byte)(SourceRow,error){
	dec:=json.NewDecoder(bytes.NewReader(raw));dec.DisallowUnknownFields()
	var row SourceRow
	if err:=dec.Decode(&row);err!=nil{return SourceRow{},err}
	var extra any;if err:=dec.Decode(&extra);err!=io.EOF{return SourceRow{},ErrInvalidRow}
	return row,nil
}

type Stager struct { Store StageStore; Adapter RowAdapter }

func (s Stager) StageJSONL(ctx context.Context,batchID int64,input io.Reader)error{
	if s.Store==nil||input==nil||batchID<=0{return ErrInvalidBatch}
	adapter:=s.Adapter;if adapter==nil{adapter=JSONRowAdapter{}}
	reader:=bufio.NewReaderSize(input,16<<10)
	var rowNumber int64
	for{
		raw,tooLarge,err:=readBoundedLine(reader,maxRawPayloadBytes)
		if err!=nil && !errors.Is(err,io.EOF){return err}
		if len(raw)>0||tooLarge{
			rowNumber++
			if tooLarge{
				if _,stageErr:=s.Store.StageRejected(ctx,batchID,rowNumber,"",[]byte("source row exceeded 65536 byte limit"),"ROW_TOO_LARGE","");stageErr!=nil{return stageErr}
			}else{
				raw=bytes.TrimSpace(raw)
				if len(raw)==0{
					if _,stageErr:=s.Store.StageRejected(ctx,batchID,rowNumber,"",[]byte("blank source row"),"BLANK_ROW","");stageErr!=nil{return stageErr}
				}else{
					row,decodeErr:=adapter.Decode(raw)
					if decodeErr!=nil{
						if _,stageErr:=s.Store.StageRejected(ctx,batchID,rowNumber,"",raw,"INVALID_JSON",truncateRunesOrg(decodeErr.Error(),512));stageErr!=nil{return stageErr}
					}else{
						normalized,normErr:=NormalizeSourceRow(row)
						if normErr!=nil{
							if _,stageErr:=s.Store.StageRejected(ctx,batchID,rowNumber,strings.TrimSpace(row.SourceRecordID),raw,"INVALID_ROW",truncateRunesOrg(normErr.Error(),512));stageErr!=nil{return stageErr}
						}else if _,stageErr:=s.Store.StageValid(ctx,batchID,rowNumber,normalized);stageErr!=nil{return stageErr}
					}
				}
			}
		}
		if errors.Is(err,io.EOF){break}
		if ctx.Err()!=nil{return ctx.Err()}
	}
	return s.Store.FinishStaging(ctx,batchID)
}

func readBoundedLine(reader *bufio.Reader,limit int)([]byte,bool,error){
	if limit<=0{return nil,false,ErrInvalidRow}
	out:=make([]byte,0,minIntOrg(limit,4096));tooLarge:=false
	for{
		part,err:=reader.ReadSlice('\n')
		if !tooLarge{
			remaining:=limit-len(out)
			if len(part)>remaining{tooLarge=true}else{out=append(out,part...)}
		}
		if err==nil{return out,tooLarge,nil}
		if errors.Is(err,bufio.ErrBufferFull){continue}
		if errors.Is(err,io.EOF){return out,tooLarge,io.EOF}
		return nil,false,err
	}
}

func truncateRunesOrg(value string,max int)string{
	value=strings.TrimSpace(value);if max<=0{return ""}
	runes:=[]rune(value);if len(runes)>max{return string(runes[:max])};return value
}
func minIntOrg(a,b int)int{if a<b{return a};return b}
