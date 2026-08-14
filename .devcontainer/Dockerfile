FROM composer:2 AS composer

FROM ghcr.io/adrfranklin/godevcontainer:debian

ARG PHP_VERSION=8.5

ENV CGO_ENABLED=1

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
        lsb-release \
        unzip && \
    curl -sSLo /tmp/debsuryorg-archive-keyring.deb \
        https://packages.sury.org/debsuryorg-archive-keyring.deb && \
    dpkg -i /tmp/debsuryorg-archive-keyring.deb && \
    echo "deb [signed-by=/usr/share/keyrings/debsuryorg-archive-keyring.gpg] https://packages.sury.org/php/ $(lsb_release -sc) main" \
        > /etc/apt/sources.list.d/php.list && \
    apt-get update && \
    apt-get install -y --no-install-recommends \
        php${PHP_VERSION}-cli \
        php${PHP_VERSION}-curl \
        php${PHP_VERSION}-mbstring \
        php${PHP_VERSION}-xml \
        php${PHP_VERSION}-zip && \
    apt-get clean && \
    rm -rf \
        /tmp/debsuryorg-archive-keyring.deb \
        /var/lib/apt/lists/*

COPY --from=composer /usr/bin/composer /usr/local/bin/composer

RUN php --version && \
    composer --version