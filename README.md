# Super Shell TUI

TUI en Go (Bubble Tea) que embebe un shell interactivo en un panel. Las teclas
se envían **en bruto** al PTY, así que el autocompletado (Tab), el historial
(↑/↓) y la búsqueda inversa (Ctrl+R) de bash funcionan igual que en una
terminal normal.

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
- En apps de pantalla completa (vim, less) esas teclas van a la app.

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
