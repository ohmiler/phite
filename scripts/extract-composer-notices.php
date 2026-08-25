<?php

declare(strict_types=1);

require_once __DIR__ . DIRECTORY_SEPARATOR . 'composer-notice-path.php';

if ($argc !== 3) {
    fwrite(STDERR, "Usage: php extract-composer-notices.php <composer.phar> <output-directory>\n");
    exit(2);
}

$pharPath = realpath($argv[1]);
if ($pharPath === false) {
    fwrite(STDERR, "Composer PHAR does not exist: {$argv[1]}\n");
    exit(1);
}

$outputDirectory = $argv[2];
$licenseDirectory = $outputDirectory . DIRECTORY_SEPARATOR . 'licenses';
if (!is_dir($licenseDirectory) && !mkdir($licenseDirectory, 0777, true) && !is_dir($licenseDirectory)) {
    fwrite(STDERR, "Cannot create output directory: {$licenseDirectory}\n");
    exit(1);
}
$licenseRoot = realpath($licenseDirectory);
if ($licenseRoot === false) {
    fwrite(STDERR, "Cannot resolve license directory: {$licenseDirectory}\n");
    exit(1);
}
$licenseDirectory = $licenseRoot;
$normalizedRoot = rtrim(str_replace('\\', '/', $licenseDirectory), '/') . '/';

$pharPrefix = 'phar://' . str_replace('\\', '/', $pharPath) . '/';
$inventory = file_get_contents($pharPrefix . 'vendor/composer/installed.json');
if ($inventory === false || file_put_contents($outputDirectory . DIRECTORY_SEPARATOR . 'inventory.json', $inventory) === false) {
    fwrite(STDERR, "Cannot extract Composer package inventory\n");
    exit(1);
}

$licenseCount = 0;
foreach (new RecursiveIteratorIterator(new Phar($pharPath)) as $file) {
    $archivePath = str_replace('\\', '/', $file->getPathName());
    $relativePath = substr($archivePath, strlen($pharPrefix));
    $destination = composer_notice_destination($relativePath, $licenseDirectory);
    if ($destination === null) {
        continue;
    }
    $parent = dirname($destination);
    if (!is_dir($parent) && !mkdir($parent, 0777, true) && !is_dir($parent)) {
        fwrite(STDERR, "Cannot create license directory: {$parent}\n");
        exit(1);
    }
    $canonicalParent = realpath($parent);
    if ($canonicalParent === false) {
        fwrite(STDERR, "Cannot resolve license directory: {$parent}\n");
        exit(1);
    }
    $normalizedParent = rtrim(str_replace('\\', '/', $canonicalParent), '/') . '/';
    if (!str_starts_with($normalizedParent, $normalizedRoot)) {
        fwrite(STDERR, "Composer license path escapes output directory: {$relativePath}\n");
        exit(1);
    }
    $contents = file_get_contents($file->getPathName());
    if ($contents === false || file_put_contents($destination, $contents) === false) {
        fwrite(STDERR, "Cannot extract license: {$relativePath}\n");
        exit(1);
    }
    ++$licenseCount;
}

fwrite(STDOUT, "extracted {$licenseCount} Composer license files\n");
