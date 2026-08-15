<?php

declare(strict_types=1);

require __DIR__ . '/vendor/autoload.php';

use Example\Concurrency\Counter;
use Example\Concurrency\Square;
use Omp\Async;
use Omp\Concurrency\Actor;
use Omp\Timer;

Async::run(Square::class, 12)->then(static function (mixed $result): void {
    printf("12 squared is %d\n", $result);
});

$counter = Actor::spawn(Counter::class, 10);
$counter->call('add', 5)->then(static function (mixed $result): void {
    printf("Counter is %d\n", $result);
});

Timer::every(1000, static function (): void {
    echo "One second passed.\n";
});
