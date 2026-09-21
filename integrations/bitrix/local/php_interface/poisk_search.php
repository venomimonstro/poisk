<?php
use Bitrix\Main\Config\Option;
use Bitrix\Main\EventManager;

if (!defined('B_PROLOG_INCLUDED')) { return; }

EventManager::getInstance()->addEventHandler('main', 'OnEndBufferContent', static function (&$content): void {
    if (!is_string($content) || stripos($content, '</head>') === false) { return; }
    $token = trim((string)Option::get('poisk.search', 'verification_token', ''));
    if ($token === '' || !preg_match('/^[A-Za-z0-9_-]{16,256}$/', $token)) { return; }
    if (stripos($content, 'name="poisk-verification"') !== false || stripos($content, "name='poisk-verification'") !== false) { return; }
    $meta = '<meta name="poisk-verification" content="' . htmlspecialchars($token, ENT_QUOTES | ENT_SUBSTITUTE, 'UTF-8') . '">';
    $content = preg_replace('/<\/head>/i', $meta . "\n</head>", $content, 1) ?? $content;
});
