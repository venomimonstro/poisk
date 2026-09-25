package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	mailcore "github.com/venomimonstro/poisk/internal/mail"
)

func runMailCtl(ctx context.Context,args []string)error{
	if len(args)==0{return errors.New("usage: app mailctl dns-check")}
	switch args[0]{
	case "dns-check":
		domain:=strings.TrimSpace(os.Getenv("MAIL_DOMAIN"))
		selector:=strings.TrimSpace(os.Getenv("MAIL_DKIM_SELECTOR"))
		expected:=strings.TrimSpace(os.Getenv("MAIL_DKIM_PUBLIC_TXT"))
		if domain==""||selector==""{return errors.New("MAIL_DOMAIN and MAIL_DKIM_SELECTOR are required")}
		checkCtx,cancel:=context.WithTimeout(ctx,10*time.Second);defer cancel()
		result,err:=mailcore.CheckInternetMailDNS(checkCtx,net.DefaultResolver,domain,selector,expected);if err!=nil{return fmt.Errorf("mail DNS readiness check: %w",err)}
		enc:=json.NewEncoder(os.Stdout);enc.SetIndent("","  ");if err:=enc.Encode(result);err!=nil{return fmt.Errorf("encode mail DNS readiness: %w",err)}
		if !result.Ready{return fmt.Errorf("mail DNS is not ready: %s",strings.Join(result.Reasons,","))}
		return nil
	default:
		return fmt.Errorf("unknown mailctl command %q; usage: app mailctl dns-check",args[0])
	}
}
