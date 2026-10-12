# Monitoreo de recursos del sistema

## Linux

`ssh-gui/app_system_stats.go` expone `GetSystemStats` y valida la salida.
`ssh-gui/backend/systeminfo/linux.go` contiene el comando remoto de referencia.
La interfaz `SystemStats` que consume el frontend conserva sus campos y unidades.

El recolector es comun para Debian, RHEL, Fedora, Arch, SUSE, Alpine y las demas
familias Linux. No necesita elegir comandos por distribucion ni leer os-release
para cada consulta. Requiere `/proc` accesible, shell POSIX, `awk`, `df` y `sleep`.
Utiliza opciones compatibles con GNU y BusyBox; no requiere `free`, `sysstat`,
`systemd` ni permisos sudo. La compatibilidad en hosts reales requiere validacion manual.

| Metrica | Fuente y calculo |
| --- | --- |
| CPU | Diferencia de dos lecturas de `/proc/stat`; idle incluye iowait. |
| Memoria | `/proc/meminfo`: MemTotal menos MemAvailable; fallback con memoria libre, buffers, cache y memoria recuperable para kernels antiguos. |
| Disco raiz | `df -Pk /`; bloques de 1024 bytes convertidos sin enteros de 32 bits. |
| Uptime | `/proc/uptime`. |
| Red | Diferencia de bytes RX/TX de `/proc/net/dev`, excluyendo loopback. |
| Disco I/O | Diferencia de sectores de `/proc/diskstats`, multiplicada por 512 bytes; excluye loop, ram y zram. |

CPU, red y disco se muestrean alrededor de `sleep 1`. Las tasas se dividen por el
tiempo transcurrido medido con `/proc/uptime`. Los contadores que retroceden
producen una tasa cero. El agregado de disco incluye dispositivos y particiones;
en configuraciones con particiones o dispositivos apilados puede contar actividad
mas de una vez, por lo que no equivale necesariamente al I/O fisico.

El protocolo interno contiene nueve lineas: CPU, memoria usada/total en bytes,
disco usado/total en bytes, uptime en segundos, red inicial RX/TX, disco inicial
read/write, red final RX/TX, disco final read/write y duracion en segundos.
La salida incompleta, no numerica o fuera de rango genera un error.
La memoria libre mostrada representa memoria disponible, incluyendo cache recuperable.

## macOS

Implementado en `ssh-gui/backend/systeminfo/macos.go`. `GetSystemStats` selecciona
el recolector segun el OS remoto. Ambos comandos se ejecutan con `/bin/sh`, sin
depender del shell predeterminado de la cuenta SSH. No requiere sudo ni instalar paquetes.

| Metrica | Fuente y calculo |
| --- | --- |
| CPU | Segunda muestra de `top -l 2 -n 0 -s 1`: 100 menos idle. |
| Memoria | `vm_stat`: paginas activas, wired y ocupadas por el compresor, multiplicadas por el tamano de pagina informado; total desde `sysctl -n hw.memsize`. |
| Disco | `df -Pk /System/Volumes/Data` cuando existe, o `/` en sistemas anteriores. |
| Uptime | Diferencia entre `date +%s` y `sysctl -n kern.boottime`. |
| Red | Contadores Ibytes/Obytes de `netstat -ibn`, una fila Link por interfaz; excluye loopback. |
| Disco I/O | Contadores acumulados Bytes (Read)/Bytes (Write) de los drivers IOBlockStorageDriver, obtenidos con `ioreg`. |

Usa el protocolo de nueve lineas. Los contadores de disco estan en bytes, mientras
que Linux los entrega en sectores de 512 bytes; el parser aplica la unidad correcta.
Las muestras de red y disco rodean la medicion de CPU. La duracion se mide con
`date +%s`, con resolucion de un segundo; las tasas son aproximadas y un ajuste del
reloj del host puede afectar la muestra. La memoria usada es una estimacion basada
en paginas activas, wired y comprimidas; no reproduce exactamente Activity Monitor.
El espacio libre de APFS se calcula como total menos usado y puede incluir espacio
reservado o compartido. Red y disco agregan las interfaces y drivers visibles;
interfaces virtuales o drivers apilados pueden duplicar actividad.

Fuentes: [memoria virtual de Apple](https://developer.apple.com/library/archive/documentation/Performance/Conceptual/ManagingMemory/Articles/VMPages.html),
[contadores de IOBlockStorageDriver](https://github.com/apple-oss-distributions/IOStorageFamily/blob/main/IOBlockStorageDriver.h),
[implementacion de netstat](https://github.com/apple-oss-distributions/network_cmds/blob/main/netstat.tproj/if.c).

Validacion pendiente en hosts macOS Intel y Apple Silicon. No se ejecutaron
pruebas dinamicas ni builds.

## Windows (referencia, no implementado)

El siguiente script usa ocho lineas y requiere adaptar el parser.

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
