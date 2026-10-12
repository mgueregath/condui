package systeminfo

// LinuxCommand uses kernel interfaces and POSIX tools shared by Linux families.
// It requires procfs, awk, df and sleep, including their BusyBox variants.
const LinuxCommand = `export LC_ALL=C
set -e
cpu() { awk '/^cpu/ {total=0;for(i=2;i<=9;i++)total+=$i;printf "%s %.0f %.0f\n",$1,total,$5+$6;found=1} END {if(!found)exit 1}' /proc/stat; }
net() { awk 'NR>2 {sub(/^[ \t]*/, ""); sub(/:/," "); if($1!="lo"){rx+=$2;tx+=$10}} END {printf "%.0f %.0f\n",rx,tx}' /proc/net/dev; }
disk() { awk '$3!~/^(loop|ram|zram)/ {r+=$6;w+=$10} END {printf "%.0f %.0f\n",r,w}' /proc/diskstats; }
clock() { awk '{print $1}' /proc/uptime; }
t1=$(clock)
cpu1=$(cpu)
net1=$(net)
disk1=$(disk)
sleep 1
t2=$(clock)
cpu2=$(cpu)
net2=$(net)
disk2=$(disk)
memout=$(cat /proc/meminfo)
cpuUsage=$(printf '%s\n' "$cpu1" "$cpu2" | awk '{if($1 in totals){dt=$2-totals[$1];di=$3-idles[$1];pct=0;if(dt>0)pct=100*(1-di/dt);if(pct<0)pct=0;if(pct>100)pct=100;printf "%s %.2f\n",$1,pct}totals[$1]=$2;idles[$1]=$3}')
printf '%s\n' "$cpuUsage" | awk '$1=="cpu" {print $2;found=1} END {if(!found)exit 1}'
printf '%s\n' "$memout" | awk '/^MemTotal:/ {total=$2} /^MemAvailable:/ {available=$2;hasAvailable=1} /^MemFree:/ {free=$2} /^Buffers:/ {buffers=$2} /^Cached:/ {cached=$2} /^SReclaimable:/ {reclaimable=$2} /^Shmem:/ {shared=$2} END {if(total<=0)exit 1;cache=buffers+cached+reclaimable-shared;if(cache<0)cache=0;if(cache>total)cache=total;if(!hasAvailable)available=free+cache;if(available<0)available=0;if(available>total)available=total;printf "%.0f %.0f\n",(total-available)*1024,total*1024}'
dfout=$(df -Pk /)
printf '%s\n' "$dfout" | awk 'NR==2 {printf "%.0f %.0f\n",$3*1024,$2*1024;found=1} END {if(!found)exit 1}'
printf '%s\n' "$t2" "$net1" "$disk1" "$net2" "$disk2"
awk -v a="$t1" -v b="$t2" 'BEGIN {if(b<=a)exit 1;printf "%.2f\n",b-a}'
printf '%s\n' "$memout" | awk '/^MemTotal:/ {total=$2} /^MemAvailable:/ {available=$2;hasAvailable=1} /^MemFree:/ {free=$2} /^Buffers:/ {buffers=$2} /^Cached:/ {cached=$2} /^SReclaimable:/ {reclaimable=$2} /^Shmem:/ {shared=$2} END {if(total<=0)exit 1;cache=buffers+cached+reclaimable-shared;if(cache<0)cache=0;if(cache>total)cache=total;if(!hasAvailable)available=free+cache;if(available<0)available=0;if(available>total)available=total;printf "MEM 0 %.0f %.0f\n",cache*1024,available*1024}'
printf '%s\n' "$cpuUsage" | awk '$1~/^cpu[0-9]+$/ {sub(/^cpu/,"",$1);printf "CORE %s %s\n",$1,$2}'
`
