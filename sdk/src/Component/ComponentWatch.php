<?php
declare(strict_types=1);
namespace Omp\Component;
final class ComponentWatch
{
    private bool $active = true;
    public function __construct(public readonly int $id) {}
    public function cancel(): bool
    {
        if (!$this->active) return false;
        $this->active = false;
        return Components::cancelWatch($this->id);
    }
}
