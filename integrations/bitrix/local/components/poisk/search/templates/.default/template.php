<?php
if (!defined('B_PROLOG_INCLUDED') || B_PROLOG_INCLUDED !== true) { die(); }
?>
<div id="<?=htmlspecialcharsbx($arResult['TARGET_ID'])?>"
     data-poisk-search
     data-poisk-key="<?=htmlspecialcharsbx($arResult['PUBLIC_KEY'])?>"></div>
<script async
        src="<?=htmlspecialcharsbx($arResult['SCRIPT_URL'])?>"
        data-poisk-key="<?=htmlspecialcharsbx($arResult['PUBLIC_KEY'])?>"
        data-poisk-target="#<?=htmlspecialcharsbx($arResult['TARGET_ID'])?>"></script>
