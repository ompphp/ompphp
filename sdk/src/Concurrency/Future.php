<?php

declare(strict_types=1);

namespace Omp\Concurrency;

final class Future
{
    private const PENDING = 0;
    private const FULFILLED = 1;
    private const REJECTED = 2;
    private const CANCELLED = 3;

    /** @var array<int, self> */
    private static array $pending = [];

    /** @var list<array{self, array{?callable, ?callable, self}}> */
    private static array $queue = [];
    private static bool $flushing = false;

    private int $state = self::PENDING;
    private mixed $value = null;
    private mixed $error = null;

    /** @var list<array{?callable, ?callable, self}> */
    private array $handlers = [];

    private function __construct(private readonly ?int $handle)
    {
        if ($handle !== null) {
            self::$pending[$handle] = $this;
        }
    }

    /** @internal */
    public static function fromHandle(int $handle): self
    {
        return new self($handle);
    }

    public function then(?callable $fulfilled = null, ?callable $rejected = null): self
    {
        $next = new self(null);
        $this->handlers[] = [$fulfilled, $rejected, $next];
        if (!$this->isPending()) {
            $this->flush();
        }
        return $next;
    }

    public function catch(callable $rejected): self
    {
        return $this->then(null, $rejected);
    }

    public function finally(callable $callback): self
    {
        return $this->then(
            static function (mixed $value) use ($callback): mixed {
                $callback();
                return $value;
            },
            static function (\Throwable $error) use ($callback): mixed {
                $callback();
                throw $error;
            },
        );
    }

    public function withTimeout(int $milliseconds): self
    {
        if ($this->handle !== null && $this->isPending()) {
            \Omp\Internal\future_timeout($this->handle, $milliseconds);
        }
        return $this;
    }

    public function cancel(): bool
    {
        if (!$this->isPending()) {
            return false;
        }
        if ($this->handle !== null) {
            return \Omp\Internal\future_cancel($this->handle);
        }
        $this->reject(new CancelledException(), self::CANCELLED);
        return true;
    }

    public function isPending(): bool { return $this->state === self::PENDING; }
    public function isFulfilled(): bool { return $this->state === self::FULFILLED; }
    public function isRejected(): bool { return $this->state === self::REJECTED; }
    public function isCancelled(): bool { return $this->state === self::CANCELLED; }

    /** @internal
     * @param array{class?: string, message?: string, file?: string, line?: int, trace?: string, worker?: int, task?: int}|null $remote
     */
    public static function complete(int $handle, mixed $value, ?array $remote, bool $cancelled): void
    {
        $future = self::$pending[$handle] ?? null;
        unset(self::$pending[$handle]);
        if ($future === null || !$future->isPending()) {
            return;
        }
        if ($cancelled) {
            $future->reject(new CancelledException(), self::CANCELLED);
        } elseif ($remote !== null) {
            $error = ($remote['class'] ?? null) === 'TimeoutException'
                ? new TimeoutException($remote['message'] ?? 'The asynchronous operation timed out.')
                : new RemoteTaskException($remote);
            $future->reject($error, self::REJECTED);
        } else {
            $future->resolve($value);
        }
    }

    private function resolve(mixed $value): void
    {
        if (!$this->isPending()) {
            return;
        }
        if ($value instanceof self) {
            if ($value === $this) {
                $this->reject(new \LogicException('A Future cannot resolve to itself.'));
                return;
            }
            $value->then(
                function (mixed $result): void { $this->resolve($result); },
                function (\Throwable $error): void { $this->reject($error); },
            );
            return;
        }
        $this->state = self::FULFILLED;
        $this->value = $value;
        $this->flush();
    }

    private function reject(\Throwable $error, int $state = self::REJECTED): void
    {
        if (!$this->isPending()) {
            return;
        }
        $this->state = $state;
        $this->error = $error;
        $this->flush();
    }

    private function flush(): void
    {
        $handlers = $this->handlers;
        $this->handlers = [];
        foreach ($handlers as $handler) {
            self::$queue[] = [$this, $handler];
        }
        if (self::$flushing) {
            return;
        }
        self::$flushing = true;
        try {
            while (self::$queue !== []) {
                $batch = self::$queue;
                self::$queue = [];
                foreach ($batch as [$source, $handler]) {
                    $source->runHandler($handler);
                }
            }
        } finally {
            self::$queue = [];
            self::$flushing = false;
        }
    }

    /** @param array{?callable, ?callable, self} $handler */
    private function runHandler(array $handler): void
    {
        [$fulfilled, $rejected, $next] = $handler;
        try {
            if ($this->state === self::FULFILLED) {
                $next->resolve($fulfilled === null ? $this->value : $fulfilled($this->value));
            } elseif ($rejected === null) {
                $next->reject($this->failure(), $this->state);
            } else {
                $next->resolve($rejected($this->failure()));
            }
        } catch (\Throwable $error) {
            $next->reject($error);
        }
    }

    private function failure(): \Throwable
    {
        return $this->error instanceof \Throwable
            ? $this->error
            : new \RuntimeException('Asynchronous operation failed.');
    }
}
