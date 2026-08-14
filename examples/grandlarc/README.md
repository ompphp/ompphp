# Grand Larceny

This is the ompphp version of the classic Grand Larceny example, ported from
ompgo. It includes the three-city selection flow, textdraw UI, character
classes, money transfer on death, weapon checks, and all 106 spawn locations.

Install and run it from a source checkout with:

```bash
composer install
OMPPHP_ENTRY=examples/grandlarc/gamemode.php ./omp-server
```

The original example loads its static vehicles from text files that are not
shipped in ompgo. If you have those files, place them in
`examples/grandlarc/scriptfiles/vehicles/`. The gamemode discovers every `.txt`
file in that directory; each non-empty line should contain:

```text
model,x,y,z,rotation,colour1,colour2
```

Without that directory the example still runs, but does not create static
vehicles.
