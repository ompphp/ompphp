<?php
declare(strict_types=1);
namespace Omp\Component;
final readonly class CallableParameter
{
    public function __construct(
        public string $name,
        public CallableType $type,
        public bool $optional,
        public bool $hasDefault,
        public mixed $default = null,
    ) {}
}
