package manticore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

func (c *Client) EnsureOrganizationsSchema(ctx context.Context) error {
	if _,err:=c.execSQL(ctx,OrganizationsSchemaSQL);err!=nil{return err}
	_,err:=c.execSQL(ctx,"DESC "+OrganizationsIndex)
	if err!=nil{return fmt.Errorf("describe organizations index: %w",err)}
	return nil
}

func (c *Client) CurrentOrganizationVersion(ctx context.Context,id int64)(int64,bool,error){
	if id<=0{return 0,false,errors.New("organization id must be positive")}
	body,err:=c.execSQL(ctx,"SELECT entity_version FROM "+OrganizationsIndex+" WHERE id="+strconv.FormatInt(id,10)+" LIMIT 1")
	if err!=nil{return 0,false,err}
	var sets []rawResultSet
	if err:=json.Unmarshal(body,&sets);err!=nil{return 0,false,fmt.Errorf("decode organization version: %w",err)}
	if len(sets)==0||len(sets[0].Data)==0{return 0,false,nil}
	raw,ok:=sets[0].Data[0]["entity_version"];if !ok{return 0,false,errors.New("organization version missing")}
	var version int64
	if err:=json.Unmarshal(raw,&version);err==nil{return version,true,nil}
	var text string;if err:=json.Unmarshal(raw,&text);err!=nil{return 0,false,err}
	version,err=strconv.ParseInt(text,10,64);if err!=nil{return 0,false,err}
	return version,true,nil
}

func (c *Client) ApplyOrganization(ctx context.Context,doc OrganizationDocument)(bool,error){
	if doc.ID<=0||doc.EntityVersion<=0{return false,errors.New("organization id and version must be positive")}
	if doc.Name==""||doc.NormalizedName==""{return false,errors.New("organization name is required")}
	if doc.SourceCount<0{return false,errors.New("organization source count cannot be negative")}
	current,exists,err:=c.CurrentOrganizationVersion(ctx,doc.ID);if err!=nil{return false,err}
	if exists&&current>doc.EntityVersion{return false,nil}
	q:="REPLACE INTO "+OrganizationsIndex+" (id,name,address,category_key,phone,website,normalized_name,normalized_address,latitude,longitude,has_location,quality_score,source_count,entity_version,status) VALUES ("+
		strconv.FormatInt(doc.ID,10)+","+quote(doc.Name)+","+quote(doc.Address)+","+quote(doc.CategoryKey)+","+quote(doc.Phone)+","+quote(doc.Website)+","+
		quote(doc.NormalizedName)+","+quote(doc.NormalizedAddress)+","+strconv.FormatFloat(doc.Latitude,'f',7,64)+","+strconv.FormatFloat(doc.Longitude,'f',7,64)+","+
		boolSQL(doc.HasLocation)+","+strconv.FormatFloat(clamp100(doc.QualityScore),'f',-1,64)+","+strconv.Itoa(doc.SourceCount)+","+
		strconv.FormatInt(doc.EntityVersion,10)+","+quote(doc.Status)+")"
	_,err=c.execSQL(ctx,q)
	return err==nil,err
}

func (c *Client) DeleteOrganization(ctx context.Context,id int64,version int64)error{
	if id<=0||version<=0{return errors.New("organization id and version must be positive")}
	current,exists,err:=c.CurrentOrganizationVersion(ctx,id);if err!=nil{return err}
	if exists&&current>version{return nil}
	_,err=c.execSQL(ctx,"DELETE FROM "+OrganizationsIndex+" WHERE id="+strconv.FormatInt(id,10))
	return err
}

func boolSQL(value bool)string{if value{return "1"};return "0"}
