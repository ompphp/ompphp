<?php

declare(strict_types=1);

namespace Omp\Concurrency;

final class Actor
{
    public static function spawn(string $class, mixed $constructorData = null): ActorRef
    {
        [$actor, $future] = \Omp\Internal\actor_spawn($class, $constructorData);
        return new ActorRef($actor, Future::fromHandle($future));
    }
}
