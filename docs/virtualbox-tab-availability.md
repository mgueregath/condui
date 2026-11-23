# Disponibilidad de la pestaña VirtualBox

El panel inferior y la navegación móvil muestran VirtualBox cuando la sesión SSH
activa encuentra `VBoxManage` o `vboxmanage` en el PATH remoto. La detección se
ejecuta al abrir la sesión y se reutiliza al volver a ella. No requiere sudo ni
máquinas virtuales registradas.
Si la pestaña seleccionada no está disponible, el panel inferior muestra
Transferencias.
