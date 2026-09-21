ALTER TABLE site_search_widgets ALTER COLUMN max_results SET DEFAULT 20;
UPDATE site_search_widgets SET max_results=20 WHERE max_results<20;
