<?php
if (!defined('B_PROLOG_INCLUDED') || B_PROLOG_INCLUDED !== true) { die(); }

$publicKey = trim((string)($arParams['PUBLIC_KEY'] ?? ''));
$scriptUrl = trim((string)($arParams['SCRIPT_URL'] ?? ''));

if (!preg_match('/^psw_[A-Za-z0-9_-]{16,128}$/', $publicKey)) {
    ShowError('Poisk Site Search: invalid public key');
    return;
}

$parts = parse_url($scriptUrl);
if (!$parts || strtolower((string)($parts['scheme'] ?? '')) !== 'https' || empty($parts['host'])) {
    ShowError('Poisk Site Search: SCRIPT_URL must be HTTPS');
    return;
}

$arResult = [
    'PUBLIC_KEY' => $publicKey,
    'SCRIPT_URL' => $scriptUrl,
    'TARGET_ID' => 'poisk-search-' . bin2hex(random_bytes(8)),
];

$this->IncludeComponentTemplate();
