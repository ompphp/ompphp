# Examples

- `basic` welcomes connecting players.
- `commands` implements `/heal` and `/m4` using the generated public API.
- `dialog` opens a dialog on connect and handles its response.
- `grandlarc` is a larger, multi-file port of the classic Grand Larceny
  gamemode, including its complete city spawn table.

Each example uses the SDK in `../../sdk` while working from a source checkout:

```bash
cd examples/basic
composer install
```

For a release installation, use the SDK artifact repository instructions in the
project README instead of the local path repository.
