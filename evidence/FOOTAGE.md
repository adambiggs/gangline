# Footage record — Gangline mushing backdrop

Fetched: 2026-09-12

## Source and licence

- Title: [*Dogs pulling a sled*](https://commons.wikimedia.org/wiki/File:Dogs_pulling_a_sled.webm)
- Author: Jan Mosimann
- Licence: [Creative Commons Attribution 2.0 Generic](https://creativecommons.org/licenses/by/2.0/)
- Commons' licence statement: the file page identifies Jan Mosimann as author, states
  that the video is licensed under CC BY 2.0, and records that its Flickr source was
  reviewed and confirmed under that licence on 2018-02-18.
- Source file: `https://commons.wikimedia.org/wiki/Special:FilePath/Dogs%20pulling%20a%20sled.webm`
- SHA-256: `d05625fcbe4b8b626b031734293847c7588e562e77d8f8cafabe22464e0e2f25`

## Derived loop

The committed frame array is a derived work, not the video. It uses source seconds
20.0–24.0 at 8 fps, crops `1440:810:240:90`, converts to 88 columns by 27 rows with
the `█▓▒░ ` shade ramp, and crossfades its last six frames to the opening pose. The
renderer then removes those six initial frames so the final blended frame naturally
wraps to its successor. Recreate it with:

```sh
FFMPEG=ffmpeg python3 site/tools/render_mushing_ascii.py /path/to/Dogs\ pulling\ a\ sled.webm
```

The footer gives CC BY attribution and links the source page. No video is committed.
