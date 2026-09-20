package manticore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type OrganizationSearchRequest struct {
	Text string
	CityKey string
	CategoryKey string
	Limit int
	HasBounds bool
	MinLat float64
	MaxLat float64
	MinLon float64
	MaxLon float64
}

type OrganizationHit struct {
	ID int64
	Score float64
	Name string
	Address string
	CityKey string
	CategoryKey string
	Phone string
	Website string
	Latitude float64
	Longitude float64
	HasLocation bool
	QualityScore float64
	SourceCount int
}

func (c *Client) SearchOrganizations(ctx context.Context,req OrganizationSearchRequest)([]OrganizationHit,error){
	if c==nil{return nil,errors.New("manticore client is nil")}
	if req.Limit<=0||req.Limit>200{return nil,errors.New("organization search limit must be between 1 and 200")}
	if req.HasBounds&&(req.MinLat < -90||req.MaxLat>90||req.MinLat>req.MaxLat||req.MinLon < -180||req.MinLon>180||req.MaxLon < -180||req.MaxLon>180){return nil,errors.New("invalid organization bounds")}
	where:=[]string{"status='ACTIVE'"}
	if strings.TrimSpace(req.Text)!=""{where=append(where,"MATCH("+quote(strings.TrimSpace(req.Text))+")")}
	if req.CityKey!=""{where=append(where,"city_key="+quote(req.CityKey))}
	if req.CategoryKey!=""{where=append(where,"category_key="+quote(req.CategoryKey))}
	if req.HasBounds{
		where=append(where,"has_location=1")
		where=append(where,"latitude BETWEEN "+floatSQL(req.MinLat)+" AND "+floatSQL(req.MaxLat))
		if req.MinLon<=req.MaxLon{
			where=append(where,"longitude BETWEEN "+floatSQL(req.MinLon)+" AND "+floatSQL(req.MaxLon))
		}else{
			where=append(where,"(longitude >= "+floatSQL(req.MinLon)+" OR longitude <= "+floatSQL(req.MaxLon)+")")
		}
	}
	order:="quality_score DESC,id ASC"
	if strings.TrimSpace(req.Text)!=""{order="score DESC,quality_score DESC,id ASC"}
	query:="SELECT id,WEIGHT() AS score,name,address,city_key,category_key,phone,website,latitude,longitude,has_location,quality_score,source_count FROM "+OrganizationsIndex+
		" WHERE "+strings.Join(where," AND ")+" ORDER BY "+order+" LIMIT "+strconv.Itoa(req.Limit)
	body,err:=c.execSQL(ctx,query);if err!=nil{return nil,err}
	var sets []rawResultSet
	if err:=json.Unmarshal(body,&sets);err!=nil{return nil,fmt.Errorf("decode organization search: %w",err)}
	if len(sets)==0{return nil,nil}
	out:=make([]OrganizationHit,0,len(sets[0].Data))
	for _,row:=range sets[0].Data{
		hit:=OrganizationHit{}
		if hit.ID,err=rawInt64(row["id"]);err!=nil{return nil,err}
		hit.Score,_=rawFloat(row["score"])
		hit.Name,_=rawString(row["name"]);hit.Address,_=rawString(row["address"]);hit.CityKey,_=rawString(row["city_key"]);hit.CategoryKey,_=rawString(row["category_key"])
		hit.Phone,_=rawString(row["phone"]);hit.Website,_=rawString(row["website"]);hit.Latitude,_=rawFloat(row["latitude"]);hit.Longitude,_=rawFloat(row["longitude"])
		has,_:=rawInt64(row["has_location"]);hit.HasLocation=has!=0
		hit.QualityScore,_=rawFloat(row["quality_score"]);sources,_:=rawInt64(row["source_count"]);hit.SourceCount=int(sources)
		out=append(out,hit)
	}
	return out,nil
}

func floatSQL(value float64)string{return strconv.FormatFloat(value,'f',7,64)}
func rawString(raw json.RawMessage)(string,error){if len(raw)==0{return "",nil};var s string;if err:=json.Unmarshal(raw,&s);err==nil{return s,nil};return "",errors.New("invalid string field")}
func rawFloat(raw json.RawMessage)(float64,error){if len(raw)==0{return 0,nil};var f float64;if err:=json.Unmarshal(raw,&f);err==nil{return f,nil};var s string;if err:=json.Unmarshal(raw,&s);err!=nil{return 0,err};return strconv.ParseFloat(s,64)}
func rawInt64(raw json.RawMessage)(int64,error){if len(raw)==0{return 0,nil};var i int64;if err:=json.Unmarshal(raw,&i);err==nil{return i,nil};var f float64;if err:=json.Unmarshal(raw,&f);err==nil{return int64(f),nil};var s string;if err:=json.Unmarshal(raw,&s);err!=nil{return 0,err};return strconv.ParseInt(s,10,64)}
