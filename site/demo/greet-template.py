#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""A short, full-terminal greeting animation."""

import math
import shutil
import sys
import time


BACKGROUND = "\033[48;2;6;9;15m"
COLORS = (
    "\033[38;2;240;162;60m",
    "\033[38;2;132;170;214m",
    "\033[38;2;233;238;247m",
)
FONT = {
    "H": ("#   #", "#   #", "#   #", "#####", "#   #", "#   #", "#   #"),
    "e": ("     ", " ### ", "#   #", "#####", "#    ", "#   #", " ### "),
    "l": ("#    ", "#    ", "#    ", "#    ", "#    ", "#    ", " ### "),
    "o": ("     ", " ### ", "#   #", "#   #", "#   #", "#   #", " ### "),
    "t": ("  #  ", "  #  ", "#####", "  #  ", "  #  ", "  # # ", "   # "),
    "a": ("     ", " ### ", "    #", " ####", "#   #", "#   #", " ####"),
    "m": ("     ", "## # ", "# # #", "# # #", "# # #", "# # #", "# # #"),
    ",": ("     ", "     ", "     ", "     ", "     ", "  ## ", "  #  "),
    "!": ("  #  ", "  #  ", "  #  ", "  #  ", "  #  ", "     ", "  #  "),
    " ": ("     ",) * 7,
}


def draw(frame, total):
    width, height = shutil.get_terminal_size((80, 24))
    width, height = max(1, width - 1), max(1, height)
    cells = [[" " for _ in range(width)] for _ in range(height)]
    inks = [[0 for _ in range(width)] for _ in range(height)]
    cx, cy = (width - 1) / 2, (height - 1) / 2
    progress = frame / (total - 1)
    radius = progress * math.hypot(width, height) * 0.65

    def put(x, y, char, color):
        if 0 <= x < width and 0 <= y < height:
            cells[y][x] = char
            inks[y][x] = color

    for ray in range(24):
        angle = math.tau * ray / 24
        dx, dy = math.cos(angle), math.sin(angle) * 0.48
        for tail in range(7):
            distance = radius - tail * 2.1
            if distance < 1:
                continue
            x = round(cx + dx * distance)
            y = round(cy + dy * distance)
            char = "*" if tail == 0 else ("+" if tail < 3 else ".")
            put(x, y, char, (ray + tail) % 2)

    for ring in range(3):
        distance = radius * (0.35 + ring * 0.22)
        for point in range(32):
            angle = math.tau * point / 32 + ring * 0.2
            x = round(cx + math.cos(angle) * distance)
            y = round(cy + math.sin(angle) * distance * 0.48)
            if (point + frame // 2) % 3 == 0:
                put(x, y, ".", 1)

    if progress > 0.37:
        message = "Hello, team!"
        scale = 2 if width >= 145 and height >= 22 else 1
        glyph_width = (len(message) * 6 - 1) * scale
        left = (width - glyph_width) // 2
        top = (height - 7 * scale) // 2
        for index, letter in enumerate(message):
            for gy, row in enumerate(FONT[letter]):
                for gx, pixel in enumerate(row):
                    if pixel == "#":
                        for sy in range(scale):
                            for sx in range(scale):
                                put(left + (index * 6 + gx) * scale + sx,
                                    top + gy * scale + sy, "#", 2)
        subtitle = "Hello, team!"
        subtitle_y = top + 7 * scale + 2
        for index, char in enumerate(subtitle):
            put((width - len(subtitle)) // 2 + index, subtitle_y, char, 0)

    lines = []
    for y in range(height):
        line = []
        current = None
        for x, char in enumerate(cells[y]):
            color = inks[y][x]
            if color != current:
                line.append(COLORS[color])
                current = color
            line.append(char)
        lines.append("".join(line))
    sys.stdout.write(BACKGROUND + "\033[H\033[2J" + ("\n".join(lines)))
    sys.stdout.flush()


def main():
    frames = 33
    sys.stdout.write("\033[?25l\033[2J")
    try:
        for frame in range(frames):
            draw(frame, frames)
            if frame < frames - 1:
                time.sleep(0.125)
    finally:
        sys.stdout.write("\033[0m\033[?25h")
        sys.stdout.flush()


if __name__ == "__main__":
    main()
