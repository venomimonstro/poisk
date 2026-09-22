package datahub

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"unicode"
)

var ErrInvalid=errors.New("invalid data hub input")

type PageType string
const(
	PageCity PageType="CITY"
	PageCategory PageType="CATEGORY"
	PageCityCategory PageType="CITY_CATEGORY"
	PageOrganization PageType="ORGANIZATION"
	PageWebsite PageType="WEBSITE"
)

type Evidence struct{
	Organizations int
	WithWebsite int
	WithAddress int
	AverageQuality float64
	DistinctSources int
}

type Gate struct{
	Publish bool `json:"publish"`
	Score int `json:"score"`
	Reason string `json:"reason"`
}

func StableSegment(key string)string{
	key=strings.ToLower(strings.TrimSpace(key));if key==""{return ""}
	var b strings.Builder;dash:=false
	for _,r:=range key{
		if (r>='a'&&r<='z')||(r>='0'&&r<='9')||r=='.'||r=='_'||r=='~'{b.WriteRune(r);dash=false;continue}
		if r=='-'||unicode.IsSpace(r){if b.Len()>0&&!dash{b.WriteByte('-');dash=true}}
	}
	out:=strings.Trim(b.String(),"-")
	if out!=""&&len(out)<=160{return out}
	sum:=sha256.Sum256([]byte(key));return "k-"+hex.EncodeToString(sum[:8])
}

func Identity(t PageType,cityKey,categoryKey string,placeID,domainID int64)(slug,path string,err error){
	city:=StableSegment(cityKey);cat:=StableSegment(categoryKey)
	switch t{
	case PageCity:
		if city==""||categoryKey!=""||placeID!=0||domainID!=0{return "","",ErrInvalid};return city,"/data/city/"+city,nil
	case PageCategory:
		if cat==""||cityKey!=""||placeID!=0||domainID!=0{return "","",ErrInvalid};return cat,"/data/category/"+cat,nil
	case PageCityCategory:
		if city==""||cat==""||placeID!=0||domainID!=0{return "","",ErrInvalid};slug=city+"~"+cat;return slug,"/data/city/"+city+"/"+cat,nil
	case PageOrganization:
		if placeID<=0||cityKey!=""||categoryKey!=""||domainID!=0{return "","",ErrInvalid};slug=fmt.Sprintf("org-%d",placeID);return slug,"/data/organization/"+slug,nil
	case PageWebsite:
		if domainID<=0||cityKey!=""||categoryKey!=""||placeID!=0{return "","",ErrInvalid};slug=fmt.Sprintf("site-%d",domainID);return slug,"/data/website/"+slug,nil
	default:return "","",ErrInvalid
	}
}

func Evaluate(t PageType,e Evidence)Gate{
	if e.Organizations<0||e.WithWebsite<0||e.WithAddress<0||e.DistinctSources<0{return Gate{Reason:"invalid_evidence"}}
	q:=e.AverageQuality;if q<0{q=0};if q>100{q=100}
	score:=0
	if e.Organizations>0{score+=min(50,e.Organizations*5)}
	score+=min(15,e.WithWebsite*3)
	score+=min(15,e.WithAddress*3)
	score+=min(10,e.DistinctSources*2)
	score+=int(q/10)
	if score>100{score=100}
	minOrg:=5
	switch t{case PageCity:minOrg=10;case PageCategory:minOrg=10;case PageCityCategory:minOrg=5;case PageOrganization:minOrg=1;case PageWebsite:minOrg=1;default:return Gate{Reason:"invalid_page_type"}}
	if e.Organizations<minOrg{return Gate{Score:score,Reason:"insufficient_organizations"}}
	if t==PageCityCategory||t==PageCity||t==PageCategory{
		if e.WithAddress<minOrg/2{return Gate{Score:score,Reason:"insufficient_address_evidence"}}
		if e.AverageQuality<35{return Gate{Score:score,Reason:"low_average_quality"}}
	}
	if score<45{return Gate{Score:score,Reason:"low_evidence_score"}}
	return Gate{Publish:true,Score:score,Reason:"ok"}
}

func min(a,b int)int{if a<b{return a};return b}
