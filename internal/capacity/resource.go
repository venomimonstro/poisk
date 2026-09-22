package capacity

import (
	"errors"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type ResourceMark struct{
	CPUUsageUsec int64
	At time.Time
}

type ResourceSnapshot struct{
	CPUPercent float64 `json:"cpu_percent"`
	MemoryCurrentBytes int64 `json:"memory_current_bytes"`
	MemoryLimitBytes int64 `json:"memory_limit_bytes"`
	RAMPercent float64 `json:"ram_percent"`
	DiskTotalBytes int64 `json:"disk_total_bytes"`
	DiskAvailableBytes int64 `json:"disk_available_bytes"`
	DiskPercent float64 `json:"disk_percent"`
	CPUCores float64 `json:"cpu_cores"`
	ProbeWarnings []string `json:"probe_warnings,omitempty"`
}

func MarkResources()ResourceMark{return ResourceMark{CPUUsageUsec:readCPUUsageUsec(),At:time.Now()}}

func MeasureResources(start ResourceMark,diskPath string)ResourceSnapshot{
	out:=ResourceSnapshot{CPUCores:cpuLimitCores()};if out.CPUCores<=0{out.CPUCores=float64(runtime.NumCPU())}
	end:=MarkResources();elapsed:=end.At.Sub(start.At).Seconds();if elapsed>0&&start.CPUUsageUsec>=0&&end.CPUUsageUsec>=start.CPUUsageUsec{out.CPUPercent=float64(end.CPUUsageUsec-start.CPUUsageUsec)/1_000_000/elapsed/out.CPUCores*100;if out.CPUPercent>100{out.CPUPercent=100}}
	current,err:=readIntFile("/sys/fs/cgroup/memory.current");if err==nil{out.MemoryCurrentBytes=current}else{out.ProbeWarnings=append(out.ProbeWarnings,"memory.current unavailable")}
	limit,err:=readMemoryMax();if err==nil{out.MemoryLimitBytes=limit;if limit>0{out.RAMPercent=float64(current)/float64(limit)*100}}else{out.ProbeWarnings=append(out.ProbeWarnings,"memory.max unavailable")}
	if strings.TrimSpace(diskPath)==""{diskPath="/"};var st syscall.Statfs_t;if err:=syscall.Statfs(diskPath,&st);err==nil{out.DiskTotalBytes=safeUintToInt64(st.Blocks*uint64(st.Bsize));out.DiskAvailableBytes=safeUintToInt64(st.Bavail*uint64(st.Bsize));used:=out.DiskTotalBytes-out.DiskAvailableBytes;if out.DiskTotalBytes>0{out.DiskPercent=float64(used)/float64(out.DiskTotalBytes)*100}}else{out.ProbeWarnings=append(out.ProbeWarnings,"disk statfs unavailable")}
	return out
}

func readCPUUsageUsec()int64{
	raw,err:=os.ReadFile("/sys/fs/cgroup/cpu.stat");if err!=nil{return -1};for _,line:=range strings.Split(string(raw),"\n"){f:=strings.Fields(line);if len(f)==2&&f[0]=="usage_usec"{v,err:=strconv.ParseInt(f[1],10,64);if err==nil{return v}}};return -1
}

func cpuLimitCores()float64{
	raw,err:=os.ReadFile("/sys/fs/cgroup/cpu.max");if err!=nil{return 0};f:=strings.Fields(string(raw));if len(f)!=2||f[0]=="max"{return 0};quota,err1:=strconv.ParseFloat(f[0],64);period,err2:=strconv.ParseFloat(f[1],64);if err1!=nil||err2!=nil||quota<=0||period<=0{return 0};return quota/period
}
func readMemoryMax()(int64,error){raw,err:=os.ReadFile("/sys/fs/cgroup/memory.max");if err!=nil{return 0,err};v:=strings.TrimSpace(string(raw));if v=="max"{return 0,errors.New("memory limit is max")};return strconv.ParseInt(v,10,64)}
func readIntFile(path string)(int64,error){raw,err:=os.ReadFile(path);if err!=nil{return 0,err};return strconv.ParseInt(strings.TrimSpace(string(raw)),10,64)}
func safeUintToInt64(v uint64)int64{const max=uint64(^uint64(0)>>1);if v>max{return int64(max)};return int64(v)}
