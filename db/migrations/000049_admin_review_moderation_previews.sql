ALTER TABLE admin_action_previews DROP CONSTRAINT IF EXISTS admin_action_previews_action_type_check;
ALTER TABLE admin_action_previews
    ADD CONSTRAINT admin_action_previews_action_type_check
    CHECK (action_type IN ('DOMAIN_POLICY','QUERY_GAP_STATE','DATAHUB_STATE','ORG_REVIEW_DECISION','REVIEW_MODERATION'));

ALTER TABLE admin_action_previews DROP CONSTRAINT IF EXISTS admin_action_previews_target_type_check;
ALTER TABLE admin_action_previews
    ADD CONSTRAINT admin_action_previews_target_type_check
    CHECK (target_type IN ('DOMAIN','QUERY_GAP','DATAHUB_PAGE','ORG_REVIEW','ORGANIZATION_REVIEW'));
