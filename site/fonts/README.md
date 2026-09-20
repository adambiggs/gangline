# Fonts

All three families are licensed under the SIL Open Font License 1.1; the
licence texts sit beside the files they cover.

| File | Family | Upstream |
|---|---|---|
| `newsreader-latin.woff2` | Newsreader, roman, variable | <https://github.com/productiontype/Newsreader> |
| `newsreader-italic-latin.woff2` | Newsreader, italic, variable | same |
| `instrument-sans-latin.woff2` | Instrument Sans, roman, variable 400 to 700 | <https://github.com/Instrument/instrument-sans> |
| `instrument-sans-italic-latin.woff2` | Instrument Sans, italic, variable 400 to 700 | same |
| `jetbrains-mono-latin.woff2` | JetBrains Mono, regular, static 400 | <https://github.com/JetBrains/JetBrainsMono> |

Newsreader sets headings and keeps its optical-size axis. Instrument Sans sets
everything else; its width axis is pinned at 100 and only the weight axis ships.
JetBrains Mono sets code, at the one weight code uses, with the box-drawing
range kept for terminal output.

## Rebuilding Instrument Sans

Take `InstrumentSans[wdth,wght].ttf` and `InstrumentSans-Italic[wdth,wght].ttf`
from `ofl/instrumentsans/` in <https://github.com/google/fonts>, then for each
(needs `fonttools` and `brotli`):

```sh
fonttools varLib.instancer in.ttf wdth=100 -o inst.ttf
pyftsubset inst.ttf --flavor=woff2 \
  --unicodes='U+0000-00FF,U+2018-201D,U+2026' \
  --output-file=out.woff2
```

## Rebuilding Newsreader

Take the latin `woff2` the Google Fonts CSS API serves for the family, then:

```sh
fonttools ttLib.woff2 decompress in.woff2 -o in.ttf
fonttools varLib.instancer in.ttf 'wght=400:600' -o inst.ttf
pyftsubset inst.ttf --flavor=woff2 \
  --unicodes='U+0000-00FF,U+2018-201D,U+2026' \
  --output-file=out.woff2
```

## Rebuilding JetBrains Mono

Take `JetBrainsMono[wght].ttf` from `ofl/jetbrainsmono/` in
<https://github.com/google/fonts>, then:

```sh
fonttools varLib.instancer in.ttf wght=400 -o inst.ttf
pyftsubset inst.ttf --flavor=woff2 \
  --unicodes='U+0000-00FF,U+2018-201D,U+2026,U+2500-257F' \
  --layout-features='kern,calt' \
  --output-file=out.woff2
```
