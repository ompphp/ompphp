<?php
declare(strict_types=1);
namespace Omp\Component;
final readonly class CallableDescriptor
{
    /** @param list<CallableParameter> $parameters */
    public function __construct(
        public string $name,
        public string $documentation,
        public array $parameters,
        public CallableType $returnType,
        public bool $deprecated,
        public bool $mayCallback,
    ) {}
}
