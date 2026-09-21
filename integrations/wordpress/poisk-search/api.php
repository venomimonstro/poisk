<?php
if (!defined('ABSPATH')) { exit; }

final class Poisk_Webmaster_API {
    private const OPTION_API_BASE = 'poisk_api_base';
    private const OPTION_SITE_ID = 'poisk_site_id';
    private const OPTION_TOKEN = 'poisk_webmaster_token_cipher';

    public static function boot(): void {
        add_action('admin_init', [self::class, 'registerSettings']);
        add_action('admin_post_poisk_save_webmaster_token', [self::class, 'saveToken']);
        add_action('admin_post_poisk_submit_sitemap', [self::class, 'submitSitemap']);
        add_action('admin_post_poisk_submit_url', [self::class, 'submitURL']);
    }

    public static function registerSettings(): void {
        register_setting('poisk_search', self::OPTION_API_BASE, [
            'type' => 'string', 'sanitize_callback' => [self::class, 'sanitizeAPIBase'], 'default' => '',
        ]);
        register_setting('poisk_search', self::OPTION_SITE_ID, [
            'type' => 'integer', 'sanitize_callback' => static fn($v) => max(0, (int)$v), 'default' => 0,
        ]);
    }

    public static function sanitizeAPIBase($value): string {
        $value = untrailingslashit(esc_url_raw(trim((string)$value), ['https']));
        $parts = wp_parse_url($value);
        if (!$parts || strtolower((string)($parts['scheme'] ?? '')) !== 'https' || empty($parts['host']) || !wp_http_validate_url($value)) { return ''; }
        return $value;
    }

    public static function settingsFields(): void {
        $configured = get_option(self::OPTION_TOKEN, '') !== '';
        ?>
        <tr><th scope="row"><label for="poisk_api_base">Poisk API base</label></th><td><input class="regular-text code" id="poisk_api_base" name="<?php echo esc_attr(self::OPTION_API_BASE); ?>" value="<?php echo esc_attr(get_option(self::OPTION_API_BASE, '')); ?>" placeholder="https://search.example"></td></tr>
        <tr><th scope="row"><label for="poisk_site_id">Webmaster site ID</label></th><td><input type="number" min="1" id="poisk_site_id" name="<?php echo esc_attr(self::OPTION_SITE_ID); ?>" value="<?php echo (int)get_option(self::OPTION_SITE_ID, 0); ?>"></td></tr>
        <tr><th scope="row">Webmaster API token</th><td><strong><?php echo $configured ? 'Настроен (зашифрован)' : 'Не настроен'; ?></strong><p class="description">Токен не выводится обратно в HTML.</p></td></tr>
        <?php
    }

    public static function credentialForm(): void {
        if (!current_user_can('manage_options')) { return; }
        ?>
        <form method="post" action="<?php echo esc_url(admin_url('admin-post.php')); ?>">
            <input type="hidden" name="action" value="poisk_save_webmaster_token">
            <?php wp_nonce_field('poisk_save_webmaster_token'); ?>
            <input type="password" name="token" class="regular-text code" autocomplete="new-password" placeholder="Bearer token">
            <?php submit_button('Сохранить/заменить API token', 'secondary', 'submit', false); ?>
        </form>
        <?php
    }

    public static function submitForms(): void {
        if (!current_user_can('manage_options')) { return; }
        ?>
        <h2>Отправка в Poisk</h2>
        <form method="post" action="<?php echo esc_url(admin_url('admin-post.php')); ?>" style="margin-bottom:12px">
            <input type="hidden" name="action" value="poisk_submit_sitemap"><?php wp_nonce_field('poisk_submit_sitemap'); ?>
            <input type="url" name="url" class="regular-text code" value="<?php echo esc_attr(home_url('/sitemap.xml')); ?>" required>
            <?php submit_button('Отправить sitemap', 'secondary', 'submit', false); ?>
        </form>
        <form method="post" action="<?php echo esc_url(admin_url('admin-post.php')); ?>">
            <input type="hidden" name="action" value="poisk_submit_url"><?php wp_nonce_field('poisk_submit_url'); ?>
            <input type="url" name="url" class="regular-text code" value="<?php echo esc_attr(home_url('/')); ?>" required>
            <select name="operation"><option value="SUBMIT">SUBMIT</option><option value="REINDEX">REINDEX</option><option value="DELETE">DELETE</option></select>
            <?php submit_button('Отправить URL', 'secondary', 'submit', false); ?>
        </form>
        <?php
    }

