package resources

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type State string
const (
	Normal State="NORMAL"
	High State="HIGH"
	Critical State="CRITICAL"
)

type Snapshot struct {
	State State `json:"state"`
	DiskUsedPct float64 `json:"disk_used_pct"`
	MemoryUsedPct float64 `json:"memory_used_pct"`
	CheckedAt time.Time `json:"checked_at"`
}

type Monitor struct{DB *pgxpool.Pool;Path string;HighDiskPct,CriticalDiskPct,HighMemoryPct,CriticalMemoryPct float64}

func (m Monitor) Check() (Snapshot,error){
	if m.Path==""{m.Path="/"};if m.HighDiskPct<=0{m.HighDiskPct=85};if m.CriticalDiskPct<=0{m.CriticalDiskPct=93};if m.HighMemoryPct<=0{m.HighMemoryPct=85};if m.CriticalMemoryPct<=0{m.CriticalMemoryPct=95}
	if !(m.HighDiskPct<m.CriticalDiskPct&&m.HighMemoryPct<m.CriticalMemoryPct){return Snapshot{},errors.New("invalid resource watermarks")}
	disk,err:=diskUsage(m.Path);if err!=nil{return Snapshot{},err};memory,_:=memoryUsage()
	state:=Normal;if disk>=m.HighDiskPct||memory>=m.HighMemoryPct{state=High};if disk>=m.CriticalDiskPct||memory>=m.CriticalMemoryPct{state=Critical}
	return Snapshot{State:state,DiskUsedPct:disk,MemoryUsedPct:memory,CheckedAt:time.Now().UTC()},nil
}

func (m Monitor) Persist(ctx context.Context,s Snapshot)error{
	if m.DB==nil{return errors.New("resource monitor database is unavailable")};raw,err:=json.Marshal(s);if err!=nil{return err};_,err=m.DB.Exec(ctx,`INSERT INTO system_settings(key,value,updated_at) VALUES('resource_pressure',$1::jsonb,now()) ON CONFLICT(key) DO UPDATE SET value=EXCLUDED.value,updated_at=now()`,string(raw));return err
}
func (m Monitor) Run(ctx context.Context,interval time.Duration)error{if interval<=0{interval=5*time.Second};ticker:=time.NewTicker(interval);defer ticker.Stop();for{snapshot,err:=m.Check();if err==nil{err=m.Persist(ctx,snapshot)};if err!=nil&&ctx.Err()==nil{return err};select{case<-ctx.Done():return ctx.Err();case<-ticker.C:}}}

func diskUsage(path string)(float64,error){var stat syscall.Statfs_t;if err:=syscall.Statfs(path,&stat);err!=nil{return 0,err};total:=float64(stat.Blocks)*float64(stat.Bsize);free:=float64(stat.Bavail)*float64(stat.Bsize);if total<=0{return 0,errors.New("invalid filesystem size")};return (total-free)*100/total,nil}
func memoryUsage()(float64,error){currentRaw,err:=os.ReadFile("/sys/fs/cgroup/memory.current");if err!=nil{return 0,err};maxRaw,err:=os.ReadFile("/sys/fs/cgroup/memory.max");if err!=nil{return 0,err};maxText:=strings.TrimSpace(string(maxRaw));if maxText=="max"{return 0,nil};current,err:=strconv.ParseUint(strings.TrimSpace(string(currentRaw)),10,64);if err!=nil{return 0,err};max,err:=strconv.ParseUint(maxText,10,64);if err!=nil||max==0{return 0,err};return float64(current)*100/float64(max),nil}
