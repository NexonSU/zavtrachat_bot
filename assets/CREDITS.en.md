# Images in this directory

[Русский](CREDITS.md) | **English**

`bonk.png` and `hug.png` are third-party internet memes. They are **not** covered by the MIT license in
the root of the repository: the MIT license covers the bot's source code, and nobody grants rights to
these images.

| File | What it is | Size |
| --- | --- | --- |
| `bonk.png` | the "Bonk" meme — a shiba inu hitting another shiba inu with a bat (derived from Doge) | 512×314, RGBA |
| `hug.png` | the "hug" meme from the same family of reaction templates | 512×392, RGBA |

The original authors and sources could not be established: both have been circulating for years, and
they reached the repository already scaled down to a 512 px width — the Telegram sticker format.

How they are used: embedded into the binary with `go:embed` (`utils_files.go`); `/bonk` and `/hug` draw
the username over them and send the result as a sticker.

When publishing the project publicly, remove them or replace them with your own. Note that `go:embed` in
`utils_files.go` requires both files to be in place, so simply deleting them breaks the build of `/bonk`
and `/hug`.

## /quote card font

`NotoEmoji.ttf` is Noto Emoji, Copyright 2013 Google LLC, under SIL Open Font License 1.1.
Source: https://github.com/google/fonts/tree/main/ofl/notoemoji
Full license: `NotoEmoji-OFL.txt`. The font is embedded for monochrome emoji.
Body text uses the Go fonts bundled with `golang.org/x/image`.
