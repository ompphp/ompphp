<?php

declare(strict_types=1);

namespace Omp\Concurrency;

final class CancelledException extends \RuntimeException
{
    public function __construct()
    {
        parent::__construct('The asynchronous operation was cancelled.');
    }
}
