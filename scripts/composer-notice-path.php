<?php

declare(strict_types=1);

function composer_notice_destination(string $relativePath, string $licenseDirectory): ?string
{
    if ($relativePath === 'LICENSE') {
        return $licenseDirectory . DIRECTORY_SEPARATOR . 'composer' . DIRECTORY_SEPARATOR . 'LICENSE';
    }
    $component = '[A-Za-z0-9](?:[A-Za-z0-9_.-]*[A-Za-z0-9])?';
    $pattern = '#^vendor/(' . $component . '/' . $component
        . ')/(LICENSE|COPYING|NOTICE)(\.[A-Za-z0-9_-]+)?$#iD';
    if (preg_match($pattern, $relativePath, $matches) !== 1) {
        return null;
    }
    return $licenseDirectory . DIRECTORY_SEPARATOR
        . str_replace('/', DIRECTORY_SEPARATOR, $matches[1]) . DIRECTORY_SEPARATOR
        . $matches[2] . ($matches[3] ?? '');
}
