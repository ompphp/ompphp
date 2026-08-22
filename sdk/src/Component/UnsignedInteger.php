<?php
declare(strict_types=1);
namespace Omp\Component;
final readonly class UnsignedInteger implements \Stringable
{
    public string $value;
    public function __construct(string|int $value)
    {
        $text = (string) $value;
        if (!preg_match('/^(0|[1-9][0-9]*)$/', $text)) throw new \InvalidArgumentException("Invalid unsigned integer: $text");
        $this->value = $text;
    }
    public function __toString(): string { return $this->value; }
}
