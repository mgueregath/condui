package systeminfo

// LinuxCommand uses kernel interfaces and POSIX tools shared by Linux families.
// It requires procfs, awk, df and sleep, including their BusyBox variants.
const LinuxCommand = `export LC_ALL=C
set -e
cpu() { awk '/^cpu / {total=0; for(i=2;i<=9;i++) total+=$i; printf "%.0f %.0f\n",total,$5+$6; found=1; exit} END {if(!found) exit 1}' /proc/stat; }
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
awk -v a="$cpu1" -v b="$cpu2" 'BEGIN {split(a,x," ");split(b,y," ");dt=y[1]-x[1];di=y[2]-x[2];pct=0;if(dt>0)pct=100*(1-di/dt);if(pct<0)pct=0;if(pct>100)pct=100;printf "%.2f\n",pct}'
awk '/^MemTotal:/ {total=$2} /^MemAvailable:/ {available=$2;hasAvailable=1} /^MemFree:/ {free=$2} /^Buffers:/ {buffers=$2} /^Cached:/ {cached=$2} /^SReclaimable:/ {reclaimable=$2} /^Shmem:/ {shared=$2} END {if(total<=0)exit 1;if(!hasAvailable)available=free+buffers+cached+reclaimable-shared;if(available<0)available=0;if(available>total)available=total;printf "%.0f %.0f\n",(total-available)*1024,total*1024}' /proc/meminfo
dfout=$(df -Pk /)
printf '%s\n' "$dfout" | awk 'NR==2 {printf "%.0f %.0f\n",$3*1024,$2*1024;found=1} END {if(!found)exit 1}'
printf '%s\n' "$t2" "$net1" "$disk1" "$net2" "$disk2"
awk -v a="$t1" -v b="$t2" 'BEGIN {if(b<=a)exit 1;printf "%.2f\n",b-a}'
`
