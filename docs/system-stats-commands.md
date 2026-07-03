# Comandos para Stats de Sistema por OS

Referencia de comandos para obtener stats de sistema en hosts remotos por SSH.
El formato recomendado es salida numerica simple para mapearla a `SystemStats`
sin depender de texto localizado del sistema operativo.

> Estado actual: `ssh-gui/app_docker.go` implementa el flujo Linux usando `/proc`.
> macOS y Windows quedan documentados aqui como base para extender soporte
> multiplataforma.

## Formato esperado

| Campo `SystemStats` | Unidad | Salida recomendada |
| --- | --- | --- |
| `cpuPercent` | porcentaje | `0` a `100` |
| `memUsedGB` / `memFreeGB` / `memTotalGB` | bytes de entrada, GB en Go | `used total` |
| `diskUsedGB` / `diskFreeGB` / `diskTotalGB` | bytes de entrada, GB en Go | `used total` |
| `uptimeSecs` | segundos | `seconds` |
| `netRxBps` / `netTxBps` | bytes por segundo | calcular diferencia entre dos snapshots |
| `diskReadBps` / `diskWriteBps` | bytes por segundo | calcular diferencia entre dos snapshots |

## Tabla de comandos

| Metrica | Linux | macOS | Windows PowerShell |
| --- | --- | --- | --- |
| CPU % | `cpu1=$(awk 'NR==1{print $2+$3+$4+$5+$6+$7+$8+$9,$5+$6}' /proc/stat 2>/dev/null); sleep 0.2; cpu2=$(awk 'NR==1{print $2+$3+$4+$5+$6+$7+$8+$9,$5+$6}' /proc/stat 2>/dev/null); awk -v a="$cpu1" -v b="$cpu2" 'BEGIN{split(a,x," ");split(b,y," ");dt=y[1]-x[1];di=y[2]-x[2];if(dt>0){pct=(1-di/dt)*100;if(pct<0)pct=0;if(pct>100)pct=100;printf "%.0f\n",pct}else print 0}'` | `top -l 2 -n 0 -s 1 \| awk '/CPU usage/ {idle=$7} END {gsub(/%/,"",idle); printf "%.0f\n",100-idle}'` | `(Get-Counter '\Processor(_Total)\% Processor Time').CounterSamples.CookedValue \| ForEach-Object { [math]::Round($_) }` |
| Memoria | `free -b 2>/dev/null \| awk '/Mem:/{printf "%d %d\n",$3,$2}'` | `vm_stat \| awk '/Pages active/ {a=$3} /Pages wired/ {w=$4} /Pages occupied by compressor/ {c=$5} END {gsub("\\\\.","",a);gsub("\\\\.","",w);gsub("\\\\.","",c); used=(a+w+c)*4096; cmd="sysctl -n hw.memsize"; cmd \| getline total; close(cmd); printf "%.0f %.0f\n",used,total}'` | `$os=Get-CimInstance Win32_OperatingSystem; $total=[double]$os.TotalVisibleMemorySize*1024; $free=[double]$os.FreePhysicalMemory*1024; "{0} {1}" -f [math]::Round($total-$free),[math]::Round($total)` |
| Disco raiz | `df -B1 / 2>/dev/null \| awk 'NR==2{printf "%d %d\n",$3,$2}'` | `df -k / \| awk 'NR==2{printf "%.0f %.0f\n",$3*1024,$2*1024}'` | `$d=Get-CimInstance Win32_LogicalDisk -Filter "DeviceID='C:'"; "{0} {1}" -f ($d.Size-$d.FreeSpace),$d.Size` |
| Uptime | `awk '{printf "%.0f\n",$1}' /proc/uptime 2>/dev/null` | `boot=$(sysctl -n kern.boottime \| sed -E 's/.*sec = ([0-9]+).*/\1/'); now=$(date +%s); echo $((now-boot))` | `$os=Get-CimInstance Win32_OperatingSystem; [math]::Round(((Get-Date)-$os.LastBootUpTime).TotalSeconds)` |
| Red snapshot RX/TX | `awk 'NR>2{gsub(/:/,"",$1);if($1!="lo"){rx+=$2;tx+=$10}} END{print rx+0,tx+0}' /proc/net/dev 2>/dev/null` | `netstat -ibn \| awk 'NR>1 && $1!="lo0" {rx+=$7; tx+=$10} END {print rx+0,tx+0}'` | `Get-NetAdapterStatistics \| Measure-Object -Property ReceivedBytes,SentBytes -Sum \| ForEach-Object {}` *(usar script recomendado abajo)* |
| Disco snapshot read/write | `awk '$3!~/^loop/{r+=$6;w+=$10} END{print r+0,w+0}' /proc/diskstats 2>/dev/null` | `iostat -Id disk0 1 2 \| awk 'NF>=3 && $1 ~ /^disk/ {r=$2; w=$3} END {printf "%.0f %.0f\n",r*1024,w*1024}'` | `Get-Counter '\PhysicalDisk(_Total)\Disk Read Bytes/sec','\PhysicalDisk(_Total)\Disk Write Bytes/sec'` |

## Scripts recomendados por OS

### Linux

Este es el flujo equivalente al implementado actualmente. Emite ocho lineas:
CPU, memoria, disco, uptime, red snapshot 1, disco snapshot 1, red snapshot 2,
disco snapshot 2.

