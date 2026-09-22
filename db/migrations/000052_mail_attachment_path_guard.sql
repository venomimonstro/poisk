ALTER TABLE mail_attachments
    ADD CONSTRAINT mail_attachments_no_backslash_path
    CHECK (position(chr(92) in original_filename)=0);
