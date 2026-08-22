<?php
declare(strict_types=1);
namespace Omp\Component;
final readonly class ComponentVersion
{
    public function __construct(public int $major, public int $minor, public int $patch, public int $preRelease) {}
    public function __toString(): string { return "$this->major.$this->minor.$this->patch" . ($this->preRelease ? "-$this->preRelease" : ''); }
}
