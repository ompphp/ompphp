<?php

declare(strict_types=1);

require __DIR__ . '/vendor/autoload.php';

use Omp\Api\Core;
use Omp\Component\Components;
use Omp\Runtime;

Runtime::assertCompatible();

$component = Components::require('0x4f4d505048500001');
Core::log(sprintf('Found %s component version %s', $component->name, $component->version));

$watch = $component->watch(static function (string $uid): void {
    Core::log("Component $uid was unloaded; stop using its registered callables.");
});

foreach ($component->callables()->all() as $callable) {
    Core::log("Registered callable: {$callable->descriptor->name}");
}

// A component-specific Composer package can wrap require()->invoke() in
// typed PHP methods, just as a Pawn include wraps registered natives.
