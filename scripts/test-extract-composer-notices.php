<?php

declare(strict_types=1);

require_once __DIR__ . DIRECTORY_SEPARATOR . 'composer-notice-path.php';

$licenseRoot = sys_get_temp_dir() . DIRECTORY_SEPARATOR . 'phite-license-root';
$maliciousEntries = [
    'vendor/example/dependency/LICENSE../../../../../../escaped.txt',
    'vendor/example/dependency/LICENSE/../../escaped.txt',
    'vendor/../dependency/LICENSE',
    'vendor/example/dependency/NOTICE.php/../../escaped.txt',
    'vendor/example/dependency/LICENSE.txt:stream',
];
foreach ($maliciousEntries as $entry) {
    if (composer_notice_destination($entry, $licenseRoot) !== null) {
        fwrite(STDERR, "Malicious Composer PHAR entry was accepted: {$entry}\n");
        exit(1);
    }
}

$expected = $licenseRoot . DIRECTORY_SEPARATOR . 'example' . DIRECTORY_SEPARATOR
    . 'dependency' . DIRECTORY_SEPARATOR . 'LICENSE.txt';
$actual = composer_notice_destination('vendor/example/dependency/LICENSE.txt', $licenseRoot);
if ($actual !== $expected) {
    fwrite(STDERR, "Valid Composer license path was not mapped correctly\n");
    exit(1);
}

fwrite(STDOUT, "malicious Composer PHAR paths were rejected\n");
