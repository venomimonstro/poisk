package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/venomimonstro/poisk/internal/admin"
)

func newAdminService(pool *pgxpool.Pool)(*admin.Service,error){
	raw:=strings.TrimSpace(os.Getenv("ADMIN_SECRET_KEY_B64"));if raw==""{return nil,errors.New("ADMIN_SECRET_KEY_B64 is required")}
	key,err:=base64.RawStdEncoding.DecodeString(raw);if err!=nil{key,err=base64.StdEncoding.DecodeString(raw)};if err!=nil||len(key)!=32{return nil,errors.New("ADMIN_SECRET_KEY_B64 must decode to 32 bytes")}
	box,err:=admin.NewSecretBox(key);if err!=nil{return nil,err};return &admin.Service{Store:admin.NewRepository(pool),Secrets:box,SessionTTL:12*time.Hour},nil
}

func runAdminCtl(ctx context.Context,pool *pgxpool.Pool,args []string)error{
	if len(args)==0{return errors.New("usage: adminctl create|setup-2fa|revoke-sessions")}
	service,err:=newAdminService(pool);if err!=nil{return err}
	switch args[0]{
	case "create":
		if len(args)!=4{return errors.New("usage: adminctl create <email> <role> <password>")}
		hash,err:=admin.HashPassword(args[3],admin.DefaultArgonParams());if err!=nil{return err};created,err:=service.Store.CreateAdmin(ctx,args[1],hash,args[2]);if err!=nil{return err};return json.NewEncoder(os.Stdout).Encode(map[string]any{"admin_id":created.ID,"email":created.Email,"role":created.Role})
	case "setup-2fa":
		if len(args)!=2{return errors.New("usage: adminctl setup-2fa <admin-id>")};id,err:=strconv.ParseInt(args[1],10,64);if err!=nil||id<=0{return errors.New("invalid admin id")};secret,codes,err:=service.SetupTOTP(ctx,id);if err!=nil{return err};return json.NewEncoder(os.Stdout).Encode(map[string]any{"admin_id":id,"totp_secret":secret,"recovery_codes":codes,"warning":"shown once; store securely"})
	case "revoke-sessions":
		if len(args)!=2{return errors.New("usage: adminctl revoke-sessions <admin-id>")};id,err:=strconv.ParseInt(args[1],10,64);if err!=nil||id<=0{return errors.New("invalid admin id")};if err:=service.Store.RevokeAllSessions(ctx,id);err!=nil{return err};return nil
	default:return fmt.Errorf("unknown adminctl command %q",args[0])
	}
}
