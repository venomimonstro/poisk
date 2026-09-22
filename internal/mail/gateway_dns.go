package mail

import (
	"context"
	"errors"
	"net"
	"strings"
)

type DNSResolver interface {
	LookupMX(context.Context,string)([]*net.MX,error)
	LookupTXT(context.Context,string)([]string,error)
}

type DNSReadiness struct {
	Domain string `json:"domain"`
	Selector string `json:"selector"`
	MX bool `json:"mx"`
	SPF bool `json:"spf"`
	DMARC bool `json:"dmarc"`
	DKIM bool `json:"dkim"`
	Ready bool `json:"ready"`
	Reasons []string `json:"reasons,omitempty"`
}

func CheckInternetMailDNS(ctx context.Context,resolver DNSResolver,domain,selector,expectedDKIM string)(DNSReadiness,error){
	if resolver==nil{resolver=net.DefaultResolver};domain=strings.ToLower(strings.TrimSuffix(strings.TrimSpace(domain),"."));selector=strings.ToLower(strings.TrimSpace(selector));expectedDKIM=compactTXT(expectedDKIM)
	out:=DNSReadiness{Domain:domain,Selector:selector};if domain==""||selector==""||strings.ContainsAny(domain+selector," /:@?#\\"){return out,ErrInvalid}
	mx,err:=resolver.LookupMX(ctx,domain);if err!=nil{if !dnsNotFound(err){return out,err}};out.MX=len(mx)>0;if !out.MX{out.Reasons=append(out.Reasons,"mx_missing")}
	txt,err:=resolver.LookupTXT(ctx,domain);if err!=nil&& !dnsNotFound(err){return out,err};for _,v:=range txt{if strings.HasPrefix(strings.ToLower(compactTXT(v)),"v=spf1"){out.SPF=true;break}};if !out.SPF{out.Reasons=append(out.Reasons,"spf_missing")}
	dmarc,err:=resolver.LookupTXT(ctx,"_dmarc."+domain);if err!=nil&&!dnsNotFound(err){return out,err};for _,v:=range dmarc{if strings.HasPrefix(strings.ToLower(compactTXT(v)),"v=dmarc1"){out.DMARC=true;break}};if !out.DMARC{out.Reasons=append(out.Reasons,"dmarc_missing")}
	dkim,err:=resolver.LookupTXT(ctx,selector+"._domainkey."+domain);if err!=nil&&!dnsNotFound(err){return out,err};for _,v:=range dkim{value:=compactTXT(v);if strings.HasPrefix(strings.ToLower(value),"v=dkim1")&&(expectedDKIM==""||strings.EqualFold(value,expectedDKIM)){out.DKIM=true;break}};if !out.DKIM{out.Reasons=append(out.Reasons,"dkim_missing_or_mismatch")}
	out.Ready=out.MX&&out.SPF&&out.DMARC&&out.DKIM;return out,nil
}

func compactTXT(v string)string{return strings.Join(strings.Fields(strings.TrimSpace(v)),"")}
func dnsNotFound(err error)bool{var dnsErr *net.DNSError;return errors.As(err,&dnsErr)&&dnsErr.IsNotFound}
