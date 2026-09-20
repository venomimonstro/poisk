INSERT INTO system_settings(key,value,updated_at)
VALUES('resource_pressure','{"state":"NORMAL","disk_used_pct":0,"memory_used_pct":0,"checked_at":null}'::jsonb,now())
ON CONFLICT(key) DO NOTHING;
