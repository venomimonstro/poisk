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

type upgradeInstallReport struct {
	Database string `json:"database"`
	FromSchema int64 `json:"from_schema"`
	ToSchema int64 `json:"to_schema"`
	BeforeCounts map[string]int64 `json:"before_counts"`
	AfterCounts map[string]int64 `json:"after_counts"`
	AppliedVersions []int64 `json:"applied_versions"`
}

var upgradePreservedTables=[]string{"domains","urls","document_versions","webmaster_sites","organizations","addresses","consumer_users","mailboxes"}

func runUpgradeDatabaseCheck(ctx context.Context,cfg config.Config,expected []int64)(upgradeInstallReport,error){
	dsn,dbName,fromSchema,err:=upgradeInstallDSN();if err!=nil{return upgradeInstallReport{},err}
	candidateDB:=strings.TrimSpace(os.Getenv("POSTGRES_DB"));if candidateDB==""{candidateDB="poisk"};if strings.EqualFold(dbName,candidateDB){return upgradeInstallReport{},errors.New("upgrade database must differ from candidate POSTGRES_DB")}
	if len(expected)==0{return upgradeInstallReport{},errors.New("expected migrations are empty")};expectedCopy:=append([]int64(nil),expected...);sort.Slice(expectedCopy,func(i,j int)bool{return expectedCopy[i]<expectedCopy[j]});latest:=expectedCopy[len(expectedCopy)-1];if fromSchema>=latest{return upgradeInstallReport{},fmt.Errorf("READINESS_UPGRADE_FROM_SCHEMA must be older than current schema %d",latest)}
	pool,err:=pgxpool.New(ctx,dsn);if err!=nil{return upgradeInstallReport{},fmt.Errorf("connect upgrade database: %w",err)};defer pool.Close();if err=pool.Ping(ctx);err!=nil{return upgradeInstallReport{},err}
	appliedBefore,err:=readAppliedVersions(ctx,pool);if err!=nil{return upgradeInstallReport{},fmt.Errorf("read starting migrations: %w",err)};if len(appliedBefore)==0||appliedBefore[len(appliedBefore)-1]!=fromSchema{return upgradeInstallReport{},fmt.Errorf("upgrade source schema mismatch: expected max=%d applied=%v",fromSchema,appliedBefore)}
	expectedBefore:=make([]int64,0,len(expectedCopy));for _,v:=range expectedCopy{if v<=fromSchema{expectedBefore=append(expectedBefore,v)}};if !sameInt64s(expectedBefore,appliedBefore){return upgradeInstallReport{},fmt.Errorf("upgrade source migration set mismatch: expected=%v applied=%v",expectedBefore,appliedBefore)}
	before,err:=canonicalRowCounts(ctx,pool);if err!=nil{return upgradeInstallReport{},err};var retained int64;for _,n:=range before{retained+=n};if retained==0{return upgradeInstallReport{},errors.New("upgrade source contains no retained canonical rows; use a representative database copy")}
	if err=migrate.Up(ctx,pool,cfg.MigrationsDir);err!=nil{return upgradeInstallReport{},fmt.Errorf("upgrade migrations: %w",err)}
	appliedAfter,err:=readAppliedVersions(ctx,pool);if err!=nil{return upgradeInstallReport{},err};if !sameInt64s(expectedCopy,appliedAfter){return upgradeInstallReport{},fmt.Errorf("upgrade final migration set mismatch: expected=%v applied=%v",expectedCopy,appliedAfter)}
	after,err:=canonicalRowCounts(ctx,pool);if err!=nil{return upgradeInstallReport{},err};for table,beforeCount:=range before{afterCount,ok:=after[table];if !ok{return upgradeInstallReport{},fmt.Errorf("canonical table %s disappeared during upgrade",table)};if afterCount<beforeCount{return upgradeInstallReport{},fmt.Errorf("canonical table %s lost rows during upgrade: before=%d after=%d",table,beforeCount,afterCount)}}
	return upgradeInstallReport{Database:dbName,FromSchema:fromSchema,ToSchema:latest,BeforeCounts:before,AfterCounts:after,AppliedVersions:appliedAfter},nil
}

func upgradeInstallDSN()(string,string,int64,error){
	host:=strings.TrimSpace(os.Getenv("READINESS_UPGRADE_POSTGRES_HOST"));port:=strings.TrimSpace(os.Getenv("READINESS_UPGRADE_POSTGRES_PORT"));db:=strings.TrimSpace(os.Getenv("READINESS_UPGRADE_POSTGRES_DB"));user:=strings.TrimSpace(os.Getenv("READINESS_UPGRADE_POSTGRES_USER"));password:=os.Getenv("READINESS_UPGRADE_POSTGRES_PASSWORD");sslmode:=strings.TrimSpace(os.Getenv("READINESS_UPGRADE_POSTGRES_SSLMODE"));fromRaw:=strings.TrimSpace(os.Getenv("READINESS_UPGRADE_FROM_SCHEMA"))
	if port==""{port="5432"};if sslmode==""{sslmode="disable"};if host==""||db==""||user==""||password==""||fromRaw==""{return "","",0,errors.New("READINESS_UPGRADE_POSTGRES_HOST/DB/USER/PASSWORD and READINESS_UPGRADE_FROM_SCHEMA are required")}
	p,err:=strconv.Atoi(port);if err!=nil||p<1||p>65535{return "","",0,errors.New("invalid READINESS_UPGRADE_POSTGRES_PORT")};from,err:=strconv.ParseInt(fromRaw,10,64);if err!=nil||from<=0{return "","",0,errors.New("invalid READINESS_UPGRADE_FROM_SCHEMA")}
	lowerDB:=strings.ToLower(db);if !(strings.Contains(lowerDB,"readiness")||strings.Contains(lowerDB,"test")){return "","",0,errors.New("upgrade database name must contain readiness or test")};if len(db)>63||strings.ContainsAny(db,"/\\\x00\r\n\t"){return "","",0,errors.New("invalid upgrade database name")}
	u:=url.URL{Scheme:"postgres",User:url.UserPassword(user,password),Host:net.JoinHostPort(host,port),Path:"/"+db};q:=u.Query();q.Set("sslmode",sslmode);u.RawQuery=q.Encode();return u.String(),db,from,nil
}

func canonicalRowCounts(ctx context.Context,pool *pgxpool.Pool)(map[string]int64,error){
	out:=map[string]int64{};for _,table:=range upgradePreservedTables{var exists bool;if err:=pool.QueryRow(ctx,`SELECT to_regclass('public.'||$1) IS NOT NULL`,table).Scan(&exists);err!=nil{return nil,err};if !exists{continue};var count int64;query:="SELECT count(*) FROM "+table;if err:=pool.QueryRow(ctx,query).Scan(&count);err!=nil{return nil,fmt.Errorf("count %s: %w",table,err)};out[table]=count};return out,nil
}
