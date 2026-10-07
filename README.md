# Super Shell TUI

TUI en Go (Bubble Tea) que embebe un shell interactivo en un panel. Las teclas
se envían **en bruto** al PTY, así que el autocompletado (Tab), el historial
(↑/↓) y la búsqueda inversa (Ctrl+R) de bash funcionan igual que en una
terminal normal.

## Instalación

Binarios para Linux y macOS (amd64 y arm64) en
[Releases](https://github.com/madkoding/super-shell-tui/releases):

```sh
# ejemplo: Linux amd64 (reemplaza VERSION, p. ej. 0.1.0)
curl -sSL https://github.com/madkoding/super-shell-tui/releases/download/vVERSION/super-shell_VERSION_linux_amd64.tar.gz | tar -xz super-shell
sudo mv super-shell /usr/local/bin/
```

Con Go instalado:

```sh
go install github.com/madkoding/super-shell-tui@latest   # instala el binario como super-shell-tui
```

Para publicar una versión: `git tag v0.1.0 && git push origin v0.1.0`; el
workflow de release compila y sube los binarios con GoReleaser.

## Uso

```sh
go build -o super-shell .
./super-shell              # usa $SHELL o /bin/bash
./super-shell -shell /bin/zsh
```

## Atajos (prefijo `Ctrl+]`)

| Tecla        | Acción                       |
|--------------|------------------------------|
| `Ctrl+] ?`   | Mostrar/ocultar ayuda        |
| `Ctrl+] s`   | Mostrar/ocultar panel lateral|
| `Ctrl+] q`   | Salir                        |
| `Ctrl+] ]`   | Enviar `Ctrl+]` al shell     |

Cualquier otra tecla va directo al shell.

## Historial (scrollback)

- `Shift+PgUp` / `Shift+PgDn`: desplazan el historial (hasta 10 000 líneas).
- Cualquier tecla enviada al shell vuelve a la vista en vivo.
- La rueda del mouse también desplaza el historial (en `less`/`man` envía ↑/↓).
- En apps de pantalla completa (vim, less) esas teclas van a la app.

## Mouse y portapapeles

- Arrastra con el botón izquierdo para seleccionar texto del panel del shell;
  al soltar se copia al portapapeles (OSC 52) y solo se copia el texto del
  shell, sin bordes ni panel lateral.
- Si el programa dentro del shell usa el mouse (vim con `set mouse=a`, htop,
  mc), los eventos se le reenvían con coordenadas del panel.
- La mayoría de terminales permiten la selección nativa con `Shift` + arrastrar.
- En tmux, activa `set -g set-clipboard on` para que la copia llegue al
  sistema.

## Arquitectura

- `main.go`: pone la terminal en modo raw y arranca Bubble Tea con
  `WithInput(nil)` (Bubble Tea nunca lee el teclado).
- `internal/input`: lee stdin byte a byte, intercepta solo el prefijo y
  traduce flechas a SS3 cuando el shell activa *application cursor mode*.
- `internal/shell`: lanza el shell en un PTY (`creack/pty`) y emula la
  pantalla con `charmbracelet/x/vt` (scrollback, caracteres anchos, modos
  del terminal); `Render` la convierte a texto con colores ANSI.
- `internal/ui`: layout (cabecera, panel lateral, panel del shell, estado) y
  redimensionado del PTY.

## Tests

```sh
go test ./...
```
