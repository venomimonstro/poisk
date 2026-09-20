package manticore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

func (c *Client) EnsureAddressesSchema(ctx context.Context) error {
	if _,err:=c.execSQL(ctx,AddressesSchemaSQL);err!=nil{return err}
	if _,err:=c.execSQL(ctx,"DESC "+AddressesIndex);err!=nil{return fmt.Errorf("describe addresses index: %w",err)}
	return nil
}

func (c *Client) ResetAddressesSchema(ctx context.Context) error {
	if _,err:=c.execSQL(ctx,"DROP TABLE IF EXISTS "+AddressesIndex);err!=nil{return fmt.Errorf("drop addresses index: %w",err)}
	return c.EnsureAddressesSchema(ctx)
}

func (c *Client) CurrentAddressVersion(ctx context.Context,id int64)(int64,bool,error){
	if id<=0{return 0,false,errors.New("address id must be positive")}
	body,err:=c.execSQL(ctx,"SELECT entity_version FROM "+AddressesIndex+" WHERE id="+strconv.FormatInt(id,10)+" LIMIT 1")
	if err!=nil{return 0,false,err}
	var sets []rawResultSet
	if err:=json.Unmarshal(body,&sets);err!=nil{return 0,false,fmt.Errorf("decode address version: %w",err)}
	if len(sets)==0||len(sets[0].Data)==0{return 0,false,nil}
	raw,ok:=sets[0].Data[0]["entity_version"];if !ok{return 0,false,errors.New("address version missing")}
	var version int64
	if err:=json.Unmarshal(raw,&version);err==nil{return version,true,nil}
	var text string;if err:=json.Unmarshal(raw,&text);err!=nil{return 0,false,err}
	version,err=strconv.ParseInt(text,10,64);if err!=nil{return 0,false,err}
	return version,true,nil
}

func (c *Client) ApplyAddress(ctx context.Context,doc AddressDocument)(bool,error){
	if doc.ID<=0||doc.EntityVersion<=0{return false,errors.New("address id and version must be positive")}
	if doc.DisplayName==""||doc.FullAddress==""||doc.NormalizedName==""{return false,errors.New("address display fields are required")}
	current,exists,err:=c.CurrentAddressVersion(ctx,doc.ID);if err!=nil{return false,err}
	if exists&&current>doc.EntityVersion{return false,nil}
	q:="REPLACE INTO "+AddressesIndex+" (id,display_name,full_address,normalized_name,region_code,object_kind,level,parent_address_id,latitude,longitude,has_location,entity_version,status) VALUES ("+
		strconv.FormatInt(doc.ID,10)+","+quote(doc.DisplayName)+","+quote(doc.FullAddress)+","+quote(doc.NormalizedName)+","+strconv.Itoa(doc.RegionCode)+","+quote(doc.ObjectKind)+","+strconv.Itoa(doc.Level)+","+strconv.FormatInt(doc.ParentAddressID,10)+","+strconv.FormatFloat(doc.Latitude,'f',7,64)+","+strconv.FormatFloat(doc.Longitude,'f',7,64)+","+boolSQL(doc.HasLocation)+","+strconv.FormatInt(doc.EntityVersion,10)+","+quote(doc.Status)+")"
	_,err=c.execSQL(ctx,q)
	return err==nil,err
}

func (c *Client) DeleteAddress(ctx context.Context,id,version int64) error {
	if id<=0||version<=0{return errors.New("address id and version must be positive")}
	current,exists,err:=c.CurrentAddressVersion(ctx,id);if err!=nil{return err}
	if exists&&current>version{return nil}
	_,err=c.execSQL(ctx,"DELETE FROM "+AddressesIndex+" WHERE id="+strconv.FormatInt(id,10))
	return err
}
