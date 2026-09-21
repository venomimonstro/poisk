<?php
if (!defined('B_PROLOG_INCLUDED') || B_PROLOG_INCLUDED !== true) { die(); }

$arComponentParameters = [
    'PARAMETERS' => [
        'PUBLIC_KEY' => [
            'PARENT' => 'BASE',
            'NAME' => 'Poisk publishable widget key',
            'TYPE' => 'STRING',
            'DEFAULT' => '',
        ],
        'SCRIPT_URL' => [
            'PARENT' => 'BASE',
            'NAME' => 'Poisk widget HTTPS URL',
            'TYPE' => 'STRING',
            'DEFAULT' => '',
        ],
    ],
];
