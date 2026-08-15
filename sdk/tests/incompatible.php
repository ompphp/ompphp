<?php

declare(strict_types=1);

function ompphp_native_call(string $name, mixed ...$arguments): mixed { return null; }
function ompphp_runtime_version(): string { return '0.1.0-test'; }
function ompphp_api_version(): int { return 1; }

require dirname(__DIR__) . '/vendor/autoload.php';

try {
    \Omp\Runtime::assertCompatible();
    throw new RuntimeException('An incompatible native API was accepted.');
} catch (RuntimeException $error) {
    if (!str_contains($error->getMessage(), 'requires ompphp native API 3; API 1 is loaded')) {
        throw $error;
    }
}

echo "SDK incompatibility test passed\n";
