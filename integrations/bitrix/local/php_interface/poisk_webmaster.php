<?php
use Bitrix\Main\Web\HttpClient;
use Bitrix\Main\Web\Json;

final class PoiskWebmasterClient
{
    private string $apiBase;
    private string $token;
    private int $siteId;

    public function __construct(string $apiBase, string $bearerToken, int $siteId)
    {
        $apiBase = rtrim(trim($apiBase), '/');
        $parts = parse_url($apiBase);
        if (!$parts || strtolower((string)($parts['scheme'] ?? '')) !== 'https' || empty($parts['host'])) {
            throw new InvalidArgumentException('Poisk API base must be HTTPS');
        }
        $bearerToken = trim($bearerToken);
        if ($bearerToken === '' || strlen($bearerToken) > 4096 || $siteId <= 0) {
            throw new InvalidArgumentException('Invalid Poisk Webmaster credentials');
        }
        $this->apiBase = $apiBase;
        $this->token = $bearerToken;
        $this->siteId = $siteId;
    }

    public function submitSitemap(string $url): array
    {
        return $this->post('sitemaps', ['url' => $this->validateSiteURL($url)]);
    }

    public function submitURL(string $url, string $operation = 'SUBMIT'): array
    {
        $operation = strtoupper(trim($operation));
        if (!in_array($operation, ['SUBMIT', 'REINDEX', 'DELETE'], true)) {
            throw new InvalidArgumentException('Invalid URL operation');
        }
        return $this->post('urls', ['url' => $this->validateSiteURL($url), 'operation' => $operation]);
    }

    private function post(string $resource, array $payload): array
    {
        $client = new HttpClient(['socketTimeout' => 5, 'streamTimeout' => 8, 'redirect' => false]);
        $client->setHeader('Authorization', 'Bearer ' . $this->token, true);
        $client->setHeader('Content-Type', 'application/json', true);
        $client->setHeader('Accept', 'application/json', true);
        $body = $client->post($this->apiBase . '/api/webmaster/sites/' . $this->siteId . '/' . $resource, Json::encode($payload));
        $status = (int)$client->getStatus();
        if ($status < 200 || $status >= 300) {
            throw new RuntimeException('Poisk API returned HTTP ' . $status);
        }
        $decoded = Json::decode((string)$body);
        return is_array($decoded) ? $decoded : [];
    }

    private function validateSiteURL(string $url): string
    {
        $url = trim($url);
        $parts = parse_url($url);
        if (!$parts || !in_array(strtolower((string)($parts['scheme'] ?? '')), ['https', 'http'], true) || empty($parts['host']) || isset($parts['user']) || isset($parts['pass'])) {
            throw new InvalidArgumentException('Invalid site URL');
        }
        return $url;
    }
}
