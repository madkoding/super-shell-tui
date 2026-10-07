"""Converts a `tmux capture-pane -e` dump to an HTML page that looks like a
terminal window. Usage: python3 ans2html.py IN.ans > OUT.html"""
import html
import re
import sys

# Catppuccin Mocha: the 16 ANSI colors, default foreground and background.
PALETTE = ["#1e1e2e", "#f38ba8", "#a6e3a1", "#f9e2af", "#89b4fa", "#f5c2e7", "#94e2d5", "#bac2de",
           "#585b70", "#f38ba8", "#a6e3a1", "#f9e2af", "#89b4fa", "#f5c2e7", "#94e2d5", "#a6adc8"]
FG, BG = "#cdd6f4", "#1e1e2e"
SGR = re.compile(r"(\x1b\[[0-9;:]*m)")


def color256(n):
    if n < 16:
        return PALETTE[n]
    if n < 232:
        n -= 16
        v = [0, 95, 135, 175, 215, 255]
        return "#%02x%02x%02x" % (v[n // 36], v[n // 6 % 6], v[n % 6])
    g = 8 + (n - 232) * 10
    return "#%02x%02x%02x" % (g, g, g)


class Pen:
    def __init__(self):
        self.reset()

    def reset(self):
        self.fg = self.bg = None
        self.bold = self.faint = self.italic = self.underline = self.reverse = False

    def apply(self, params):
        i = 0
        while i < len(params):
            x = params[i]
            if x == 0:
                self.reset()
            elif x == 1:
                self.bold = True
            elif x == 2:
                self.faint = True
            elif x == 3:
                self.italic = True
            elif x == 4:
                self.underline = True
            elif x == 7:
                self.reverse = True
            elif x == 22:
                self.bold = self.faint = False
            elif x == 23:
                self.italic = False
            elif x == 24:
                self.underline = False
            elif x == 27:
                self.reverse = False
            elif 30 <= x <= 37:
                self.fg = PALETTE[x - 30]
            elif 90 <= x <= 97:
                self.fg = PALETTE[x - 82]
            elif 40 <= x <= 47:
                self.bg = PALETTE[x - 40]
            elif 100 <= x <= 107:
                self.bg = PALETTE[x - 92]
            elif x == 39:
                self.fg = None
            elif x == 49:
                self.bg = None
            elif x in (38, 48) and i + 1 < len(params):
                attr = "fg" if x == 38 else "bg"
                if params[i + 1] == 5:
                    setattr(self, attr, color256(params[i + 2]))
                    i += 2
                elif params[i + 1] == 2:
                    setattr(self, attr, "#%02x%02x%02x" % tuple(params[i + 2:i + 5]))
                    i += 4
            i += 1

    def span(self, text):
        fg, bg = self.fg or FG, self.bg or BG
        if self.reverse:
            fg, bg = bg, fg
        style = f"color:{fg};"
        if self.bg or self.reverse:
            style += f"background:{bg};"
        if self.bold:
            style += "font-weight:bold;"
        if self.faint:
            style += "opacity:.6;"
        if self.italic:
            style += "font-style:italic;"
        if self.underline:
            style += "text-decoration:underline;"
        return f'<span style="{style}">{html.escape(text)}</span>'


def convert(dump):
    pen, lines = Pen(), []
    for line in dump.split("\n"):
        out = []
        for part in SGR.split(line):
            if part.startswith("\x1b["):
                pen.apply([int(x) if x else 0 for x in re.split("[;:]", part[2:-1])])
            elif part:
                out.append(pen.span(part))
        lines.append("".join(out))
    return "\n".join(lines)


PAGE = """<!doctype html><meta charset=utf-8><style>
body{margin:0;background:#11111b;padding:28px}
.w{display:inline-block;background:%(bg)s;border-radius:10px;box-shadow:0 10px 40px #000a;overflow:hidden}
.bar{height:30px;background:#181825;display:flex;align-items:center;padding-left:14px;gap:8px}
.bar i{width:12px;height:12px;border-radius:50%%;display:block}
pre{margin:0;padding:12px 14px;font:14px/1.18 "DejaVu Sans Mono",monospace;color:%(fg)s}
</style><div class=w><div class=bar><i style="background:#f38ba8"></i><i style="background:#f9e2af"></i>\
<i style="background:#a6e3a1"></i></div><pre>%(body)s</pre></div>"""

if __name__ == "__main__":
    with open(sys.argv[1], encoding="utf-8") as f:
        print(PAGE % {"bg": BG, "fg": FG, "body": convert(f.read())})
