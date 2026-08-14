# ompphp SDK

This package contains the PHP API for ompphp gamemodes. It is designed for the
PHP 8.2 language level implemented by the embedded runtime.

Release archives can be installed through Composer's artifact repository:

```bash
mkdir -p packages
# Download ompphp-sdk_<version>.zip into packages/, without extracting it.
composer config repositories.ompphp artifact packages
composer require ompphp/sdk:^0.1
```

Require `vendor/autoload.php` from the gamemode and deploy the gamemode together
with its generated `vendor/` directory.
