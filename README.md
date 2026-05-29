# SpoTUI

Reproductor de Spotify para la terminal, escrito en Go. Controla la
reproducción a través de la API web de Spotify y reproduce el audio
directamente en tu PC mediante un demonio `spotifyd` embebido, así que **no
necesitas tener abierta la app de Spotify** ni ningún otro cliente.

## Características

- Navegación por tus playlists y reproducción de canciones.
- Búsqueda de canciones y playlists.
- Reproducción local en el PC (vía `spotifyd`), sin depender de otro cliente.
- Carátulas del álbum renderizadas como pixel-art en la terminal.
- Vista "Now Playing" a pantalla completa.
- Control de reproducción: play/pausa, siguiente, anterior y volumen.

## Requisitos

- **Cuenta de Spotify Premium** (obligatorio para reproducir).
- **Linux, macOS o Windows 11.**
- El instalador se encarga del motor de audio; no necesitas instalarlo a mano:
  - En **Linux/macOS** usa `spotifyd`.
  - En **Windows** usa `librespot` (el mismo motor que spotifyd lleva por
    dentro, que sí funciona en Windows). SpoTUI detecta el sistema operativo
    automáticamente y arranca el correcto.

## Instalación

### Linux / macOS

```bash
git clone https://github.com/yjns4584/SpoTUI.git
cd SpoTUI
./install.sh
```

El script detecta tu gestor de paquetes (pacman, apt, dnf, zypper o brew),
instala Go y spotifyd si te faltan, compila el binario y lo deja en tu `PATH`.

### Windows 11

Abre **PowerShell** dentro del repo clonado y ejecuta:

```powershell
git clone https://github.com/yjns4584/SpoTUI.git
cd SpoTUI
Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass
.\install.ps1
```

El script detecta tu gestor de paquetes (winget, scoop o choco), instala Go y
`librespot` si te faltan, compila `spotui.exe` y lo añade a tu `PATH`. Como no
hay binarios precompilados de librespot para Windows, se compila desde el código
con Rust (la primera vez tarda unos minutos). Usa `-SkipLibrespot` si solo
quieres controlar otros dispositivos sin reproducir audio en el PC.

> Usa **Windows Terminal** para que la interfaz se vea correctamente.

Después, lánzalo desde cualquier terminal:

```bash
spotui
```

### Primera ejecución

La primera vez se abrirá el navegador **dos veces** para iniciar sesión:

1. Una para autorizar el control de la reproducción (API web).
2. Otra para autorizar el motor de audio local (`spotifyd` en Linux/macOS,
   `librespot` en Windows).

Es normal y solo ocurre la primera vez; después arranca directo.

## Controles

| Tecla            | Acción                                         |
| ---------------- | ---------------------------------------------- |
| `espacio`        | Play / Pausa                                   |
| `n` / `p`        | Siguiente / Anterior                           |
| `+` / `-`        | Subir / Bajar volumen                          |
| `d`              | Reproducir en este PC (mover el audio aquí)    |
| `enter`          | Reproducir la canción / abrir la playlist      |
| `↑`/`k`, `↓`/`j` | Mover el cursor                                |
| `tab`            | Cambiar de panel                               |
| `/` o `3`        | Buscar                                         |
| `2`              | Vista "Now Playing" a pantalla completa        |
| `1`              | Volver a la vista principal                    |
| `esc`            | Volver / cancelar búsqueda                     |
| `q`              | Salir                                          |

## Compartirlo con otras personas

La app de Spotify está en **modo desarrollo**, lo que significa que solo
pueden usarla las cuentas que el dueño de la app autorice (hasta 25).

Si eres el dueño de la app y quieres dar acceso a alguien:

1. Entra en [developer.spotify.com/dashboard](https://developer.spotify.com/dashboard).
2. Abre tu app → **Settings** → **User Management**.
3. Añade el **nombre** y el **email de Spotify** de cada persona.

Sin este paso, al intentar iniciar sesión Spotify les rechazará.

## Limitaciones

- Solo puedes abrir **tus propias playlists**. Las playlists de otras personas
  (las que aparecen con `↳`) están restringidas por Spotify en modo desarrollo;
  para esas, usa la búsqueda (`/`).
- Requiere Premium para cualquier acción de reproducción.

## Desarrollo

Compilar y ejecutar manualmente (requiere Go y spotifyd instalados):

```bash
make build   # compila en bin/spotui
make run     # compila y ejecuta
make install # instala con `go install`
```
