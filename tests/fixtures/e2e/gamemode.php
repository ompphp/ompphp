<?php

declare(strict_types=1);

namespace {
    ompphp_native_call('Core_Log', 'OMPPHP_E2E_READY');
}

namespace Omp\Internal {

    function dispatch(string $event, array $arguments = []): ?bool
    {
        if ($event === 'Tick' && empty($GLOBALS['ompphp_e2e_tick'])) {
            $GLOBALS['ompphp_e2e_tick'] = true;
            \ompphp_native_call('Core_Log', 'OMPPHP_E2E_TICK');
        }
        return null;
    }
}
