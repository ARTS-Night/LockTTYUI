# LockTTYUI v0.2

A Linux TTY monitoring dashboard. Go 1.23+; no third-party dependencies.

## Build and run

```sh
go build -o lockttyui .
./lockttyui --exhibit
```

- `./lockttyui --exhibit`: Unicode step-line graphs; Ctrl+] exits, q and Ctrl+C ignored.
- `./lockttyui --glyph-test`: visually inspect console glyphs.
- `./lockttyui --exhibit --ascii`: ASCII fallback.
- `./lockttyui --interval 100ms --sample 500ms`: defaults, display refresh and metric sampling separately scheduled, approximate timing.
- `./lockttyui --sample 100ms`: faster sampling; 60-second history averaged into available columns.

## Security and limitations

No root or network access required. Exhibit mode is not an authentication lock; exit returns to the invoking shell. Physical VT switching and external signals are not blocked. Unicode glyph rendering depends on the installed console font. No font is modified. The v0.2 build still requires physical AlmaLinux 9 TTY validation.

## Test

```sh
go test ./...
go vet ./...
```