```sh
cpu1=$(awk 'NR==1{print $2+$3+$4+$5+$6+$7+$8+$9,$5+$6}' /proc/stat 2>/dev/null)
sleep 0.2
cpu2=$(awk 'NR==1{print $2+$3+$4+$5+$6+$7+$8+$9,$5+$6}' /proc/stat 2>/dev/null)
awk -v a="$cpu1" -v b="$cpu2" 'BEGIN{split(a,x," ");split(b,y," ");dt=y[1]-x[1];di=y[2]-x[2];if(dt>0){pct=(1-di/dt)*100;if(pct<0)pct=0;if(pct>100)pct=100;printf "%.0f\n",pct}else print 0}'
free -b 2>/dev/null | awk '/Mem:/{printf "%d %d\n",$3,$2}' || echo '0 0'
df -B1 / 2>/dev/null | awk 'NR==2{printf "%d %d\n",$3,$2}' || echo '0 0'
awk '{printf "%.0f\n",$1}' /proc/uptime 2>/dev/null || echo 0
awk 'NR>2{gsub(/:/,"",$1);if($1!="lo"){rx+=$2;tx+=$10}} END{print rx+0,tx+0}' /proc/net/dev 2>/dev/null || echo '0 0'
awk '$3!~/^loop/{r+=$6;w+=$10} END{print r+0,w+0}' /proc/diskstats 2>/dev/null || echo '0 0'
sleep 1
awk 'NR>2{gsub(/:/,"",$1);if($1!="lo"){rx+=$2;tx+=$10}} END{print rx+0,tx+0}' /proc/net/dev 2>/dev/null || echo '0 0'
awk '$3!~/^loop/{r+=$6;w+=$10} END{print r+0,w+0}' /proc/diskstats 2>/dev/null || echo '0 0'
```

### macOS

Emite el mismo esquema de ocho lineas. Para disco I/O se usa `iostat` sobre
discos `disk*` y se suma la actividad visible.

```sh
top -l 2 -n 0 -s 1 | awk '/CPU usage/ {idle=$7} END {gsub(/%/,"",idle); printf "%.0f\n",100-idle}'
vm_stat | awk '/Pages active/ {a=$3} /Pages wired/ {w=$4} /Pages occupied by compressor/ {c=$5} END {gsub("\\.","",a);gsub("\\.","",w);gsub("\\.","",c); used=(a+w+c)*4096; cmd="sysctl -n hw.memsize"; cmd | getline total; close(cmd); printf "%.0f %.0f\n",used,total}'
df -k / | awk 'NR==2{printf "%.0f %.0f\n",$3*1024,$2*1024}'
boot=$(sysctl -n kern.boottime | sed -E 's/.*sec = ([0-9]+).*/\1/'); now=$(date +%s); echo $((now-boot))
netstat -ibn | awk 'NR>1 && $1!="lo0" {rx+=$7; tx+=$10} END {print rx+0,tx+0}'
iostat -Id disk0 1 2 | awk 'NF>=3 && $1 ~ /^disk/ {r+=$2; w+=$3} END {printf "%.0f %.0f\n",r*1024,w*1024}'
sleep 1
netstat -ibn | awk 'NR>1 && $1!="lo0" {rx+=$7; tx+=$10} END {print rx+0,tx+0}'
iostat -Id disk0 1 2 | awk 'NF>=3 && $1 ~ /^disk/ {r+=$2; w+=$3} END {printf "%.0f %.0f\n",r*1024,w*1024}'
```

### Windows PowerShell

Emite el mismo esquema de ocho lineas. Red y disco ya se consultan como tasas
por segundo, por lo que se pueden mapear directamente sin doble snapshot si el
backend decide tratar Windows como caso especifico.

```powershell
[math]::Round((Get-Counter '\Processor(_Total)\% Processor Time').CounterSamples.CookedValue)
$os=Get-CimInstance Win32_OperatingSystem; $total=[double]$os.TotalVisibleMemorySize*1024; $free=[double]$os.FreePhysicalMemory*1024; "{0} {1}" -f [math]::Round($total-$free),[math]::Round($total)
$d=Get-CimInstance Win32_LogicalDisk -Filter "DeviceID='C:'"; "{0} {1}" -f ($d.Size-$d.FreeSpace),$d.Size
$os=Get-CimInstance Win32_OperatingSystem; [math]::Round(((Get-Date)-$os.LastBootUpTime).TotalSeconds)
$rx=(Get-NetAdapterStatistics | Measure-Object -Property ReceivedBytes -Sum).Sum; $tx=(Get-NetAdapterStatistics | Measure-Object -Property SentBytes -Sum).Sum; "{0} {1}" -f $rx,$tx
$disk=Get-Counter '\PhysicalDisk(_Total)\Disk Read Bytes/sec','\PhysicalDisk(_Total)\Disk Write Bytes/sec'; "{0} {1}" -f [math]::Round($disk.CounterSamples[0].CookedValue),[math]::Round($disk.CounterSamples[1].CookedValue)
Start-Sleep -Seconds 1
$rx=(Get-NetAdapterStatistics | Measure-Object -Property ReceivedBytes -Sum).Sum; $tx=(Get-NetAdapterStatistics | Measure-Object -Property SentBytes -Sum).Sum; "{0} {1}" -f $rx,$tx
$disk=Get-Counter '\PhysicalDisk(_Total)\Disk Read Bytes/sec','\PhysicalDisk(_Total)\Disk Write Bytes/sec'; "{0} {1}" -f [math]::Round($disk.CounterSamples[0].CookedValue),[math]::Round($disk.CounterSamples[1].CookedValue)
```

## Notas de implementacion

- Preferir comandos que existan por defecto en cada OS.
- Mantener una salida de ocho lineas para reutilizar el parser actual cuando sea posible.
- En Linux, los valores de `/proc/diskstats` son sectores; el backend actual multiplica por `512` para obtener bytes/s.
- En macOS, `iostat` puede requerir ajustar el identificador de disco (`disk0`) si se quiere cubrir todos los discos fisicos.
- En Windows, PowerShell por SSH puede requerir ejecutar el comando bajo `powershell -NoProfile -Command`.
