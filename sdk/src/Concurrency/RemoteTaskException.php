<?php

declare(strict_types=1);

namespace Omp\Concurrency;

final class RemoteTaskException extends \RuntimeException
{
    /** @param array{class?: string, message?: string, file?: string, line?: int, trace?: string, worker?: int, task?: int} $remote */
    public function __construct(private readonly array $remote)
    {
        $class = $remote['class'] ?? 'Throwable';
        $message = $remote['message'] ?? 'Worker task failed.';
        parent::__construct(sprintf('%s: %s', $class, $message));
    }

    /** @return array{class?: string, message?: string, file?: string, line?: int, trace?: string, worker?: int, task?: int} */
    public function remote(): array
    {
        return $this->remote;
    }
}
