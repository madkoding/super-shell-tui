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

Funciona en Linux y macOS. En macOS el directorio de cada pestaña se obtiene
con `lsof`, que viene con el sistema.

## Configuración

Archivo TOML en `~/.config/super-shell/config.toml` (Linux) o
`~/Library/Application Support/super-shell/config.toml` (macOS):

```sh
super-shell -init-config          # crea el archivo comentado con los valores por defecto
super-shell -config otra.toml     # usa otro archivo
```

| Clave           | Por defecto | Descripción                                      |
|-----------------|-------------|--------------------------------------------------|
| `shell`         | `$SHELL`    | Shell a ejecutar (`-shell` lo sobrescribe)       |
| `prefix`        | `ctrl+]`    | Prefijo: `ctrl+a`…`ctrl+z`, `ctrl+\`, `ctrl+]`, `ctrl+^`, `ctrl+_` |
| `sidebar`       | `true`      | Mostrar el panel lateral al iniciar              |
| `sidebar_width` | `30`        | Ancho del panel lateral (16-80)                  |
| `scrollback`    | `10000`     | Líneas de historial por shell (1-100000)         |
| `restore_tabs`  | `true`      | Reabrir las pestañas de la sesión anterior       |
| `restore_command` | `type`    | Programa que corría cada pestaña al salir: `type` lo deja escrito, `run` lo ejecuta, `off` lo ignora |
| `colors.accent` | adaptativo  | Color principal en hex (`#9D7CFF`)               |
| `colors.muted`  | adaptativo  | Color secundario en hex                          |

Un valor inválido o una clave desconocida detienen el arranque con un error
que indica el archivo y la clave.

## Atajos (prefijo `Ctrl+]`, configurable)

| Tecla        | Acción                       |
|--------------|------------------------------|
| `Ctrl+] ?`   | Mostrar/ocultar ayuda        |
| `Ctrl+] s`   | Mostrar/ocultar panel lateral|
| `Ctrl+] q`   | Salir                        |
| `Ctrl+] c`   | Nueva pestaña de shell       |
| `Ctrl+] n` / `Ctrl+] p` | Pestaña siguiente / anterior |
| `Ctrl+] 1`…`9` | Ir a la pestaña N          |
| `Ctrl+] r`   | Renombrar la pestaña (Enter guarda, Esc cancela, vacío = automático) |
| `Ctrl+] x`   | Cerrar la pestaña (pide confirmación con `y`) |
| `Ctrl+] /`   | Buscar en el historial       |
| `Ctrl+] Ctrl+]` | Enviar `Ctrl+]` al shell  |

Cualquier otra tecla va directo al shell.

## Pestañas

- Hasta 9 shells abiertos; click en una pestaña de la cabecera para cambiar.
- La cabecera muestra `[activa]` y marca con `•` las
  pestañas en segundo plano que tuvieron salida nueva.
- Al salir de un shell (`exit`, Ctrl+D) se cierra su pestaña; al cerrar la
  última termina la aplicación.
- Una pestaña nueva abre en el directorio de la pestaña activa.
- Al salir con `Ctrl+] q` se guardan las pestañas abiertas (directorio y
  nombre) en `~/.local/state/super-shell/tabs.json` y se reabren al volver a
  iniciar. Si se
  cierra la última pestaña, el archivo se borra y el próximo inicio es limpio.
- Si una pestaña tenía un programa corriendo al salir (por ejemplo
  `npm run dev`), al reabrirla queda escrito en el prompt para relanzarlo con
  Enter. Con `restore_command = "run"` se ejecuta solo; con `"off"` se ignora.
  El historial de la pantalla no se restaura.

## Historial (scrollback)

- `Shift+PgUp` / `Shift+PgDn`: desplazan el historial (hasta 10 000 líneas).
- Cualquier tecla enviada al shell vuelve a la vista en vivo.
- La rueda del mouse también desplaza el historial (en `less`/`man` envía ↑/↓).
- En apps de pantalla completa (vim, less) esas teclas van a la app.
- `Ctrl+] /` busca texto mientras escribes, desde lo más reciente. `↑` o
  `Ctrl+R` salta a la coincidencia anterior, `↓` o `Ctrl+S` a la siguiente,
  `Enter` deja la vista ahí y `Esc` vuelve a la vista en vivo. Ignora
  mayúsculas salvo que la búsqueda tenga alguna.

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
