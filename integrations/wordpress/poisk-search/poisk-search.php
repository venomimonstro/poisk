<?php
/**
 * Plugin Name: Poisk Site Search
 * Description: Adds Poisk ownership verification, Webmaster submit controls and the Poisk site-search widget.
 * Version: 0.2.0
 * Requires at least: 6.4
 * Requires PHP: 8.1
 */

if (!defined('ABSPATH')) { exit; }
require_once __DIR__ . '/api.php';

final class Poisk_Site_Search {
    private const OPTION_VERIFICATION = 'poisk_verification_token';
    private const OPTION_WIDGET_KEY = 'poisk_widget_public_key';
    private const OPTION_WIDGET_SRC = 'poisk_widget_script_url';

    public static function boot(): void {
        add_action('admin_menu', [self::class, 'adminMenu']);
        add_action('admin_init', [self::class, 'registerSettings']);
        add_action('wp_head', [self::class, 'verificationMeta'], 1);
        add_shortcode('poisk_search', [self::class, 'shortcode']);
    }

    public static function adminMenu(): void {
        add_options_page('Poisk Search', 'Poisk Search', 'manage_options', 'poisk-search', [self::class, 'settingsPage']);
    }

    public static function registerSettings(): void {
        register_setting('poisk_search', self::OPTION_VERIFICATION, [
            'type' => 'string',
            'sanitize_callback' => [self::class, 'sanitizeToken'],
            'default' => '',
        ]);
        register_setting('poisk_search', self::OPTION_WIDGET_KEY, [
            'type' => 'string',
            'sanitize_callback' => [self::class, 'sanitizePublicKey'],
            'default' => '',
        ]);
        register_setting('poisk_search', self::OPTION_WIDGET_SRC, [
            'type' => 'string',
            'sanitize_callback' => [self::class, 'sanitizeScriptURL'],
            'default' => '',
        ]);
    }

    public static function sanitizeToken($value): string {
        $value = trim((string)$value);
        return preg_match('/^[A-Za-z0-9_-]{16,256}$/', $value) ? $value : '';
    }

    public static function sanitizePublicKey($value): string {
        $value = trim((string)$value);
        return preg_match('/^psw_[A-Za-z0-9_-]{16,128}$/', $value) ? $value : '';
    }

    public static function sanitizeScriptURL($value): string {
        $value = esc_url_raw(trim((string)$value), ['https']);
        if ($value === '') { return ''; }
        $parts = wp_parse_url($value);
        if (!$parts || strtolower((string)($parts['scheme'] ?? '')) !== 'https' || empty($parts['host'])) { return ''; }
        return $value;
    }

    public static function verificationMeta(): void {
        $token = get_option(self::OPTION_VERIFICATION, '');
        if ($token === '') { return; }
        echo '<meta name="poisk-verification" content="' . esc_attr($token) . '">' . "\n";
    }

    public static function shortcode(array $atts = []): string {
        $key = get_option(self::OPTION_WIDGET_KEY, '');
        $src = get_option(self::OPTION_WIDGET_SRC, '');
        if ($key === '' || $src === '') { return ''; }
        $id = 'poisk-search-' . wp_generate_uuid4();
        return '<div id="' . esc_attr($id) . '" data-poisk-search data-poisk-key="' . esc_attr($key) . '"></div>' .
            '<script async src="' . esc_url($src) . '" data-poisk-key="' . esc_attr($key) . '" data-poisk-target="#' . esc_attr($id) . '"></script>';
    }

    public static function settingsPage(): void {
        if (!current_user_can('manage_options')) { return; }
        $status = sanitize_key((string)($_GET['poisk_status'] ?? ''));
        ?>
        <div class="wrap">
            <h1>Poisk Site Search</h1>
            <p>Verification token и publishable widget key не являются аккаунтными секретами. Webmaster bearer token хранится только server-side в зашифрованном виде и никогда не выводится обратно в HTML.</p>
            <?php if ($status !== ''): ?><div class="notice notice-info"><p><?php echo esc_html($status); ?></p></div><?php endif; ?>
            <form method="post" action="options.php">
                <?php settings_fields('poisk_search'); ?>
                <table class="form-table" role="presentation">
                    <tr><th scope="row"><label for="poisk_verification_token">Verification token</label></th><td><input class="regular-text code" id="poisk_verification_token" name="<?php echo esc_attr(self::OPTION_VERIFICATION); ?>" value="<?php echo esc_attr(get_option(self::OPTION_VERIFICATION, '')); ?>" autocomplete="off"></td></tr>
                    <tr><th scope="row"><label for="poisk_widget_public_key">Widget public key</label></th><td><input class="regular-text code" id="poisk_widget_public_key" name="<?php echo esc_attr(self::OPTION_WIDGET_KEY); ?>" value="<?php echo esc_attr(get_option(self::OPTION_WIDGET_KEY, '')); ?>" autocomplete="off"></td></tr>
                    <tr><th scope="row"><label for="poisk_widget_script_url">Widget script URL</label></th><td><input class="regular-text code" id="poisk_widget_script_url" name="<?php echo esc_attr(self::OPTION_WIDGET_SRC); ?>" value="<?php echo esc_attr(get_option(self::OPTION_WIDGET_SRC, '')); ?>" placeholder="https://search.example/widget/poisk-search.js"></td></tr>
                    <?php Poisk_Webmaster_API::settingsFields(); ?>
                </table>
                <?php submit_button(); ?>
            </form>
            <?php Poisk_Webmaster_API::credentialForm(); ?>
            <?php Poisk_Webmaster_API::submitForms(); ?>
            <p>Для вывода поиска добавьте shortcode <code>[poisk_search]</code>.</p>
        </div>
        <?php
    }
}

Poisk_Site_Search::boot();
