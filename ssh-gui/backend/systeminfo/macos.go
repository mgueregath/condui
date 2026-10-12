package systeminfo

// MacOSCommand emits the same nine-line protocol as Linux, with disk bytes.
// Run under /bin/sh independently of the SSH account's default shell.
const MacOSCommand = `export LC_ALL=C
export PATH=/usr/bin:/bin:/usr/sbin:/sbin
set -e
net() {
    data=$(netstat -ibn)
    printf '%s\n' "$data" | awk 'NR==1 {for(i=1;i<=NF;i++){if($i=="Ibytes")rxcol=i;if($i=="Obytes")txcol=i} if(!rxcol||!txcol)exit 1;next} $1!~/^lo[0-9]/ && $3~/^<Link#/ && !seen[$1]++ {rx+=$rxcol;tx+=$txcol} END {if(!rxcol||!txcol)exit 1;printf "%.0f %.0f\n",rx,tx}'
}
disk() {
    data=$(ioreg -r -c IOBlockStorageDriver -w 0)
    printf '%s\n' "$data" | awk '/"Statistics"/ {n=split($0,fields,",");for(i=1;i<=n;i++){if(fields[i]~/"Bytes \(Read\)"/){count=split(fields[i],pair,"=");gsub(/[^0-9]/,"",pair[count]);r+=pair[count];hasRead=1} if(fields[i]~/"Bytes \(Write\)"/){count=split(fields[i],pair,"=");gsub(/[^0-9]/,"",pair[count]);w+=pair[count];hasWrite=1}}} END {if(!hasRead||!hasWrite)exit 1;printf "%.0f %.0f\n",r,w}'
}
t1=$(date +%s)
net1=$(net)
disk1=$(disk)
cpuout=$(top -l 2 -n 0 -s 1)
net2=$(net)
disk2=$(disk)
t2=$(date +%s)
printf '%s\n' "$cpuout" | awk '/CPU usage:/ {for(i=2;i<=NF;i++)if($i=="idle"){idle=$(i-1);gsub(/%/,"",idle);found=1}} END {if(!found)exit 1;printf "%.2f\n",100-idle}'
total=$(sysctl -n hw.memsize)
vmout=$(vm_stat)
printf '%s\n' "$vmout" | awk -v total="$total" '/page size of/ {for(i=1;i<NF;i++)if($i=="of")page=$(i+1)} /^Pages active:/ {active=$NF;gsub(/\./,"",active);hasActive=1} /^Pages wired down:/ {wired=$NF;gsub(/\./,"",wired);hasWired=1} /^Pages occupied by compressor:/ {compressed=$NF;gsub(/\./,"",compressed)} END {if(page<=0||total<=0||!hasActive||!hasWired)exit 1;used=(active+wired+compressed)*page;if(used>total)used=total;printf "%.0f %.0f\n",used,total}'
volume=/
if [ -d /System/Volumes/Data ]; then volume=/System/Volumes/Data; fi
dfout=$(df -Pk "$volume")
printf '%s\n' "$dfout" | awk 'NR==2 {printf "%.0f %.0f\n",$3*1024,$2*1024;found=1} END {if(!found)exit 1}'
bootout=$(sysctl -n kern.boottime)
printf '%s\n' "$bootout" | awk -v now="$t2" 'match($0,/sec = [0-9]+/) {boot=substr($0,RSTART,RLENGTH);sub(/sec = /,"",boot);if(boot<=0||now<boot)exit 1;printf "%.0f\n",now-boot;found=1} END {if(!found)exit 1}'
printf '%s\n' "$net1" "$disk1" "$net2" "$disk2"
awk -v a="$t1" -v b="$t2" 'BEGIN {if(b<=a)exit 1;printf "%.0f\n",b-a}'
`
