ALTER TABLE commercial_readiness_evidence
    DROP CONSTRAINT IF EXISTS commercial_readiness_evidence_evidence_type_check;

ALTER TABLE commercial_readiness_evidence
    ADD CONSTRAINT commercial_readiness_evidence_evidence_type_check
    CHECK (evidence_type IN (
        'BUILD_UNIT','INTEGRATION','FRESH_INSTALL','UPGRADE','BROWSER_SMOKE',
        'SECURITY_REGRESSION','EDGE_TLS_PROXY','MTA_FLOW'
    ));
