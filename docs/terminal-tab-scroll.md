# Scroll al volver a una pestaña de terminal

Cada terminal SSH o local permanece montada en el mismo contenedor. Al cambiar
de pestaña, las terminales en segundo plano se ocultan con `visibility: hidden`
y conservan sus dimensiones. Así, xterm puede mantener sincronizados el historial
y la barra de desplazamiento aunque sigan llegando datos.

Al volver a una pestaña, la vista conserva la posición del historial que se
estaba leyendo o sigue al final si ya estaba al final. El cambio también se
aplica al alternar entre la terminal y una sesión RDP o VNC.