    public static function saveToken(): void {
        self::requireAdmin('poisk_save_webmaster_token');
        $plain = trim((string)wp_unslash($_POST['token'] ?? ''));
        if ($plain === '' || strlen($plain) > 4096) { self::redirect('invalid_token'); }
        $cipher = self::encrypt($plain);
        if ($cipher === '') { self::redirect('crypto_unavailable'); }
        update_option(self::OPTION_TOKEN, $cipher, false);
        self::redirect('token_saved');
    }

    public static function submitSitemap(): void {
        self::requireAdmin('poisk_submit_sitemap');
        $url = esc_url_raw((string)wp_unslash($_POST['url'] ?? ''), ['https','http']);
        self::call('sitemaps', ['url' => $url]);
    }

    public static function submitURL(): void {
        self::requireAdmin('poisk_submit_url');
        $url = esc_url_raw((string)wp_unslash($_POST['url'] ?? ''), ['https','http']);
        $operation = strtoupper(trim((string)wp_unslash($_POST['operation'] ?? 'SUBMIT')));
        if (!in_array($operation, ['SUBMIT','REINDEX','DELETE'], true)) { self::redirect('invalid_operation'); }
        self::call('urls', ['url' => $url, 'operation' => $operation]);
    }

    private static function call(string $resource, array $payload): void {
        $base = self::sanitizeAPIBase(get_option(self::OPTION_API_BASE, ''));
        $siteID = (int)get_option(self::OPTION_SITE_ID, 0);
        $token = self::decrypt((string)get_option(self::OPTION_TOKEN, ''));
        if ($base === '' || $siteID <= 0 || $token === '') { self::redirect('api_not_configured'); }
        $target = $base . '/api/webmaster/sites/' . $siteID . '/' . $resource;
        $response = wp_safe_remote_post($target, [
            'timeout' => 8,
            'redirection' => 0,
            'headers' => ['Authorization' => 'Bearer ' . $token, 'Content-Type' => 'application/json', 'Accept' => 'application/json'],
            'body' => wp_json_encode($payload),
            'data_format' => 'body',
        ]);
        if (is_wp_error($response)) { self::redirect('network_error'); }
        $code = (int)wp_remote_retrieve_response_code($response);
        if ($code < 200 || $code >= 300) { self::redirect('poisk_http_' . $code); }
        self::redirect('submitted');
    }

    private static function encrypt(string $plain): string {
        if (!defined('AUTH_KEY') || !function_exists('sodium_crypto_secretbox')) { return ''; }
        $key = hash('sha256', (string)AUTH_KEY, true);
        $nonce = random_bytes(SODIUM_CRYPTO_SECRETBOX_NONCEBYTES);
        return base64_encode($nonce . sodium_crypto_secretbox($plain, $nonce, $key));
    }

    private static function decrypt(string $encoded): string {
        if ($encoded === '' || !defined('AUTH_KEY') || !function_exists('sodium_crypto_secretbox_open')) { return ''; }
        $raw = base64_decode($encoded, true); if ($raw === false || strlen($raw) <= SODIUM_CRYPTO_SECRETBOX_NONCEBYTES) { return ''; }
        $nonce = substr($raw, 0, SODIUM_CRYPTO_SECRETBOX_NONCEBYTES); $cipher = substr($raw, SODIUM_CRYPTO_SECRETBOX_NONCEBYTES);
        $plain = sodium_crypto_secretbox_open($cipher, $nonce, hash('sha256', (string)AUTH_KEY, true));
        return is_string($plain) ? $plain : '';
    }

    private static function requireAdmin(string $nonceAction): void {
        if (!current_user_can('manage_options')) { wp_die('Forbidden', 403); }
        check_admin_referer($nonceAction);
    }

    private static function redirect(string $status): void {
        wp_safe_redirect(add_query_arg(['page' => 'poisk-search', 'poisk_status' => sanitize_key($status)], admin_url('options-general.php')));
        exit;
    }
}

Poisk_Webmaster_API::boot();
