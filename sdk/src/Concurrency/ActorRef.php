<?php

declare(strict_types=1);

namespace Omp\Concurrency;

final class ActorRef
{
    private bool $stopping = false;

    /** @internal */
    public function __construct(private readonly int $handle, private readonly Future $ready) {}

    public function ready(): Future { return $this->ready; }

    public function call(string $method, mixed $payload = null): Future
    {
        if ($this->stopping) {
            throw new \RuntimeException('The actor is stopping.');
        }
        return Future::fromHandle(\Omp\Internal\actor_call($this->handle, $method, $payload));
    }

    public function stop(): Future
    {
        if ($this->stopping) {
            throw new \RuntimeException('The actor is already stopping.');
        }
        $this->stopping = true;
        try {
            return Future::fromHandle(\Omp\Internal\actor_stop($this->handle));
        } catch (\Throwable $error) {
            $this->stopping = false;
            throw $error;
        }
    }
}
