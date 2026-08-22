<?php
declare(strict_types=1);
namespace Omp\Network;
final class NetworkSubscription
{
    private bool $active = true;
    public function __construct(public readonly int $id) {}
    public function cancel(): bool
    {
        if (!$this->active) return false;
        $this->active = false;
        return Network::cancel($this->id);
    }
}
