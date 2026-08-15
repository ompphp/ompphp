<?php

declare(strict_types=1);

namespace Omp\Concurrency;

final class ActorPool
{
    /** @param non-empty-list<ActorRef> $actors */
    private function __construct(private readonly int $handle, private readonly array $actors) {}

    public static function spawn(string $class, int $count, mixed $constructorData = null): self
    {
        if ($count < 1) {
            throw new \InvalidArgumentException('Actor pool size must be greater than zero.');
        }
        [$pool, $handles] = \Omp\Internal\actor_pool_spawn($class, $count, $constructorData);
        $actors = [];
        foreach ($handles as [$actor, $future]) {
            $actors[] = new ActorRef($actor, Future::fromHandle($future));
        }
        if ($actors === []) {
            throw new \RuntimeException('The runtime returned an empty actor pool.');
        }
        return new self($pool, $actors);
    }

    public function size(): int { return count($this->actors); }

    public function actor(int $shard): ActorRef
    {
        if (!isset($this->actors[$shard])) {
            throw new \OutOfBoundsException("Actor pool shard {$shard} does not exist.");
        }
        return $this->actors[$shard];
    }

    public function actorFor(int|string $key): ActorRef
    {
        $hash = is_int($key) ? $key : (int) sprintf('%u', crc32($key));
        return $this->actors[abs($hash % count($this->actors))];
    }

    public function call(int|string $key, string $method, mixed $payload = null): Future
    {
        return $this->actorFor($key)->call($method, $payload);
    }

    /** @return list<Future> */
    public function ready(): array
    {
        return array_map(static fn (ActorRef $actor): Future => $actor->ready(), $this->actors);
    }

    /** @return list<Future> */
    public function stop(): array
    {
        return array_map(static fn (int $id): Future => Future::fromHandle($id), \Omp\Internal\actor_pool_stop($this->handle));
    }
}
