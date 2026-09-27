package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/venomimonstro/poisk/internal/platform/config"
	"github.com/venomimonstro/poisk/internal/platform/migrate"
)

type freshInstallReport struct {
	Database string `json:"database"`
	ExpectedVersions []int64 `json:"expected_versions"`
	AppliedVersions []int64 `json:"applied_versions"`
	ExpectedSchema int64 `json:"expected_schema"`
	CoreTables []string `json:"core_tables"`
}

func runFreshInstallDatabaseCheck(ctx context.Context,cfg config.Config,expected []int64)(freshInstallReport,error){
	dsn,dbName,err:=freshInstallDSN();if err!=nil{return freshInstallReport{},err}
	candidateDB:=strings.ToLower(strings.TrimSpace(os.Getenv("POSTGRES_DB")));if candidateDB==""{candidateDB="poisk"}
	if strings.EqualFold(dbName,candidateDB){return freshInstallReport{},errors.New("fresh-install database must differ from candidate POSTGRES_DB")}
	pool,err:=pgxpool.New(ctx,dsn);if err!=nil{return freshInstallReport{},fmt.Errorf("connect fresh-install database: %w",err)};defer pool.Close()
	if err=pool.Ping(ctx);err!=nil{return freshInstallReport{},fmt.Errorf("ping fresh-install database: %w",err)}
	if err=assertFreshPublicSchema(ctx,pool);err!=nil{return freshInstallReport{},err}
	if err=migrate.Up(ctx,pool,cfg.MigrationsDir);err!=nil{return freshInstallReport{},fmt.Errorf("fresh-install migrations: %w",err)}
	applied,err:=readAppliedVersions(ctx,pool);if err!=nil{return freshInstallReport{},err}
	expectedCopy:=append([]int64(nil),expected...);sort.Slice(expectedCopy,func(i,j int)bool{return expectedCopy[i]<expectedCopy[j]})
	if !sameInt64s(expectedCopy,applied){return freshInstallReport{},fmt.Errorf("fresh-install migration set mismatch: expected=%v applied=%v",expectedCopy,applied)}
	core:=[]string{"domains","webmaster_users","consumer_users","organizations","organization_reviews","mailboxes","commercial_readiness_evidence"}
	for _,table:=range core{var exists bool;if err=pool.QueryRow(ctx,`SELECT to_regclass('public.'||$1) IS NOT NULL`,table).Scan(&exists);err!=nil{return freshInstallReport{},err};if !exists{return freshInstallReport{},fmt.Errorf("fresh-install core table %s is missing",table)}}
	return freshInstallReport{Database:dbName,ExpectedVersions:expectedCopy,AppliedVersions:applied,ExpectedSchema:expectedCopy[len(expectedCopy)-1],CoreTables:core},nil
}

func freshInstallDSN()(string,string,error){
	host:=strings.TrimSpace(os.Getenv("READINESS_FRESH_POSTGRES_HOST"));port:=strings.TrimSpace(os.Getenv("READINESS_FRESH_POSTGRES_PORT"));db:=strings.TrimSpace(os.Getenv("READINESS_FRESH_POSTGRES_DB"));user:=strings.TrimSpace(os.Getenv("READINESS_FRESH_POSTGRES_USER"));password:=os.Getenv("READINESS_FRESH_POSTGRES_PASSWORD");sslmode:=strings.TrimSpace(os.Getenv("READINESS_FRESH_POSTGRES_SSLMODE"))
	if port==""{port="5432"};if sslmode==""{sslmode="disable"}
	if host==""||db==""||user==""||password==""{return "","",errors.New("READINESS_FRESH_POSTGRES_HOST/DB/USER/PASSWORD are required")}
	p,err:=strconv.Atoi(port);if err!=nil||p<1||p>65535{return "","",errors.New("invalid READINESS_FRESH_POSTGRES_PORT")}
	lowerDB:=strings.ToLower(db);if !(strings.Contains(lowerDB,"readiness")||strings.Contains(lowerDB,"test")){return "","",errors.New("fresh-install database name must contain readiness or test")}
	if len(db)>63||strings.ContainsAny(db,"/\\\x00\r\n\t"){return "","",errors.New("invalid fresh-install database name")}
	u:=url.URL{Scheme:"postgres",User:url.UserPassword(user,password),Host:net.JoinHostPort(host,port),Path:"/"+db};q:=u.Query();q.Set("sslmode",sslmode);u.RawQuery=q.Encode();return u.String(),db,nil
}

func assertFreshPublicSchema(ctx context.Context,pool *pgxpool.Pool)error{
	rows,err:=pool.Query(ctx,`SELECT tablename FROM pg_tables WHERE schemaname='public' ORDER BY tablename`);if err!=nil{return err};defer rows.Close()
	var unexpected []string;for rows.Next(){var table string;if err:=rows.Scan(&table);err!=nil{return err};if table!="spatial_ref_sys"{unexpected=append(unexpected,table)}};if err:=rows.Err();err!=nil{return err};if len(unexpected)>0{return fmt.Errorf("fresh-install database is not empty; unexpected public tables=%v",unexpected)};return nil
}

func readAppliedVersions(ctx context.Context,pool *pgxpool.Pool)([]int64,error){rows,err:=pool.Query(ctx,`SELECT version FROM schema_migrations ORDER BY version`);if err!=nil{return nil,err};defer rows.Close();out:=[]int64{};for rows.Next(){var v int64;if err:=rows.Scan(&v);err!=nil{return nil,err};out=append(out,v)};return out,rows.Err()}
func sameInt64s(a,b []int64)bool{if len(a)!=len(b){return false};for i:=range a{if a[i]!=b[i]{return false}};return true}
