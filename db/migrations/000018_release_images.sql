ALTER TABLE app_releases
    ADD COLUMN backend_image TEXT,
    ADD COLUMN frontend_image TEXT;

ALTER TABLE app_releases
    ADD CONSTRAINT chk_release_backend_image
    CHECK (backend_image IS NULL OR (length(backend_image) BETWEEN 3 AND 512 AND position(' ' in backend_image)=0));
ALTER TABLE app_releases
    ADD CONSTRAINT chk_release_frontend_image
    CHECK (frontend_image IS NULL OR (length(frontend_image) BETWEEN 3 AND 512 AND position(' ' in frontend_image)=0));
