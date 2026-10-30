# Evaluación de clientes RDP en Go

Revisión estática de documentación, API y código público al 29 de octubre de 2026. No se descargaron dependencias, compilaron proyectos ni realizaron conexiones. El usuario confirmó que gopher-grdp corresponde a bouncyball-git/gopher-rdp.

## Comparación para Condui

| Criterio | gopher-rdp | nakagami/grdp | adfnekc/grdp |
| --- | --- | --- | --- |
| Escritorio interactivo | Bitmaps, puntero, teclado, ratón; también ejemplo web | OnBitmap, punteros y entrada; ejemplo web separado grdpwasm | OnFrame, framebuffer BGRA, cursor e input |
| Autenticación | TLS y NLA/CredSSP; negocia alternativas | Ofrece SSL e HYBRID/NLA al negociar | Setting.Protocol permite NLA/CredSSP explícito |
| Go declarado en la fuente consultada | 1.25.6, compatible con el 1.25.7 del proyecto | 1.26.3 en master; requiere actualizar Go o revisar una versión anterior | master declara 1.18; confirmar requisito de la revisión finalmente seleccionada |
| Dependencias | Núcleo descrito como biblioteca estándar; el módulo también incluye Ebiten para su visor | go.mod declara struc y x/crypto; H.264 opcional añade requisitos nativos | Mayor grafo declarado: struc, asn1ber, x/crypto y paquetes de visores |
| Ciclo de vida | Connect, Close, Done y OnDisconnect; API consultada sin ConnectContext | Dialer inyectable y callbacks; Login con espera interna de 30 segundos | LoginContext y errores clasificados simplifican cancelación y prueba |
| Licencia publicada | MIT | GPL-3.0 | GPL-3.0 |

Fuentes primarias y enlaces al código en [rdp-fuentes.md](rdp-fuentes.md). Las ramas y versiones consultadas pueden diferir; no atribuir a una versión fijada todas las funciones de otra revisión.

## Recomendación

Elegir gopher-rdp como candidato principal para el PoC: encaja con el Go actual, ofrece la sesión interactiva y permite integrar el núcleo en Wails sin adoptar su aplicación de escritorio. Esto es una recomendación de integración basada en la revisión estática, no una validación de funcionamiento.

nakagami/grdp es una alternativa relevante, especialmente como referencia de renderizado web y teclado, pero su rama actual introduce un cambio de herramienta de Go. No actualizar Go ni adoptar una revisión antigua sin evaluar el coste de ambas opciones.

adfnekc/grdp conserva una ventaja concreta: LoginContext y un framebuffer uniforme reducen trabajo de adaptación del ciclo de vida. Queda como alternativa si gopher-rdp exige modificaciones demasiado extensas para cancelación o certificados.

## Ajustes previos a incorporar gopher-rdp

El código revisado de upgradeTLS establece InsecureSkipVerify=true y admite TLS 1.0 como mínimo inicial. Options no expone configuración TLS en la revisión consultada. No tratar ese comportamiento como verificación del servidor: se necesita una API de verificación o un parche acotado, y una política explícita de certificados. nakagami también desactiva la verificación en su SocketLayer.

La cancelación debe cubrir marcación, negociación, autenticación y sesión; no basta con un timeout externo que deje trabajando una goroutine. Revisar que Close realmente interrumpa cada etapa.

Desactivar reconexión automática, portapapeles y redirecciones no incluidas en el PoC. La reconexión manual y el estado de desconexión deben seguir el flujo común SSH/VNC de Condui. No permitir degradación silenciosa del nivel de autenticación seleccionado.

## Decisión aplicada

El usuario autorizó gopher-rdp y la adaptación local. Se incorporó v1.0.8 con TLS verificado, NLA y cancelación. Ver [implementación](rdp-implementacion.md). La evaluación anterior describe las fuentes upstream revisadas.
