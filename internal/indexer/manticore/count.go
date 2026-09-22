package manticore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

func (c *Client) CountDocuments(ctx context.Context)(int64,error){
	body,err:=c.execSQL(ctx,"SELECT COUNT(*) AS documents FROM "+WebIndex);if err!=nil{return 0,err}
	var sets []rawResultSet;if err:=json.Unmarshal(body,&sets);err!=nil{return 0,fmt.Errorf("decode document count: %w",err)}
	if len(sets)==0||len(sets[0].Data)==0{return 0,errors.New("manticore document count result is empty")}
	raw,ok:=sets[0].Data[0]["documents"];if !ok{return 0,errors.New("manticore document count missing documents field")}
	var count int64;if err:=json.Unmarshal(raw,&count);err==nil{if count<0{return 0,errors.New("negative manticore document count")};return count,nil}
	var text string;if err:=json.Unmarshal(raw,&text);err!=nil{return 0,fmt.Errorf("decode manticore document count: %w",err)}
	count,err=strconv.ParseInt(text,10,64);if err!=nil||count<0{return 0,errors.New("invalid manticore document count")};return count,nil
}
