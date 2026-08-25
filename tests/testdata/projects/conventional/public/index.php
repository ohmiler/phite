<?php

declare(strict_types=1);

header('Content-Type: text/plain; charset=utf-8');

echo 'front controller:' . $_SERVER['REQUEST_URI'];
